package transformers

import (
	"encoding/json"
	"github.com/victorlenerd/scheduler0/server/src/managers/execution"
	"time"
)

type Execution struct {
	UUID        string    `json:"id"`
	JobUUID     string    `json:"job_uuid"`
	StatusCode  string    `json:"status_code"`
	Timeout     uint64    `json:"timeout"`
	Response    string    `json:"response"`
	Token       string    `json:"token"`
	DateCreated time.Time `json:"date_created"`
}

func (executionTransformer *Execution) ToJson() ([]byte, error) {
	data, err := json.Marshal(executionTransformer)
	if err != nil {
		return data, err
	}
	return data, nil
}

func (executionTransformer *Execution) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &executionTransformer); err != nil {
		return err
	}
	return nil
}

func (executionTransformer *Execution) ToManager() (execution.ExecutionManager, error) {
	executionManager := execution.ExecutionManager{
		UUID:        executionTransformer.UUID,
		JobUUID:     executionTransformer.JobUUID,
		StatusCode:  executionTransformer.StatusCode,
		Timeout:     executionTransformer.Timeout,
		Response:    executionTransformer.Response,
		DateCreated: executionTransformer.DateCreated,
	}

	return executionManager, nil
}

func (executionTransformer *Execution) FromManager(executionManager execution.ExecutionManager) {
	executionTransformer.UUID = executionManager.UUID
	executionTransformer.JobUUID = executionManager.JobUUID
	executionTransformer.StatusCode = executionManager.StatusCode
	executionTransformer.Response = executionManager.Response
	executionTransformer.DateCreated = executionManager.DateCreated
}
