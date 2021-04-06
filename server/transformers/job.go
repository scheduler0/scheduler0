package transformers

import (
	"encoding/json"
	"errors"
	"scheduler0/server/managers/job"
	"time"
)

// Job transformer type
type Job struct {
	UUID        string `json:"uuid,omitempty"`
	ProjectUUID string `json:"project_uuid"`
	Description string `json:"description"`
	CronSpec    string `json:"spec,omitempty"`
	Data        string `json:"transformers,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
	CallbackUrl string `json:"callback_url"`
	StartDate   string `json:"start_date,omitempty"`
	EndDate     string `json:"end_date,omitempty"`
}

// PaginatedJob paginated container of job transformer
type PaginatedJob struct {
	Total  int   `json:"total"`
	Offset int   `json:"offset"`
	Limit  int   `json:"limit"`
	Data   []Job `json:"jobs"`
}

// ToJSON returns content of transformer as JSON
func (jobTransformer *Job) ToJSON() ([]byte, error) {
	if data, err := json.Marshal(jobTransformer); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

// FromJSON extracts content of JSON object into transformer
func (jobTransformer *Job) FromJSON(body []byte) error {
	if err := json.Unmarshal(body, &jobTransformer); err != nil {
		return err
	}
	return nil
}

// ToManager returns content of transformer as manager
func (jobTransformer *Job) ToManager() (job.Manager, error) {
	jd := job.Manager{
		UUID:        jobTransformer.UUID,
		ProjectUUID: jobTransformer.ProjectUUID,
		CronSpec:    jobTransformer.CronSpec,
		Description: jobTransformer.Description,
		CallbackUrl: jobTransformer.CallbackUrl,
		Timezone:    jobTransformer.Timezone,
	}

	if len(jobTransformer.StartDate) < 1 {
		return job.Manager{}, errors.New("start date is required")
	}

	startTime, err := time.Parse(time.RFC3339, jobTransformer.StartDate)
	if err != nil {
		return job.Manager{}, err
	}
	jd.StartDate = startTime

	if len(jobTransformer.EndDate) > 1 {
		endTime, err := time.Parse(time.RFC3339, jobTransformer.EndDate)
		if err != nil {
			return job.Manager{}, err
		}

		jd.EndDate = endTime
	}

	return jd, nil
}

// FromManager extracts content from manager into transformer
func (jobTransformer *Job) FromManager(jobManager job.Manager) {
	jobTransformer.UUID = jobManager.UUID
	jobTransformer.ProjectUUID = jobManager.ProjectUUID
	jobTransformer.Timezone = jobManager.Timezone
	jobTransformer.Description = jobManager.Description
	jobTransformer.CallbackUrl = jobManager.CallbackUrl
	jobTransformer.CronSpec = jobManager.CronSpec
	jobTransformer.StartDate = jobManager.StartDate.Format(time.RFC3339)
	jobTransformer.EndDate = jobManager.EndDate.Format(time.RFC3339)
}
