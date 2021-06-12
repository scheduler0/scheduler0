package models

import "time"

// ExecutionModel execution model
type ExecutionModel struct {
	TableName struct{} `sql:"executions"`

	ID            int64     `json:"id" sql:",pk:notnull"`
	UUID          string    `json:"uuid" sql:",unique,type:uuid,default:gen_random_uuid()"`
	JobID         int64     `json:"job_id" sql:",notnull"`
	JobUUID       string    `json:"job_uuid" sql:",notnull,type:uuid"`
	TimeAdded     time.Time `json:"time_added" sql:",notnull"`
	TimeExecuted  time.Time `json:"time_executed"`
	ExecutionTime uint64    `json:"execution_time"`
	StatusCode    string    `json:"status_code"`
	DateCreated   time.Time `json:"date_created" sql:",notnull,default:now()"`

	Job JobModel `sql:",fk:job_id"`
}
