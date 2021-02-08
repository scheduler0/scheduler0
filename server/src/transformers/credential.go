package transformers

import (
	"encoding/json"
	"github.com/victorlenerd/scheduler0/server/src/managers/credential"
)

type Credential struct {
	UUID                    string `json:"uuid"`
	ApiKey                  string `json:"api_key"`
	HTTPReferrerRestriction string `json:"http_referrer_restriction"`
}

func (credentialTransformer *Credential) ToJson() ([]byte, error) {
	if data, err := json.Marshal(credentialTransformer); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

func (credentialTransformer *Credential) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &credentialTransformer); err != nil {
		return err
	}
	return nil
}

func (credentialTransformer *Credential) ToManager() credential.CredentialManager {
	credentialManager := credential.CredentialManager{
		UUID: credentialTransformer.UUID,
		ApiKey: credentialTransformer.ApiKey,
		HTTPReferrerRestriction: credentialTransformer.HTTPReferrerRestriction,
	}
	return credentialManager
}

func (credentialTransformer *Credential) FromManager(credentialManager credential.CredentialManager) {
	credentialTransformer.UUID = credentialManager.UUID
	credentialTransformer.ApiKey = credentialManager.ApiKey
	credentialTransformer.HTTPReferrerRestriction = credentialManager.HTTPReferrerRestriction
}
