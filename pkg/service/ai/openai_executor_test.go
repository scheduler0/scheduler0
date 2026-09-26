package ai

import (
	"testing"

	"github.com/hashicorp/go-hclog"
)

func TestModelUsesMaxCompletionTokens(t *testing.T) {
	cases := map[string]bool{
		// classic chat models keep the legacy max_tokens parameter
		"gpt-4.1":         false,
		"gpt-4.1-mini":    false,
		"gpt-4.1-nano":    false,
		"gpt-4o":          false,
		"gpt-4o-mini":     false,
		"gpt-4.5-preview": false,
		// GPT-5 family and o-series require max_completion_tokens
		"gpt-5":        true,
		"gpt-5-mini":   true,
		"gpt-5-nano":   true,
		"gpt-5.4-nano": true,
		"o1":           true,
		"o3":           true,
		"o3-pro":       true,
		"o4-mini":      true,
		// case / whitespace insensitivity
		"  GPT-5-Nano ": true,
	}

	for model, want := range cases {
		if got := modelUsesMaxCompletionTokens(model); got != want {
			t.Errorf("modelUsesMaxCompletionTokens(%q) = %v, want %v", model, got, want)
		}
	}
}

func TestBuildRequestPayload_ClassicModel(t *testing.T) {
	e := NewOpenAIExecutorWithKey("test-key", "gpt-4.1-mini", hclog.NewNullLogger())
	payload := e.buildRequestPayload(SystemPromptConfig{}, "remind me tomorrow")

	if _, ok := payload["max_tokens"]; !ok {
		t.Error("classic model payload should include max_tokens")
	}
	if _, ok := payload["max_completion_tokens"]; ok {
		t.Error("classic model payload should NOT include max_completion_tokens")
	}
	if _, ok := payload["temperature"]; !ok {
		t.Error("classic model payload should include temperature")
	}
	if _, ok := payload["response_format"]; !ok {
		t.Error("payload should always include response_format")
	}
}

func TestBuildRequestPayload_ReasoningModel(t *testing.T) {
	e := NewOpenAIExecutorWithKey("test-key", "gpt-5.4-nano", hclog.NewNullLogger())
	payload := e.buildRequestPayload(SystemPromptConfig{}, "remind me tomorrow")

	if _, ok := payload["max_completion_tokens"]; !ok {
		t.Error("reasoning model payload should include max_completion_tokens")
	}
	if _, ok := payload["max_tokens"]; ok {
		t.Error("reasoning model payload should NOT include max_tokens")
	}
	if _, ok := payload["temperature"]; ok {
		t.Error("reasoning model payload should NOT include temperature (default only)")
	}
	if _, ok := payload["response_format"]; !ok {
		t.Error("payload should always include response_format")
	}
}

func TestEstimateOpenAICost_KnownAndUnknown(t *testing.T) {
	// gpt-5.4-nano is now priced; a 1M/1M split should be input+output rate.
	got := EstimateExecutionCostUSD("openai", "gpt-5.4-nano", 1_000_000, 1_000_000)
	want := 0.20 + 1.25
	if diff := got - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("cost gpt-5.4-nano = %v, want %v", got, want)
	}

	// Unknown model must fall back to 0 rather than error.
	if got := EstimateExecutionCostUSD("openai", "totally-made-up-model", 1000, 1000); got != 0 {
		t.Errorf("unknown model cost = %v, want 0", got)
	}
}
