package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

const anthropicAPIBase = "https://api.anthropic.com/v1"
const anthropicAPIVersion = "2023-06-01"

// AnthropicExecutor calls the Anthropic Messages API directly (not via Bedrock).
type AnthropicExecutor struct {
	client  *http.Client
	logger  hclog.Logger
	apiKey  string
	model   string
}

func NewAnthropicExecutorWithKey(apiKey, model string, logger hclog.Logger) *AnthropicExecutor {
	if model == "" {
		model = "claude-sonnet-4-5"
	}
	return &AnthropicExecutor{
		client:  &http.Client{Timeout: 30 * time.Second},
		logger:  logger.Named("anthropic-executor"),
		apiKey:  apiKey,
		model:   model,
	}
}

func (e *AnthropicExecutor) ProviderName() string { return "anthropic" }
func (e *AnthropicExecutor) ModelName() string    { return e.model }

func (e *AnthropicExecutor) ExecutePrompt(ctx context.Context, promptConfig SystemPromptConfig, prompt string) (*ExecutionResult, error) {
	if e.apiKey == "" {
		return nil, errors.New("anthropic API key is missing")
	}

	return e.invokeModel(ctx, map[string]any{
		"model":      e.model,
		"max_tokens": 1024,
		"system":     strings.TrimSpace(GenerateSystemPrompt(promptConfig)),
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	})
}

// Complete runs a generic completion with the given system and user prompt. Used by the
// schedule executor selector; the JSON shape is requested via the prompt text.
func (e *AnthropicExecutor) Complete(ctx context.Context, systemPrompt, userPrompt string) (*ExecutionResult, error) {
	if e.apiKey == "" {
		return nil, errors.New("anthropic API key is missing")
	}
	return e.invokeModel(ctx, map[string]any{
		"model":      e.model,
		"max_tokens": 1024,
		"system":     strings.TrimSpace(systemPrompt),
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": userPrompt,
			},
		},
	})
}

func (e *AnthropicExecutor) invokeModel(ctx context.Context, payload map[string]any) (*ExecutionResult, error) {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIBase+"/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", e.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request failed: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: read response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if json.Unmarshal(rawBody, &apiErr) == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("anthropic api error (%s): %s", apiErr.Error.Type, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("anthropic api returned status %d", resp.StatusCode)
	}

	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rawBody, &response); err != nil {
		return nil, fmt.Errorf("anthropic: unmarshal response: %w", err)
	}
	if len(response.Content) == 0 {
		return nil, errors.New("anthropic: empty content")
	}
	if response.Content[0].Type != "text" {
		return nil, errors.New("anthropic: unsupported content type")
	}

	inputTokens := uint64(max(response.Usage.InputTokens, 0))
	outputTokens := uint64(max(response.Usage.OutputTokens, 0))

	return &ExecutionResult{
		Text:         strings.TrimSpace(response.Content[0].Text),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
	}, nil
}
