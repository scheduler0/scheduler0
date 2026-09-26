package models

import "time"

// Prompt-request status values recorded in the account_prompt_requests log.
const (
	PromptRequestStatusSuccess       = "success"
	PromptRequestStatusFailed        = "failed"
	PromptRequestStatusSkippedIntent = "skipped_intent"
)

// AccountPromptRequest is a persisted record of a single AI prompt execution for an account.
// It captures the input, resolved output, the provider/model used, token/cost/duration
// metrics, and the final status so the dashboard can query and search prompt history.
type AccountPromptRequest struct {
	ID               uint64    `json:"id"`
	AccountID        uint64    `json:"account_id"`
	Prompt           string    `json:"prompt"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	Output           string    `json:"output"`
	InputTokens      uint64    `json:"input_tokens"`
	OutputTokens     uint64    `json:"output_tokens"`
	TotalTokens      uint64    `json:"total_tokens"`
	DurationMs       uint64    `json:"duration_ms"`
	EstimatedCostUSD float64   `json:"estimated_cost_usd"`
	Status           string    `json:"status"`
	Error            string    `json:"error,omitempty"`
	DateCreated      time.Time `json:"date_created"`
}
