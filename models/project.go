package models

import (
	"encoding/json"
	"time"
)

// ProjectModel project model
type ProjectModel struct {
	ID          int64     `json:"id,omitempty"`
	Name        string    `json:"name,omitempty"`
	Description string    `json:"description,omitempty"`
	DateCreated time.Time `json:"date_created,omitempty"`
}

// PaginatedProject paginated container of project transformer
type PaginatedProject struct {
	Total  int64          `json:"total,omitempty"`
	Offset int64          `json:"offset,omitempty"`
	Limit  int64          `json:"limit,omitempty"`
	Data   []ProjectModel `json:"projects,omitempty"`
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
