package cmd

import "encoding/json"

type Environment string

const (
	Local      Environment = "Local"
	Staging                = "Staging"
	Production             = "Production"
)

func (env Environment) MarshalJSON() ([]byte, error) {
	return json.Marshal(env)
}

func (env *Environment) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, env)
}

type CloudProvider string

const (
	AWS   CloudProvider = "AWS"
	GCS                 = "GCS"
	Azure               = "Azure"
)

func (provider CloudProvider) MarshalJSON() ([]byte, error) {
	return json.Marshal(provider)
}

func (provider *CloudProvider) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, provider)
}

type Config struct {
	Name           string          `json:"name"`
	Email          string          `json:"email"`
	Environments   []Environment   `json:"environments"`
	CloudProviders []CloudProvider `json:"cloud_providers"`
}
