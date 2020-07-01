package models

import "time"

type CredentialModel struct {
	TableName struct{} `sql:"credentials"`

	ID                      string    `json:"id" pg:",notnull"`
	ApiKey                  string    `json:"api_key" pg:",notnull"`
	HTTPReferrerRestriction string    `json:"http_referrer_restriction" pg:",notnull"`
	DateCreated             time.Time `json:"date_created" pg:",notnull"`
}
