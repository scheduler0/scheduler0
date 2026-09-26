package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

type OpenAIExecutor struct {
	client      *http.Client
	logger      hclog.Logger
	baseURL     string
	model       string
	bearerToken string
	orgID       string
	projectID   string
}

func NewOpenAIExecutor(cfg *config.Scheduler0Configurations, logger hclog.Logger) *OpenAIExecutor {
	token := strings.TrimSpace(cfg.OpenAIAPIKey)

	baseURL := strings.TrimSpace(cfg.OpenAIBaseURL)
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	model := strings.TrimSpace(cfg.AIPreferredModel)
	if model == "" {
		model = "gpt-4.1-mini"
	}

	return &OpenAIExecutor{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:      logger.Named("openai-executor"),
		baseURL:     strings.TrimRight(baseURL, "/"),
		model:       model,
		bearerToken: token,
		orgID:       strings.TrimSpace(cfg.OpenAIOrganizationID),
		projectID:   strings.TrimSpace(cfg.OpenAIProjectID),
	}
}

// NewOpenAIExecutorWithKey creates an OpenAI executor from explicit credentials (BYOK).
func NewOpenAIExecutorWithKey(apiKey, model string, logger hclog.Logger) *OpenAIExecutor {
	if model == "" {
		model = "gpt-4.1-mini"
	}
	return &OpenAIExecutor{
		client:      &http.Client{Timeout: 30 * time.Second},
		logger:      logger.Named("openai-executor"),
		baseURL:     "https://api.openai.com/v1",
		model:       model,
		bearerToken: apiKey,
	}
}

func (e *OpenAIExecutor) ProviderName() string {
	return constants.AIProviderOpenAI
}

func (e *OpenAIExecutor) ModelName() string {
	return e.model
}

// Token-limit budgets for the chat/completions request. Reasoning-family models
// (GPT-5, o-series) spend part of their completion budget on hidden reasoning
// tokens, so they get a larger allowance to avoid empty final output.
const (
	openAIMaxTokens          = 1024
	openAIReasoningMaxTokens = 4096
)

func (e *OpenAIExecutor) ExecutePrompt(ctx context.Context, promptConfig SystemPromptConfig, prompt string) (*ExecutionResult, error) {
	if e.bearerToken == "" {
		return nil, fmt.Errorf("%s authentication is missing: no API key configured", e.ProviderName())
	}
	return e.invokeModel(ctx, e.buildRequestPayload(promptConfig, prompt))
}

// Complete runs a generic JSON completion (json_object response format) with the given
// system and user prompt. Used by the schedule executor selector.
func (e *OpenAIExecutor) Complete(ctx context.Context, systemPrompt, userPrompt string) (*ExecutionResult, error) {
	if e.bearerToken == "" {
		return nil, fmt.Errorf("%s authentication is missing: no API key configured", e.ProviderName())
	}
	payload := map[string]any{
		"model": e.model,
		"messages": []map[string]string{
			{"role": "system", "content": strings.TrimSpace(systemPrompt)},
			{"role": "user", "content": userPrompt},
		},
		"response_format": map[string]any{"type": "json_object"},
	}
	if modelUsesMaxCompletionTokens(e.model) {
		payload["max_completion_tokens"] = openAIReasoningMaxTokens
	} else {
		payload["max_tokens"] = openAIMaxTokens
		payload["temperature"] = 0
	}
	return e.invokeModel(ctx, payload)
}

// buildRequestPayload assembles the chat/completions request, adapting the token-limit
// parameter and temperature to the target model family. The GPT-5 family and the o-series
// reasoning models reject the legacy "max_tokens" parameter (they require
// "max_completion_tokens") and only accept the default temperature, so those fields are set
// conditionally. Everything else (structured-output response_format) is shared.
func (e *OpenAIExecutor) buildRequestPayload(promptConfig SystemPromptConfig, prompt string) map[string]any {
	payload := map[string]any{
		"model": e.model,
		"messages": []map[string]string{
			{"role": "system", "content": strings.TrimSpace(GenerateSystemPrompt(promptConfig))},
			{"role": "user", "content": prompt},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "prompt_job_responses",
				"strict": true,
				"schema": promptJobResponsesSchema(),
			},
		},
	}

	if modelUsesMaxCompletionTokens(e.model) {
		// Reasoning / next-gen families: newer token param, default temperature only.
		payload["max_completion_tokens"] = openAIReasoningMaxTokens
	} else {
		payload["max_tokens"] = openAIMaxTokens
		payload["temperature"] = 0
	}

	return payload
}

// modelUsesMaxCompletionTokens reports whether a model requires the newer
// "max_completion_tokens" parameter (and rejects "max_tokens"). This covers the GPT-5
// family and the o-series reasoning models (o1, o3, o4, ...). Note that "gpt-4o" is NOT
// an o-series model, so the o-series check deliberately requires a leading bare "o".
func modelUsesMaxCompletionTokens(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if strings.HasPrefix(m, "gpt-5") {
		return true
	}
	// o-series reasoning models are named o<digit>...: o1, o3, o3-pro, o4-mini.
	if len(m) >= 2 && m[0] == 'o' && m[1] >= '1' && m[1] <= '9' {
		return true
	}
	return false
}

func (e *OpenAIExecutor) invokeModel(ctx context.Context, requestPayload map[string]any) (*ExecutionResult, error) {
	bodyBytes, err := json.Marshal(requestPayload)
	if err != nil {
		e.logger.Error("Failed to marshal OpenAI request body", "error", err)
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	endpoint := e.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create openai request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.bearerToken)
	if e.orgID != "" {
		req.Header.Set("OpenAI-Organization", e.orgID)
	}
	if e.projectID != "" {
		req.Header.Set("OpenAI-Project", e.projectID)
	}

	e.logger.Debug("Invoking model", "provider", e.ProviderName(), "model", e.model, "endpoint", endpoint, "body_size", len(bodyBytes))

	resp, err := e.client.Do(req)
	if err != nil {
		e.logger.Error("Failed to invoke OpenAI model", "error", err)
		return nil, fmt.Errorf("failed to invoke openai model: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read openai response: %w", err)
	}
	if len(rawBody) == 0 {
		return nil, errors.New("empty response from openai")
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if json.Unmarshal(rawBody, &apiErr) == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("%s api error (%s): %s", e.ProviderName(), apiErr.Error.Type, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("%s api returned status %d", e.ProviderName(), resp.StatusCode)
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rawBody, &completion); err != nil {
		return nil, fmt.Errorf("failed to unmarshal openai response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, errors.New("openai: empty choices")
	}

	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	if content == "" {
		return nil, errors.New("openai: empty response content")
	}

	totalTokens := completion.Usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = completion.Usage.PromptTokens + completion.Usage.CompletionTokens
	}

	return &ExecutionResult{
		Text:         content,
		InputTokens:  uint64(max(completion.Usage.PromptTokens, 0)),
		OutputTokens: uint64(max(completion.Usage.CompletionTokens, 0)),
		TotalTokens:  uint64(max(totalTokens, 0)),
	}, nil
}
