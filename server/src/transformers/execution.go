package transformers

import (
	"encoding/json"
	"scheduler0/server/src/managers/execution"
	"time"
)


// Execution this transformer is used for execution entity type
type Execution struct {
	UUID        string    `json:"id"`
	JobUUID     string    `json:"job_uuid"`
	StatusCode  string    `json:"status_code"`
	Timeout     uint64    `json:"timeout"`
	Response    string    `json:"response"`
	Token       string    `json:"token"`
	DateCreated time.Time `json:"date_created"`
}


// PaginatedExecutions this holds meta information for pagination
type PaginatedExecutions struct {
	Total  int         `json:"total"`
	Offset int         `json:"offset"`
	Limit  int         `json:"limit"`
	Data   []Execution `json:"executions"`
}

// ToJSON returns JSON representation of transformer
func (executionTransformer *Execution) ToJSON() ([]byte, error) {
	data, err := json.Marshal(executionTransformer)
	if err != nil {
		return data, err
	}
	return data, nil
}

// FromJSON extracts content of JSON object into transformer
func (executionTransformer *Execution) FromJSON(body []byte) error {
	if err := json.Unmarshal(body, &executionTransformer); err != nil {
		return err
	}
	return nil
}

// ToManager converts content of transformer into manager
func (executionTransformer *Execution) ToManager() (execution.Manager, error) {
	executionManager := execution.Manager{
		UUID:        executionTransformer.UUID,
		JobUUID:     executionTransformer.JobUUID,
		StatusCode:  executionTransformer.StatusCode,
		Timeout:     executionTransformer.Timeout,
		Response:    executionTransformer.Response,
		DateCreated: executionTransformer.DateCreated,
	}

	return executionManager, nil
}

// FromManager extract content of manager into transformer
func (executionTransformer *Execution) FromManager(executionManager execution.Manager) {
	executionTransformer.UUID = executionManager.UUID
	executionTransformer.JobUUID = executionManager.JobUUID
	executionTransformer.StatusCode = executionManager.StatusCode
	executionTransformer.Response = executionManager.Response
	executionTransformer.DateCreated = executionManager.DateCreated
}
