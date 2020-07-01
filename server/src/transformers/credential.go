package transformers

import (
	"cron-server/server/src/managers"
	"encoding/json"
)

type Credential struct {
	ID                      string `json:"id" pg:",notnull"`
	ApiKey                  string `json:"api_key" pg:",notnull"`
	HTTPReferrerRestriction string `json:"http_referrer_restriction" pg:",notnull"`
}

func (c *Credential) ToJson() ([]byte, error) {
	if data, err := json.Marshal(c); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

func (c *Credential) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &c); err != nil {
		return err
	}
	return nil
}

func (c *Credential) ToManager() managers.CredentialManager {
	credential := managers.CredentialManager{ID: c.ID, ApiKey: c.ApiKey, HTTPReferrerRestriction: c.HTTPReferrerRestriction}
	return credential
}

func (c *Credential) FromManager(credentialDomain managers.CredentialManager) {
	c.ID = credentialDomain.ID
	c.ApiKey = credentialDomain.ApiKey
	c.HTTPReferrerRestriction = credentialDomain.HTTPReferrerRestriction
}
