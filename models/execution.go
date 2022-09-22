package models

import (
	"encoding/json"
	"time"
)

// ExecutionModel execution model
type ExecutionModel struct {
	ID            int64     `json:"id" sql:",pk:notnull"`
	JobID         int64     `json:"job_id" sql:",notnull"`
	TimeAdded     time.Time `json:"time_added" sql:",notnull"`
	TimeExecuted  time.Time `json:"time_executed"`
	ExecutionTime int64     `json:"execution_time"`
	StatusCode    string    `json:"status_code"`
	DateCreated   time.Time `json:"date_created" sql:",notnull,default:now()"`
}

// PaginatedExecution this holds meta information for pagination
type PaginatedExecution struct {
	Total  int64            `json:"total"`
	Offset int64            `json:"offset"`
	Limit  int64            `json:"limit"`
	Data   []ExecutionModel `json:"executions"`
}

// ToJSON returns JSON representation of transformer
func (executionModel *ExecutionModel) ToJSON() ([]byte, error) {
	data, err := json.Marshal(executionModel)
	if err != nil {
		return data, err
	}
	return data, nil
}

// FromJSON extracts content of JSON object into transformer
func (executionModel *ExecutionModel) FromJSON(body []byte) error {
	if err := json.Unmarshal(body, &executionModel); err != nil {
		return err
	}
	return nil
}
