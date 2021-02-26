package transformers

import (
	"encoding/json"
	"scheduler0/server/managers/credential"
)

// Credential transform into or from Managers or JSONs
type Credential struct {
	UUID                          string `json:"uuid"`
	Platform                      string `json:"platform"`
	ApiKey                        string `json:"api_key"`
	ApiSecret                     string `json:"api_secret"`
	HTTPReferrerRestriction       string `json:"http_referrer_restriction"`
	IPRestrictionRestriction      string `json:"ip_restriction"`
	IOSBundleIDRestriction        string `json:"ios_bundle_id_restriction"`
	AndroidPackageNameRestriction string `json:"android_package_name_restriction"`
}

// PaginatedCredential paginated container of credential transformer
type PaginatedCredential struct {
	Total  int          `json:"total"`
	Offset int          `json:"offset"`
	Limit  int          `json:"limit"`
	Data   []Credential `json:"credentials"`
}

// ToJSON returns content of transformer as JSON
func (credentialTransformer *Credential) ToJSON() ([]byte, error) {
	if data, err := json.Marshal(credentialTransformer); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

// FromJSON extracts content of JSON into transformer
func (credentialTransformer *Credential) FromJSON(body []byte) error {
	if err := json.Unmarshal(body, &credentialTransformer); err != nil {
		return err
	}
	return nil
}

// ToManager returns content of transformer as manager
func (credentialTransformer *Credential) ToManager() credential.Manager {
	credentialManager := credential.Manager{
		UUID:                          credentialTransformer.UUID,
		Platform:                      credentialTransformer.Platform,
		ApiKey:                        credentialTransformer.ApiKey,
		ApiSecret:                     credentialTransformer.ApiSecret,
		HTTPReferrerRestriction:       credentialTransformer.HTTPReferrerRestriction,
		IPRestriction:                 credentialTransformer.IPRestrictionRestriction,
		IOSBundleIDRestriction:        credentialTransformer.IOSBundleIDRestriction,
		AndroidPackageNameRestriction: credentialTransformer.AndroidPackageNameRestriction,
	}
	return credentialManager
}

// FromManager extracts content from manager into transformer
func (credentialTransformer *Credential) FromManager(credentialManager credential.Manager) {
	credentialTransformer.UUID = credentialManager.UUID
	credentialTransformer.Platform = credentialManager.Platform
	credentialTransformer.ApiKey = credentialManager.ApiKey
	credentialTransformer.ApiSecret = credentialManager.ApiSecret
	credentialTransformer.IPRestrictionRestriction = credentialManager.IPRestriction
	credentialTransformer.HTTPReferrerRestriction = credentialManager.HTTPReferrerRestriction
	credentialTransformer.IOSBundleIDRestriction = credentialManager.IOSBundleIDRestriction
	credentialTransformer.AndroidPackageNameRestriction = credentialManager.AndroidPackageNameRestriction
}
