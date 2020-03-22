package dtos

import (
	"cron-server/server/domains"
	"encoding/json"
	"time"
)

type ExecutionDto struct {
	ID          string    `json:"id"`
	JobId       string    `json:"job_id"`
	StatusCode  string    `json:"status_code"`
	Timeout     uint64    `json:"timeout"`
	Response    string    `json:"response"`
	Token       string    `json:"token"`
	DateCreated time.Time `json:"date_created"`
}

func (exec *ExecutionDto) ToJson() ([]byte, error) {
	data, err := json.Marshal(exec)
	if err != nil {
		return data, err
	}
	return data, nil
}

func (exec *ExecutionDto) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &exec); err != nil {
		return err
	}
	return nil
}

func (exec *ExecutionDto) ToDomain() (domains.ExecutionDomain, error) {
	execD := domains.ExecutionDomain{
		ID:          exec.ID,
		JobId:       exec.JobId,
		StatusCode:  exec.StatusCode,
		Timeout:     exec.Timeout,
		Response:    exec.Response,
		Token:       exec.Token,
		DateCreated: exec.DateCreated,
	}

	return execD, nil
}

func (exec *ExecutionDto) FromDomain(execD domains.ExecutionDomain) {
	exec.ID = execD.ID
	exec.JobId = execD.JobId
	exec.Token = execD.Token
	exec.StatusCode = execD.StatusCode
	exec.Response = execD.Response
	exec.DateCreated = execD.DateCreated
}
