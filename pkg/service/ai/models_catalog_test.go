package ai

import (
	"testing"
)

func TestIsModelApproved(t *testing.T) {
	cases := []struct {
		provider string
		model    string
		want     bool
	}{
		// Valid OpenAI models
		{"openai", "gpt-4.1-mini", true},
		{"openai", "gpt-4.1", true},
		{"openai", "gpt-5", true},
		{"openai", "gpt-4o", true},
		{"openai", "o3", true},
		{"openai", "o4-mini", true},
		// Case-insensitive check
		{"OPENAI", "GPT-4.1-MINI", true},
		// Invalid OpenAI model
		{"openai", "gpt-3.5-turbo", false},
		// Valid Anthropic models
		{"anthropic", "claude-sonnet-4-5", true},
		{"anthropic", "claude-opus-4-5", true},
		{"anthropic", "claude-haiku-4-5", true},
		// Invalid Anthropic model
		{"anthropic", "claude-2", false},
		// Valid Bedrock models
		{"bedrock", "global.anthropic.claude-haiku-4-5-20251001-v1:0", true},
		{"bedrock", "global.anthropic.claude-sonnet-4-5-20250929-v1:0", true},
		// Invalid Bedrock model
		{"bedrock", "some-random-model", false},
		// Valid OpenRouter models — the curated list spans many upstream vendors,
		// not just OpenAI/Anthropic.
		{"openrouter", "openai/gpt-4.1-mini", true},
		{"openrouter", "anthropic/claude-haiku-4.5", true},
		{"openrouter", "google/gemini-3.6-flash", true},
		{"openrouter", "meta-llama/llama-4-maverick", true},
		{"openrouter", "deepseek/deepseek-v4-pro", true},
		{"openrouter", "x-ai/grok-4.5", true},
		{"openrouter", "mistralai/mistral-large-2512", true},
		// The old invalid Anthropic Haiku id (never on OpenRouter) stays rejected.
		{"openrouter", "anthropic/claude-3.5-haiku", false},
		// A model not on the curated list is rejected.
		{"openrouter", "some-vendor/some-model", false},
		// Unknown provider
		{"unknown", "gpt-4.1-mini", false},
	}

	for _, tc := range cases {
		got := IsModelApproved(tc.provider, tc.model)
		if got != tc.want {
			t.Errorf("IsModelApproved(%q, %q) = %v, want %v", tc.provider, tc.model, got, tc.want)
		}
	}
}

func TestApprovedModels_ReturnsNilForUnknownProvider(t *testing.T) {
	if ApprovedModels("unknown") != nil {
		t.Error("expected nil for unknown provider")
	}
}

func TestApprovedModelsByProvider_AllProvidersPresent(t *testing.T) {
	catalog := ApprovedModelsByProvider()
	for _, p := range []string{"openai", "anthropic", "bedrock", "openrouter"} {
		if len(catalog[p]) == 0 {
			t.Errorf("expected non-empty model list for provider %q", p)
		}
	}
}

func TestDefaultModel(t *testing.T) {
	cases := []struct {
		provider string
		want     string
	}{
		{"openai", "gpt-4.1-mini"},
		{"anthropic", "claude-sonnet-5"},
		{"bedrock", "global.anthropic.claude-sonnet-5"},
		{"openrouter", "openai/gpt-4.1-mini"},
		{"unknown", ""},
	}
	for _, tc := range cases {
		got := DefaultModel(tc.provider)
		if got != tc.want {
			t.Errorf("DefaultModel(%q) = %q, want %q", tc.provider, got, tc.want)
		}
	}
}

func TestApprovedModelIDs_Sorted(t *testing.T) {
	ids := ApprovedModelIDs("openai")
	if len(ids) == 0 {
		t.Fatal("expected non-empty list for openai")
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] < ids[i-1] {
			t.Errorf("ApprovedModelIDs not sorted: %q < %q", ids[i], ids[i-1])
		}
	}
}

func TestIsOpenRouterModelAllowed_DelegatesToCatalog(t *testing.T) {
	if !IsOpenRouterModelAllowed("openai/gpt-4.1-mini") {
		t.Error("expected openai/gpt-4.1-mini to be allowed via IsOpenRouterModelAllowed")
	}
	if IsOpenRouterModelAllowed("some-vendor/not-a-real-model") {
		t.Error("expected an unlisted model to be rejected via IsOpenRouterModelAllowed")
	}
}
