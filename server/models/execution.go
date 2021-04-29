package models

import "time"

type ExecutionModel struct {
	TableName struct{} `sql:"executions"`

	ID          int64     `json:"id" sql:",pk:notnull"`
	UUID        string    `json:"uuid" sql:",unique,type:uuid,default:gen_random_uuid()"`
	JobID       int64     `json:"job_id" sql:",notnull"`
	JobUUID     string    `json:"job_uuid" sql:",notnull,type:uuid"`
	StatusCode  string    `json:"status_code"`
	Timeout     uint64    `json:"timeout"`
	DateCreated time.Time `json:"date_created" sql:",notnull,default:now()"`

	Job JobModel `sql:",fk:job_id"`
}
