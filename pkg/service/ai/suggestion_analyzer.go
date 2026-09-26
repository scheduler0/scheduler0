package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"scheduler0/pkg/models"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

// SuggestionAnalyzer analyzes a conversation and returns structured suggestions.
// It is backed by the same scheduler0-edge-classifier service as the intent
// classifier (spaCy + Duckling), just a different endpoint.
type SuggestionAnalyzer interface {
	Analyze(ctx context.Context, req models.SuggestionAnalyzeRequest) (models.SuggestionAnalyzeResult, error)
}

// httpSuggestionAnalyzer calls the scheduler0-edge-classifier FastAPI service.
type httpSuggestionAnalyzer struct {
	client  *http.Client
	logger  hclog.Logger
	baseURL string
}

// NewHTTPSuggestionAnalyzer returns an analyzer client, or nil when no URL is
// configured (which disables the suggestions endpoint entirely). It reuses the
// intent-classifier base URL since both live in the same edge service.
func NewHTTPSuggestionAnalyzer(baseURL string, logger hclog.Logger) SuggestionAnalyzer {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil
	}
	return &httpSuggestionAnalyzer{
		// Longer than the intent guardrail: analysis runs the full spaCy +
		// Duckling pipeline over every message and is the primary call here,
		// not a latency-sensitive pre-check.
		client:  &http.Client{Timeout: 15 * time.Second},
		logger:  logger.Named("suggestion-analyzer"),
		baseURL: baseURL,
	}
}

func (a *httpSuggestionAnalyzer) Analyze(ctx context.Context, reqBody models.SuggestionAnalyzeRequest) (models.SuggestionAnalyzeResult, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return models.SuggestionAnalyzeResult{}, fmt.Errorf("suggestion analyzer: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/suggestions/analyze", bytes.NewReader(body))
	if err != nil {
		return models.SuggestionAnalyzeResult{}, fmt.Errorf("suggestion analyzer: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return models.SuggestionAnalyzeResult{}, fmt.Errorf("suggestion analyzer: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.SuggestionAnalyzeResult{}, fmt.Errorf("suggestion analyzer: read response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return models.SuggestionAnalyzeResult{}, fmt.Errorf("suggestion analyzer: status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed models.SuggestionAnalyzeResult
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return models.SuggestionAnalyzeResult{}, fmt.Errorf("suggestion analyzer: unmarshal response: %w", err)
	}

	return parsed, nil
}
