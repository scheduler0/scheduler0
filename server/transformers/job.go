package transformers

import (
	"encoding/json"
	"errors"
	"scheduler0/server/managers/job"
	"time"
)

type Job struct {
	UUID        string `json:"uuid,omitempty"`
	ProjectUUID string `json:"project_uuid"`
	Description string `json:"description"`
	CronSpec    string `json:"cron_spec,omitempty"`
	Data        string `json:"transformers,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
	CallbackUrl string `json:"callback_url"`
	StartDate   string `json:"start_date,omitempty"`
	EndDate     string `json:"end_date,omitempty"`
}

type PaginatedJobs struct {
	Total  int   `json:"total"`
	Offset int   `json:"offset"`
	Limit  int   `json:"limit"`
	Data   []Job `json:"jobs"`
}

func (jobTransformer *Job) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &jobTransformer); err != nil {
		return err
	}
	return nil
}

func (jobTransformer *Job) ToJson() ([]byte, error) {
	if data, err := json.Marshal(jobTransformer); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

func (jobTransformer *Job) ToManager() (job.JobManager, error) {
	jd := job.JobManager{
		UUID:        jobTransformer.UUID,
		ProjectUUID: jobTransformer.ProjectUUID,
		CronSpec:    jobTransformer.CronSpec,
		Data:        jobTransformer.Data,
		Description: jobTransformer.Description,
		CallbackUrl: jobTransformer.CallbackUrl,
		Timezone:    jobTransformer.Timezone,
	}

	if len(jobTransformer.StartDate) < 1 {
		return job.JobManager{}, errors.New("start date is required")
	}

	startTime, err := time.Parse(time.RFC3339, jobTransformer.StartDate)
	if err != nil {
		return job.JobManager{}, err
	}
	jd.StartDate = startTime

	if len(jobTransformer.EndDate) > 1 {
		endTime, err := time.Parse(time.RFC3339, jobTransformer.EndDate)
		if err != nil {
			return job.JobManager{}, err
		}

		jd.EndDate = endTime
	}

	return jd, nil
}

func (jobTransformer *Job) FromManager(jobManager job.JobManager) {
	jobTransformer.UUID = jobManager.UUID
	jobTransformer.ProjectUUID = jobManager.ProjectUUID
	jobTransformer.Timezone = jobManager.Timezone
	jobTransformer.Description = jobManager.Description
	jobTransformer.Data = jobManager.Data
	jobTransformer.CallbackUrl = jobManager.CallbackUrl
	jobTransformer.CronSpec = jobManager.CronSpec
	jobTransformer.StartDate = jobManager.StartDate.Format(time.RFC3339)
	jobTransformer.EndDate = jobManager.EndDate.Format(time.RFC3339)
}
