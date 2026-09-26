package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"scheduler0/pkg/constants"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

// OpenRouterExecutor calls the OpenRouter chat/completions endpoint. OpenRouter is
// OpenAI-compatible but has its own error-body shape, can return provider errors inside
// a 200 OK, and reports real spend in usage.cost.
type OpenRouterExecutor struct {
	client      *http.Client
	logger      hclog.Logger
	baseURL     string
	model       string
	bearerToken string
}

// NewOpenRouterExecutorWithKey creates an OpenRouter executor from explicit credentials (BYOK).
func NewOpenRouterExecutorWithKey(apiKey, model string, logger hclog.Logger) *OpenRouterExecutor {
	if model == "" {
		model = OpenRouterDefaultModel
	}
	return &OpenRouterExecutor{
		client:      &http.Client{Timeout: 30 * time.Second},
		logger:      logger.Named("openrouter-executor"),
		baseURL:     OpenRouterBaseURL,
		model:       model,
		bearerToken: apiKey,
	}
}

func (e *OpenRouterExecutor) ProviderName() string {
	return constants.AIProviderOpenRouter
}

func (e *OpenRouterExecutor) ModelName() string {
	return e.model
}

func (e *OpenRouterExecutor) ExecutePrompt(ctx context.Context, promptConfig SystemPromptConfig, prompt string) (*ExecutionResult, error) {
	if e.bearerToken == "" {
		return nil, fmt.Errorf("openrouter authentication is missing: no API key configured")
	}
	if !IsModelApproved("openrouter", strings.TrimSpace(e.model)) {
		return nil, fmt.Errorf("openrouter model %q is not supported; choose one of the supported models", e.model)
	}
	return e.invokeModel(ctx, e.buildRequestPayload(promptConfig, prompt))
}

// Complete runs a generic JSON completion (json_object response format) with the given
// system and user prompt. Used by the schedule executor selector.
func (e *OpenRouterExecutor) Complete(ctx context.Context, systemPrompt, userPrompt string) (*ExecutionResult, error) {
	if e.bearerToken == "" {
		return nil, fmt.Errorf("openrouter authentication is missing: no API key configured")
	}
	payload := map[string]any{
		"model": e.model,
		"messages": []map[string]string{
			{"role": "system", "content": strings.TrimSpace(systemPrompt)},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":      openAIMaxTokens,
		"temperature":     0,
		"response_format": map[string]any{"type": "json_object"},
	}
	return e.invokeModel(ctx, payload)
}

// buildRequestPayload assembles the chat/completions request. OpenRouter normalises
// max_tokens across all upstream models, so the legacy max_tokens field is always used
// (no need for the max_completion_tokens branch that native OpenAI requires).
func (e *OpenRouterExecutor) buildRequestPayload(promptConfig SystemPromptConfig, prompt string) map[string]any {
	return map[string]any{
		"model": e.model,
		"messages": []map[string]string{
			{"role": "system", "content": strings.TrimSpace(GenerateSystemPrompt(promptConfig))},
			{"role": "user", "content": prompt},
		},
		"max_tokens":  openAIMaxTokens,
		"temperature": 0,
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "prompt_job_responses",
				"strict": true,
				"schema": promptJobResponsesSchema(),
			},
		},
	}
}

func (e *OpenRouterExecutor) invokeModel(ctx context.Context, requestPayload map[string]any) (*ExecutionResult, error) {
	bodyBytes, err := json.Marshal(requestPayload)
	if err != nil {
		e.logger.Error("Failed to marshal OpenRouter request body", "error", err)
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	endpoint := e.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create openrouter request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.bearerToken)
	req.Header.Set("HTTP-Referer", "https://scheduler0.com")
	req.Header.Set("X-Title", "Scheduler0")

	e.logger.Debug("Invoking model", "provider", e.ProviderName(), "model", e.model, "endpoint", endpoint, "body_size", len(bodyBytes))

	resp, err := e.client.Do(req)
	if err != nil {
		e.logger.Error("Failed to invoke OpenRouter model", "error", err)
		return nil, fmt.Errorf("failed to invoke openrouter model: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read openrouter response: %w", err)
	}
	if len(rawBody) == 0 {
		return nil, errors.New("empty response from openrouter")
	}

	// OpenRouter error shape: { "error": { "code": <int>, "message": "...", "metadata": { "error_type": "...", "provider_code": "..." } } }
	// This differs from the native OpenAI shape which has error.type instead of metadata.error_type.
	var envelope struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Metadata *struct {
				ErrorType    string `json:"error_type"`
				ProviderCode string `json:"provider_code"`
			} `json:"metadata"`
		} `json:"error"`
		Choices []struct {
			Message struct {
				Content *string `json:"content"`
			} `json:"message"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
			TotalTokens      int     `json:"total_tokens"`
			Cost             float64 `json:"cost"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal openrouter response: %w", err)
	}

	// Handle HTTP-level errors (4xx/5xx). OpenRouter's error body uses metadata.error_type
	// rather than the OpenAI-style error.type field.
	if resp.StatusCode >= http.StatusBadRequest {
		if envelope.Error != nil && envelope.Error.Message != "" {
			if envelope.Error.Metadata != nil && envelope.Error.Metadata.ErrorType != "" {
				return nil, fmt.Errorf("openrouter api error (%s): %s", envelope.Error.Metadata.ErrorType, envelope.Error.Message)
			}
			return nil, fmt.Errorf("openrouter api error: %s", envelope.Error.Message)
		}
		return nil, fmt.Errorf("openrouter api returned status %d", resp.StatusCode)
	}

	// Even on HTTP 200, OpenRouter may embed a provider error in the response body
	// (e.g. when the upstream LLM fails mid-generation).
	if envelope.Error != nil && envelope.Error.Message != "" {
		if envelope.Error.Metadata != nil && envelope.Error.Metadata.ErrorType != "" {
			return nil, fmt.Errorf("openrouter provider error (%s): %s", envelope.Error.Metadata.ErrorType, envelope.Error.Message)
		}
		return nil, fmt.Errorf("openrouter provider error: %s", envelope.Error.Message)
	}

	if len(envelope.Choices) == 0 {
		return nil, errors.New("openrouter: empty choices")
	}

	// A per-choice error can also carry the failure reason.
	if envelope.Choices[0].Error != nil && envelope.Choices[0].Error.Message != "" {
		return nil, fmt.Errorf("openrouter choice error: %s", envelope.Choices[0].Error.Message)
	}

	if envelope.Choices[0].Message.Content == nil {
		return nil, errors.New("openrouter: null response content")
	}
	content := strings.TrimSpace(*envelope.Choices[0].Message.Content)
	if content == "" {
		return nil, errors.New("openrouter: empty response content")
	}

	totalTokens := envelope.Usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = envelope.Usage.PromptTokens + envelope.Usage.CompletionTokens
	}

	result := &ExecutionResult{
		Text:         content,
		InputTokens:  uint64(max(envelope.Usage.PromptTokens, 0)),
		OutputTokens: uint64(max(envelope.Usage.CompletionTokens, 0)),
		TotalTokens:  uint64(max(totalTokens, 0)),
	}
	if envelope.Usage.Cost > 0 {
		result.ActualCostUSD = &envelope.Usage.Cost
	}
	return result, nil
}
