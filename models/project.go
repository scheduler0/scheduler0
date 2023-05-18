package models

import (
	"encoding/json"
	"time"
)

// ProjectModel project model
type ProjectModel struct {
	ID          uint64    `json:"id,omitempty" fake:"{number:1,100}"`
	Name        string    `json:"name,omitempty" fake:"{regex:[abcdef]{5}}"`
	Description string    `json:"description,omitempty" fake:"{regex:[abcdef]{5}}"`
	DateCreated time.Time `json:"date_created,omitempty"`
}

// PaginatedProject paginated container of project transformer
type PaginatedProject struct {
	Total  uint64         `json:"total,omitempty"`
	Offset uint64         `json:"offset,omitempty"`
	Limit  uint64         `json:"limit,omitempty"`
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
