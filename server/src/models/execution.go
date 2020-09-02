package models

import "time"

type ExecutionModel struct {
	TableName struct{} `sql:"executions"`

	ID          string    `json:"id" sql:",pk:notnull"`
	JobID       string    `json:"job_id" sql:",notnull"`
	StatusCode  string    `json:"status_code"`
	Timeout     uint64    `json:"timeout"`
	Response    string    `json:"response"`
	DateCreated time.Time `json:"date_created" sql:",notnull,default:now()"`

	Job 		JobModel  `pg:",fk:job_id"`
}
