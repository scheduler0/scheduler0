package models

import (
	"encoding/json"
	"time"
)

// Credential credential model
type Credential struct {
	ID           uint64     `json:"id,omitempty" fake:"{number:1,100}"`
	Archived     bool       `json:"archived,omitempty" fake:"{bool}"`
	ApiKey       string     `json:"apiKey,omitempty" fake:"{regex:[abcdef]{15}}"`
	ApiSecret    string     `json:"-" fake:"{regex:[abcdef]{15}}"`
	DateCreated  time.Time  `json:"dateCreated,omitempty" fake:"{date}"`
	AccountId    uint64     `json:"accountId,omitempty" fake:"{number:1,100}"`
	DateModified *time.Time `json:"dateModified,omitempty" fake:"{date}"`
	CreatedBy    string     `json:"createdBy,omitempty" fake:"{word}"`
	ModifiedBy   *string    `json:"modifiedBy,omitempty" fake:"{word}"`
	DeletedBy    *string    `json:"deletedBy,omitempty" fake:"{word}"`
	ArchivedBy   *string    `json:"archivedBy,omitempty" fake:"{word}"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty" fake:"{date}"`
	Scopes       []string   `json:"scopes,omitempty" fake:"skip"`
	// ExpiresInSeconds is a request-only hint for a shorter, caller-requested TTL
	// (used by the CLI login flow). It is never persisted — the repository writes
	// an explicit column list and the server clamps the value before setting
	// ExpiresAt. A nil value means "use the default expiry".
	ExpiresInSeconds *int64 `json:"expiresInSeconds,omitempty" fake:"skip"`
}

// PaginatedCredential paginated container of credential transformer
type PaginatedCredential struct {
	Total  uint64       `json:"total"`
	Offset uint64       `json:"offset"`
	Limit  uint64       `json:"limit"`
	Data   []Credential `json:"credentials"`
}

// HasScope reports whether the credential carries the given scope.
func (credentialModel *Credential) HasScope(scope string) bool {
	for _, s := range credentialModel.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// IsExpired reports whether the credential is past its expiry. A nil ExpiresAt
// means the credential never expires (system / peer credentials).
func (credentialModel *Credential) IsExpired(now time.Time) bool {
	if credentialModel.ExpiresAt == nil {
		return false
	}
	return !now.Before(*credentialModel.ExpiresAt)
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
	credentialModel.AccountId = 0
	return nil
}
