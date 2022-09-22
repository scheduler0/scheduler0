package models

import (
	"encoding/json"
	"time"
)

// JobModel job model
type JobModel struct {
	ID            int64     `json:"id,omitempty"`
	ProjectID     int64     `json:"project_id"`
	Spec          string    `json:"spec,omitempty"`
	CallbackUrl   string    `json:"callback_url"`
	Data          string    `json:"data"`
	ExecutionType string    `json:"execution_type"`
	DateCreated   time.Time `json:"date_created"`
}

// PaginatedJob paginated container of job transformer
type PaginatedJob struct {
	Total  int64      `json:"total"`
	Offset int64      `json:"offset"`
	Limit  int64      `json:"limit"`
	Data   []JobModel `json:"jobs"`
}

// ToJSON returns content of transformer as JSON
func (jobModel *JobModel) ToJSON() ([]byte, error) {
	if data, err := json.Marshal(jobModel); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

// FromJSON extracts content of JSON object into transformer
func (jobModel *JobModel) FromJSON(body []byte) error {
	if err := json.Unmarshal(body, &jobModel); err != nil {
		return err
	}
	return nil
}
