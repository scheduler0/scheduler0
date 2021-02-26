package models

import "time"

type JobModel struct {
	TableName struct{} `sql:"jobs"`

	ID             int64     `json:"id,omitempty" sql:",pk:notnull"`
	UUID           string    `json:"uuid" sql:",pk:notnull,unique,type:uuid,default:gen_random_uuid()"`
	Archived       bool      `json:"archived" sql:",notnull"`
	ProjectID      int64     `json:"project_id" sql:",notnull"`
	ProjectUUID    string    `json:"project_uuid" sql:",notnull,type:uuid"`
	Description    string    `json:"description" sql:",notnull"`
	CronSpec       string    `json:"cron_spec,omitempty" sql:",notnull"`
	CallbackUrl    string    `json:"callback_url" sql:",notnull"`
	Timezone       string    `json:"timezone"`
	StartDate      time.Time `json:"start_date,omitempty" sql:",notnull"`
	EndDate        time.Time `json:"end_date,omitempty"`
	DateCreated    time.Time `json:"date_created" sql:",notnull,default:now()"`

	Project ProjectModel `sql:",fk:project_id"`
}
