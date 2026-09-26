package ai

import (
	"context"
)

// ModelExecutor executes prompts against a single provider/model.
type ModelExecutor interface {
	ExecutePrompt(ctx context.Context, promptConfig SystemPromptConfig, prompt string) (*ExecutionResult, error)
	// Complete runs a generic JSON completion with a caller-supplied system and user
	// prompt (no job schema). It is used by the /api/v1/ai/schedule executor selector.
	Complete(ctx context.Context, systemPrompt string, userPrompt string) (*ExecutionResult, error)
	ProviderName() string
	ModelName() string
}

type ExecutionResult struct {
	Text         string
	InputTokens  uint64
	OutputTokens uint64
	TotalTokens  uint64
	// ActualCostUSD is the real spend reported by the provider (e.g. OpenRouter's
	// usage.cost). Nil means the provider did not report a cost; prompt.go falls
	// back to the estimate table in that case.
	ActualCostUSD *float64
}

type ExecutionMetrics struct {
	Provider         string
	Model            string
	InputTokens      uint64
	OutputTokens     uint64
	TotalTokens      uint64
	DurationMs       uint64
	EstimatedCostUSD float64
	Success          bool
	Error            string
}

// promptJobResponsesSchema returns a JSON Schema for the OpenAI structured output response_format.
// OpenAI requires the top-level schema to be type "object", so the job array is wrapped under a
// "jobs" property and unwrapped by parsePromptJobResponses after the model responds.
func promptJobResponsesSchema() map[string]any {
	// OpenAI strict mode rules:
	//   - additionalProperties: false is required on every object at every level
	//   - every property defined must appear in required; optional fields use ["type","null"] unions
	jobItem := map[string]any{
		"type": "object",
		"required": []string{
			"kind",
			"purpose",
			"subject",
			"nextRunAt",
			"recurrence",
			"event",
			"delivery",
			"channel",
			"timezone",
			"recipients",
			"startDate",
			"endDate",
		},
		"properties": map[string]any{
			"kind":       map[string]any{"type": "string", "enum": []string{"FOLLOW_UP", "REMINDER", "DIGEST"}},
			"purpose":    map[string]any{"type": "string"},
			"subject":    map[string]any{"type": "string"},
			"nextRunAt":  map[string]any{"type": "string", "format": "date-time"},
			"recurrence": map[string]any{"type": "string", "enum": []string{"daily", "weekly", "monthly", "yearly", "none"}},
			"event":      map[string]any{"type": "string"},
			"delivery":   map[string]any{"type": "string", "enum": []string{"email", "sms", "slack", "webhook"}},
			"channel":    map[string]any{"type": "string"},
			"timezone":   map[string]any{"type": "string"},
			"recipients": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"startDate":  map[string]any{"type": []string{"string", "null"}, "format": "date-time"},
			"endDate":    map[string]any{"type": []string{"string", "null"}, "format": "date-time"},
		},
		"additionalProperties": false,
	}

	return map[string]any{
		"type":                 "object",
		"required":             []string{"jobs"},
		"properties":           map[string]any{"jobs": map[string]any{"type": "array", "items": jobItem}},
		"additionalProperties": false,
	}
}
