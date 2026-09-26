package models

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"scheduler0-private/pkg/scheduler0time"
	"time"

	"github.com/robfig/cron"
)

type JobPriorityLevel uint64

type ExecutionTypes string

const (
	ExecutionTypeHTTP ExecutionTypes = "http"
)

// Job status constants
const (
	JobStatusActive   = "active"
	JobStatusInactive = "inactive"
)

// Job job model
type Job struct {
	ID                uint64     `json:"id,omitempty" fake:"{number:1,100}"`
	ProjectID         uint64     `json:"projectId,omitempty" fake:"{number:1,100}"`
	Spec              string     `json:"spec,omitempty" fake:"{randomstring:[* * * * *]}"`
	Data              string     `json:"data,omitempty" fake:"{word}"`
	ExecutorId        *uint64    `json:"executorId,omitempty" fake:"{number:1,100}"`
	StartDate         time.Time  `json:"startDate,omitempty" fake:"{date}"`
	EndDate           time.Time  `json:"endDate,omitempty" fake:"{date}"`
	LastExecutionDate time.Time  `json:"lastExecutionDate,omitempty" fake:"{date}"`
	Timezone          string     `json:"timezone,omitempty" fake:"{randomstring:[utc, America_NewYork]}"`
	TimezoneOffset    int64      `json:"timezoneOffset,omitempty" fake:"{number:1,100}"`
	RetryMax          int        `json:"retryMax,omitempty" fake:"{number:1,100}"`
	ExecutionId       string     `json:"executionId,omitempty" fake:"{word}"`
	DateCreated       time.Time  `json:"dateCreated,omitempty" fake:"{date}"`
	AccountId         uint64     `json:"accountId,omitempty" fake:"{number:1,100}"`
	DateModified      *time.Time `json:"dateModified,omitempty" fake:"{date}"`
	CreatedBy         string     `json:"createdBy,omitempty" fake:"{word}"`
	ModifiedBy        *string    `json:"modifiedBy,omitempty" fake:"{word}"`
	DeletedBy         *string    `json:"deletedBy,omitempty" fake:"{word}"`
	Status            string     `json:"status,omitempty" fake:"{randomstring:[active, inactive]}"`
}

type JobInvocationPayload struct {
	Job                   `json:"job"`
	LastExecutionDateTime *time.Time `json:"lastExecutionDateTime,omitempty"`
	LastExecutionStatus   string     `json:"lastExecutionStatus"`
}

// AggregatedJobInvocationPayload is the body delivered to an executor when
// payload aggregation is enabled and more than one job sharing that executor
// fires at the same scheduled time. Aggregated is always true so receivers can
// distinguish the aggregated shape ({aggregated, jobs}) from a single
// JobInvocationPayload ({job, ...}). Every job in Jobs succeeds or fails
// together since they are delivered in one call.
type AggregatedJobInvocationPayload struct {
	Aggregated bool                   `json:"aggregated"`
	Jobs       []JobInvocationPayload `json:"jobs"`
}

// JobList returns the underlying jobs of an aggregated payload, used to fan a
// single success/failure result back out to every job in the batch.
func (p AggregatedJobInvocationPayload) JobList() []Job {
	jobs := make([]Job, 0, len(p.Jobs))
	for _, entry := range p.Jobs {
		jobs = append(jobs, entry.Job)
	}
	return jobs
}

// MaxRetryMax returns the largest RetryMax among the aggregated jobs. Since the
// batch is delivered in one call, the whole call is retried up to this bound.
func (p AggregatedJobInvocationPayload) MaxRetryMax() int {
	max := 0
	for _, entry := range p.Jobs {
		if entry.RetryMax > max {
			max = entry.RetryMax
		}
	}
	return max
}

// PaginatedJob paginated container of job transformer
type PaginatedJob struct {
	Total  uint64 `json:"total"`
	Offset uint64 `json:"offset"`
	Limit  uint64 `json:"limit"`
	Data   []Job  `json:"jobs"`
}

// ToJSON returns content of transformer as JSON
func (jobModel *Job) ToJSON() ([]byte, error) {
	if data, err := json.Marshal(jobModel); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

// FromJSON extracts content of JSON object into transformer
func (jobModel *Job) FromJSON(body []byte) error {
	if err := json.Unmarshal(body, &jobModel); err != nil {
		return err
	}
	jobModel.AccountId = 0
	return nil
}

func (jobModel *Job) GetNextExecutionTime() (*time.Time, error) {
	if jobModel.Spec == "" {
		if jobModel.StartDate.IsZero() {
			return nil, fmt.Errorf("one-time job requires a start date")
		}
		if !jobModel.LastExecutionDate.IsZero() {
			zeroTime := time.Time{}
			return &zeroTime, nil
		}
		startInTimezone, err := jobModel.ConvertTimeToJobTimezone(jobModel.StartDate)
		if err != nil {
			return nil, err
		}
		return startInTimezone, nil
	}

	schedule, parseErr := cron.Parse(jobModel.Spec)
	if parseErr != nil {
		return nil, parseErr
	}

	loc, loadErr := time.LoadLocation(jobModel.Timezone)
	if loadErr != nil {
		return nil, loadErr
	}

	if jobModel.LastExecutionDate.IsZero() && !jobModel.StartDate.IsZero() {
		startInTimezone, err := jobModel.ConvertTimeToJobTimezone(jobModel.StartDate)
		if err != nil {
			return nil, err
		}
		return startInTimezone, nil
	}

	// Get current time in the job's timezone for accurate comparison
	schedulerTime := scheduler0time.GetSchedulerTime()
	nowUTC := schedulerTime.GetTime(time.Now())
	nowInJobTimezone := nowUTC.In(loc)

	// Seed the cron search from the last execution time, or from now if the
	// job has never run and has no explicit start date. Seeding from the zero
	// time here would force schedule.Next to walk forward one tick at a time
	// from year 1, which can take millions of iterations.
	currentTime := nowInJobTimezone
	if !jobModel.LastExecutionDate.IsZero() {
		dateCreatedInLocal, err := jobModel.ConvertTimeToJobTimezone(jobModel.LastExecutionDate)
		if err != nil {
			return nil, err
		}
		currentTime = *dateCreatedInLocal
	}

	// Advance through cron schedule to find the next execution time that's in the future
	// Keep advancing until we find a time that's after the present moment
	for !currentTime.After(nowInJobTimezone) {
		currentTime = schedule.Next(currentTime)
	}
	return &currentTime, nil
}

func (jobModel *Job) ConvertTimeToJobTimezone(timeToConvert time.Time) (*time.Time, error) {
	locale, err := time.LoadLocation(jobModel.Timezone)
	if err != nil {
		return nil, err
	}
	timeToConvertInTimezone := timeToConvert.In(locale)
	return &timeToConvertInTimezone, nil
}

func (jobModel *Job) GetNextExecutionId() (string, error) {
	nextExecutionTime, err := jobModel.GetNextExecutionTime()
	if err != nil {
		return "", nil
	}
	uniqueId := fmt.Sprintf(
		"%d-%d-%s-%s",
		jobModel.ProjectID,
		jobModel.ID,
		jobModel.LastExecutionDate.String(),
		nextExecutionTime.String(),
	)
	sha := sha256.New()
	return fmt.Sprintf("%x", sha.Sum([]byte(uniqueId))), err
}

// HasJobEnded checks if the job has ended, factoring in the job's timezone
func (jobModel *Job) HasJobEnded() (bool, error) {
	// If no end date is set, the job hasn't ended
	if jobModel.EndDate.IsZero() {
		return false, nil
	}

	// Load the job's timezone location
	loc, err := time.LoadLocation(jobModel.Timezone)
	if err != nil {
		return false, err
	}

	// Get current time in the job's timezone
	schedulerTime := scheduler0time.GetSchedulerTime()
	nowUTC := schedulerTime.GetTime(time.Now())
	nowInJobTimezone := nowUTC.In(loc)

	// Convert EndDate to the job's timezone for accurate comparison
	endDateInJobTimezone := jobModel.EndDate.In(loc)

	// Check if the end date (in job timezone) is before the current time (in job timezone)
	return endDateInJobTimezone.Before(nowInJobTimezone), nil
}

func (jobModel *Job) IsEndDateInPast() (bool, error) {
	if jobModel.EndDate.IsZero() {
		return false, nil
	}

	loc, err := time.LoadLocation(jobModel.Timezone)
	if err != nil {
		return false, err
	}

	return jobModel.EndDate.In(loc).Before(time.Now().In(loc)), nil
}

func (jobModel *Job) IsStartDateInPast() (bool, error) {
	if jobModel.StartDate.IsZero() {
		return false, nil
	}

	loc, err := time.LoadLocation(jobModel.Timezone)
	if err != nil {
		return false, err
	}

	return jobModel.StartDate.In(loc).Before(time.Now().In(loc)), nil
}
