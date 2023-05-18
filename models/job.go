package models

import (
	"encoding/json"
	"time"
)

type JobPriorityLevel uint64

type ExecutionTypes string

const (
	ExecutionTypeHTTP ExecutionTypes = "http"
)

// JobModel job model
type JobModel struct {
	ID                uint64    `json:"id,omitempty" fake:"{number:1,100}"`
	ProjectID         uint64    `json:"project_id,omitempty" fake:"{number:1,100}"`
	Spec              string    `json:"spec,omitempty"`
	CallbackUrl       string    `json:"callback_url,omitempty" fake:"{randomstring:[https://hello.com,https://world.com]}"`
	Data              string    `json:"data,omitempty"`
	ExecutionType     string    `json:"execution_type,omitempty"`
	LastExecutionDate time.Time `json:"last_execution_date,omitempty"`
	Timezone          string    `json:"timezone,omitempty" fake:"{randomstring:[utc, America_NewYork]}"`
	TimezoneOffset    int64     `json:"timezoneOffset,omitempty"`
	ExecutionId       string    `json:"execution_id,omitempty"`
	DateCreated       time.Time `json:"date_created,omitempty"`
}

// PaginatedJob paginated container of job transformer
type PaginatedJob struct {
	Total  uint64     `json:"total,omitempty"`
	Offset uint64     `json:"offset,omitempty"`
	Limit  uint64     `json:"limit,omitempty"`
	Data   []JobModel `json:"jobs,omitempty"`
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
