package models

import "time"

type ProjectModel struct {
	TableName struct{} `sql:"projects"`

	ID          string    `json:"id" sql:",pk:notnull"`
	Name        string    `json:"name" sql:",notnull"`
	Description string    `json:"description" sql:",notnull"`
	DateCreated time.Time `json:"date_created" sql:",notnull,default:now()"`
}
