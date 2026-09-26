package models

import "time"

type ActiveModel struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type AccountAISettings struct {
	AccountID          uint64        `json:"account_id"`
	ActiveModels       []ActiveModel `json:"active_models,omitempty"`
	OpenAIAPIKey       string        `json:"openai_api_key,omitempty"`
	AnthropicAPIKey    string        `json:"anthropic_api_key,omitempty"`
	BedrockAccessKeyID string        `json:"bedrock_access_key_id,omitempty"`
	BedrockSecretKey   string        `json:"bedrock_secret_key,omitempty"`
	BedrockRegion      string        `json:"bedrock_region,omitempty"`
	OpenRouterAPIKey   string        `json:"openrouter_api_key,omitempty"`
	DateCreated        time.Time     `json:"date_created"`
	DateModified       *time.Time    `json:"date_modified,omitempty"`
}
