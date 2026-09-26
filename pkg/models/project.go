package models

import (
	"encoding/json"
	"time"
)

// Project a model representation for projects
type Project struct {
	ID           uint64     `json:"id,omitempty" fake:"{number:1,100}"`
	Name         string     `json:"name,omitempty" fake:"{regex:[abcdef]{5}}"`
	Description  string     `json:"description,omitempty" fake:"{regex:[abcdef]{5}}"`
	DateCreated  time.Time  `json:"dateCreated,omitempty" fake:"{date}"`
	AccountId    uint64     `json:"accountId,omitempty" fake:"{number:1,100}"`
	DateModified *time.Time `json:"dateModified,omitempty" fake:"{date}"`
	CreatedBy    string     `json:"createdBy,omitempty" fake:"{word}"`
	ModifiedBy   *string    `json:"modifiedBy,omitempty" fake:"{word}"`
	DeletedBy    *string    `json:"deletedBy,omitempty" fake:"{word}"`
}

// PaginatedProject paginated container of project transformer
type PaginatedProject struct {
	Total  uint64    `json:"total"`
	Offset uint64    `json:"offset"`
	Limit  uint64    `json:"limit"`
	Data   []Project `json:"projects"`
}

// ToJSON returns content of transformer as JSON
func (projectModel *Project) ToJSON() ([]byte, error) {
	if data, err := json.Marshal(projectModel); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

// FromJSON extracts content of JSON object into transformer
func (projectModel *Project) FromJSON(body []byte) error {
	if err := json.Unmarshal(body, &projectModel); err != nil {
		return err
	}
	projectModel.AccountId = 0
	return nil
}
