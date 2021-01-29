package transformers

import (
	"encoding/json"
	"github.com/victorlenerd/scheduler0/server/src/managers"
	"time"
)

type Execution struct {
	ID          string    `json:"id"`
	JobID       string    `json:"job_id"`
	StatusCode  string    `json:"status_code"`
	Timeout     uint64    `json:"timeout"`
	Response    string    `json:"response"`
	Token       string    `json:"token"`
	DateCreated time.Time `json:"date_created"`
}

func (exec *Execution) ToJson() ([]byte, error) {
	data, err := json.Marshal(exec)
	if err != nil {
		return data, err
	}
	return data, nil
}

func (exec *Execution) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &exec); err != nil {
		return err
	}
	return nil
}

func (exec *Execution) ToManager() (managers.ExecutionManager, error) {
	execD := managers.ExecutionManager{
		ID:          exec.ID,
		JobID:       exec.JobID,
		StatusCode:  exec.StatusCode,
		Timeout:     exec.Timeout,
		Response:    exec.Response,
		DateCreated: exec.DateCreated,
	}

	return execD, nil
}

func (exec *Execution) FromManager(execD managers.ExecutionManager) {
	exec.ID = execD.ID
	exec.JobID = execD.JobID
	exec.StatusCode = execD.StatusCode
	exec.Response = execD.Response
	exec.DateCreated = execD.DateCreated
}
