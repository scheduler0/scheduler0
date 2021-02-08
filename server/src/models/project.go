package models

import "time"

type ProjectModel struct {
	TableName struct{} `sql:"projects"`

	ID          int64     `json:"id" sql:",pk:notnull"`
	UUID        string    `json:"uuid" sql:",pk:notnull,unique,type:uuid,default:gen_random_uuid()"`
	Name        string    `json:"name" sql:",unique,notnull"`
	Description string    `json:"description" sql:",notnull"`
	DateCreated time.Time `json:"date_created" sql:",notnull,default:now()"`
}
