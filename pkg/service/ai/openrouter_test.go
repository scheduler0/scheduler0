package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
)

func TestOpenRouterAllowlist(t *testing.T) {
	if !IsOpenRouterModelAllowed("openai/gpt-4.1-mini") {
		t.Error("expected openai/gpt-4.1-mini to be allowed")
	}
	if IsOpenRouterModelAllowed("some/unlisted-model") {
		t.Error("expected unlisted model to be rejected")
	}
	if len(OpenRouterAllowedModelIDs()) == 0 {
		t.Error("expected a non-empty allowlist")
	}
}

func TestOpenRouterExecutor_RejectsUnlistedModel(t *testing.T) {
	e := NewOpenRouterExecutorWithKey("test-key", "some/unlisted-model", hclog.NewNullLogger())
	if e.ProviderName() != "openrouter" {
		t.Errorf("ProviderName = %q, want openrouter", e.ProviderName())
	}
	_, err := e.ExecutePrompt(context.Background(), SystemPromptConfig{}, "remind me tomorrow")
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("expected a 'not supported' error for unlisted model, got %v", err)
	}
}

func TestOpenRouterExecutor_UsesLegacyTokenParam(t *testing.T) {
	// Even for a gpt-5 upstream, the OpenRouter path uses max_tokens (OpenRouter normalizes).
	e := NewOpenRouterExecutorWithKey("test-key", "openai/gpt-5", hclog.NewNullLogger())
	payload := e.buildRequestPayload(SystemPromptConfig{}, "remind me tomorrow")
	if _, ok := payload["max_tokens"]; !ok {
		t.Error("openrouter payload should use max_tokens")
	}
	if _, ok := payload["max_completion_tokens"]; ok {
		t.Error("openrouter payload should NOT use max_completion_tokens")
	}
}

func TestOpenRouterCost(t *testing.T) {
	got := EstimateExecutionCostUSD("openrouter", "openai/gpt-4.1-mini", 1_000_000, 1_000_000)
	want := 0.40 + 1.60
	if got != want {
		t.Errorf("openrouter cost = %v, want %v", got, want)
	}
}

// newOpenRouterExecutorForTest builds an executor pointed at the given test-server URL.
func newOpenRouterExecutorForTest(t *testing.T, serverURL string) *OpenRouterExecutor {
	t.Helper()
	e := NewOpenRouterExecutorWithKey("test-key", "openai/gpt-4.1-mini", hclog.NewNullLogger())
	e.baseURL = serverURL
	return e
}

// validOpenRouterBody produces a minimal successful completion response body.
func validOpenRouterBody(content string, costUSD float64) []byte {
	body := map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": content, "role": "assistant"}},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 5,
			"total_tokens":      15,
			"cost":              costUSD,
		},
	}
	b, _ := json.Marshal(body)
	return b
}

func TestOpenRouterExecutor_4xxErrorShape(t *testing.T) {
	// OpenRouter 4xx error body uses metadata.error_type (not error.type like OpenAI).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		body := map[string]any{
			"error": map[string]any{
				"code":    429,
				"message": "Rate limit exceeded",
				"metadata": map[string]any{
					"error_type":    "rate_limit_exceeded",
					"provider_code": "rate_limited",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	e := newOpenRouterExecutorForTest(t, srv.URL)
	_, err := e.ExecutePrompt(context.Background(), SystemPromptConfig{}, "remind me tomorrow")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rate_limit_exceeded") {
		t.Errorf("expected error_type in message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Rate limit exceeded") {
		t.Errorf("expected error message in output, got: %v", err)
	}
}

func TestOpenRouterExecutor_200WithInBandError(t *testing.T) {
	// OpenRouter can return HTTP 200 with a top-level error object when the upstream LLM fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		body := map[string]any{
			"error": map[string]any{
				"code":    502,
				"message": "Upstream provider unavailable",
				"metadata": map[string]any{
					"error_type": "provider_error",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	e := newOpenRouterExecutorForTest(t, srv.URL)
	_, err := e.ExecutePrompt(context.Background(), SystemPromptConfig{}, "remind me tomorrow")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "provider error") {
		t.Errorf("expected 'provider error' in message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Upstream provider unavailable") {
		t.Errorf("expected upstream message in output, got: %v", err)
	}
}

func TestOpenRouterExecutor_ActualCostUSD(t *testing.T) {
	wantContent := `{"jobs":[]}`
	wantCost := 0.00042
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(validOpenRouterBody(wantContent, wantCost))
	}))
	defer srv.Close()

	e := newOpenRouterExecutorForTest(t, srv.URL)
	result, err := e.ExecutePrompt(context.Background(), SystemPromptConfig{}, "remind me tomorrow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ActualCostUSD == nil {
		t.Fatal("expected ActualCostUSD to be set, got nil")
	}
	if *result.ActualCostUSD != wantCost {
		t.Errorf("ActualCostUSD = %v, want %v", *result.ActualCostUSD, wantCost)
	}
}

func TestOpenRouterExecutor_ZeroCostNotSet(t *testing.T) {
	// When usage.cost is 0 (BYOK keys often return 0), ActualCostUSD should be nil
	// so prompt.go falls back to the estimate table.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(validOpenRouterBody(`{"jobs":[]}`, 0))
	}))
	defer srv.Close()

	e := newOpenRouterExecutorForTest(t, srv.URL)
	result, err := e.ExecutePrompt(context.Background(), SystemPromptConfig{}, "remind me tomorrow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ActualCostUSD != nil {
		t.Errorf("expected ActualCostUSD to be nil for zero cost, got %v", *result.ActualCostUSD)
	}
}

