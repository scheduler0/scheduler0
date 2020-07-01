package models

import (
	"time"
)

type ProjectModel struct {
	TableName struct{} `sql:"projects"`

	Name        string    `json:"name" pg:",notnull"`
	Description string    `json:"description" pg:",notnull"`
	ID          string    `json:"id" pg:",notnull"`
	DateCreated time.Time `json:"date_created" pg:",notnull"`
}