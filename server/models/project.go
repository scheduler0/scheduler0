package models

import (
	"encoding/json"
	"time"
)

// ProjectModel project model
type ProjectModel struct {
	ID          int64     `json:"id" sql:",pk:notnull"`
	Name        string    `json:"name" sql:",unique,notnull"`
	Description string    `json:"description" sql:",notnull"`
	DateCreated time.Time `json:"date_created" sql:",notnull,default:now()"`
}

// PaginatedProject paginated container of project transformer
type PaginatedProject struct {
	Total  int64          `json:"total"`
	Offset int64          `json:"offset"`
	Limit  int64          `json:"limit"`
	Data   []ProjectModel `json:"projects"`
}

// ToJSON returns content of transformer as JSON
func (projectModel *ProjectModel) ToJSON() ([]byte, error) {
	if data, err := json.Marshal(projectModel); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

// FromJSON extracts content of JSON object into transformer
func (projectModel *ProjectModel) FromJSON(body []byte) error {
	if err := json.Unmarshal(body, &projectModel); err != nil {
		return err
	}
	return nil
}
