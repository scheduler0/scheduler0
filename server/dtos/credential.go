package dtos

import (
	"cron-server/server/domains"
	"encoding/json"
)

type CredentialDto struct {
	ID                      string `json:"id" pg:",notnull"`
	ApiKey                  string `json:"api_key" pg:",notnull"`
	HTTPReferrerRestriction string `json:"http_referrer_restriction" pg:",notnull"`
}

func (c *CredentialDto) SearchToQuery([][]string) (string, []string) {
	return "api_key != ?", []string{"null"}
}

func (c *CredentialDto) ToJson() ([]byte, error) {
	if data, err := json.Marshal(c); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

func (c *CredentialDto) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &c); err != nil {
		return err
	}
	return nil
}

func (c *CredentialDto) ToDomain() domains.CredentialDomain {
	credential := domains.CredentialDomain{ID: c.ID, ApiKey: c.ApiKey, HTTPReferrerRestriction: c.HTTPReferrerRestriction}
	return credential
}

func (c *CredentialDto) FromDomain(credentialDomain domains.CredentialDomain) {
	c.ID = credentialDomain.ID
	c.ApiKey = credentialDomain.ApiKey
	c.HTTPReferrerRestriction = credentialDomain.HTTPReferrerRestriction
}
