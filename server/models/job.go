package models

import "time"

// JobModel job model
type JobModel struct {
	TableName struct{} `sql:"jobs"`

	ID          int64     `json:"id,omitempty" sql:",pk:notnull"`
	UUID        string    `json:"uuid" sql:",pk:notnull,unique,type:uuid,default:gen_random_uuid()"`
	Archived    bool      `json:"archived" sql:",notnull"`
	ProjectID   int64     `json:"project_id" sql:",notnull"`
	ProjectUUID string    `json:"project_uuid" sql:",notnull,type:uuid"`
	Spec        string    `json:"spec,omitempty" sql:",notnull"`
	CallbackUrl string    `json:"callback_url" sql:",notnull"`
	DateCreated time.Time `json:"date_created" sql:",notnull,default:now()"`

	Project ProjectModel `sql:",fk:project_id"`
}
