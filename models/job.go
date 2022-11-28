package models

import (
	"encoding/json"
	"time"
)

type JobPriorityLevel int

// JobModel job model
type JobModel struct {
	ID                int64            `json:"id,omitempty"`
	ProjectID         int64            `json:"project_id,omitempty"`
	Spec              string           `json:"spec,omitempty"`
	CallbackUrl       string           `json:"callback_url,omitempty"`
	Data              string           `json:"data,omitempty"`
	ExecutionType     string           `json:"execution_type,omitempty"`
	PriorityLevel     JobPriorityLevel `json:"priority_level,omitempty"`
	LastExecutionDate time.Time        `json:"last_execution_date,omitempty"`
	NextExecutionId   string           `json:"next_execution_id,omitempty"`
	DateCreated       time.Time        `json:"date_created,omitempty"`
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
