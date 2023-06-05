package models

import (
	"encoding/json"
	"time"
)

// Credential credential model
type Credential struct {
	ID          uint64    `json:"id,omitempty" fake:"{number:1,100}"`
	Archived    bool      `json:"archived,omitempty"`
	ApiKey      string    `json:"api_key,omitempty" fake:"{regex:[abcdef]{15}}"`
	ApiSecret   string    `json:"api_secret,omitempty" fake:"{regex:[abcdef]{15}}"`
	DateCreated time.Time `json:"date_created,omitempty"`
}

// PaginatedCredential paginated container of credential transformer
type PaginatedCredential struct {
	Total  uint64       `json:"total,omitempty"`
	Offset uint64       `json:"offset,omitempty"`
	Limit  uint64       `json:"limit,omitempty"`
	Data   []Credential `json:"credentials,omitempty"`
}

// ToJSON returns content of transformer as JSON
func (credentialModel *Credential) ToJSON() ([]byte, error) {
	if data, err := json.Marshal(credentialModel); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

// FromJSON extracts content of JSON into transformer
func (credentialModel *Credential) FromJSON(body []byte) error {
	if err := json.Unmarshal(body, credentialModel); err != nil {
		return err
	}
	return nil
}
