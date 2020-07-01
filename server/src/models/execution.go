package models

import "time"

type ExecutionModel struct {
	TableName struct{} `sql:"executions"`

	ID          string    `json:"id"`
	JobId       string    `json:"job_id"`
	StatusCode  string    `json:"status_code"`
	Timeout     uint64    `json:"timeout"`
	Response    string    `json:"response"`
	Token       string    `json:"token"`
	DateCreated time.Time `json:"date_created"`
}