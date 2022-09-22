package models

import (
	"encoding/json"
	"time"
)

// CredentialModel credential model
type CredentialModel struct {
	ID                            int64     `json:"id" sql:",pk:notnull"`
	Archived                      bool      `json:"archived" sql:",notnull"`
	Platform                      string    `json:"platform" sql:",notnull"`
	ApiKey                        string    `json:"api_key" sql:",null"`
	ApiSecret                     string    `json:"api_secret" sql:",null"`
	IPRestriction                 string    `json:"ip_restriction" sql:",null"`
	HTTPReferrerRestriction       string    `json:"http_referrer_restriction" sql:",null"`
	IOSBundleIDRestriction        string    `json:"ios_bundle_id_restriction" sql:",null"`
	AndroidPackageNameRestriction string    `json:"android_package_name_restriction" sql:",null"`
	DateCreated                   time.Time `json:"date_created" sql:",notnull,default:now()"`
}

// PaginatedCredential paginated container of credential transformer
type PaginatedCredential struct {
	Total  int64             `json:"total"`
	Offset int64             `json:"offset"`
	Limit  int64             `json:"limit"`
	Data   []CredentialModel `json:"credentials"`
}

// ToJSON returns content of transformer as JSON
func (credentialModel *CredentialModel) ToJSON() ([]byte, error) {
	if data, err := json.Marshal(credentialModel); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

// FromJSON extracts content of JSON into transformer
func (credentialModel *CredentialModel) FromJSON(body []byte) error {
	if err := json.Unmarshal(body, &credentialModel); err != nil {
		return err
	}
	return nil
}
