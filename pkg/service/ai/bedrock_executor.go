package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"scheduler0/pkg/config"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsbedrockruntime "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/hashicorp/go-hclog"
)

type ClaudeSonnetExecutor struct {
	client  *awsbedrockruntime.Client
	logger  hclog.Logger
	modelID string
}

func NewClaudeSonnetExecutor(client *awsbedrockruntime.Client, cfg *config.Scheduler0Configurations, logger hclog.Logger) *ClaudeSonnetExecutor {
	modelID := strings.TrimSpace(cfg.AIBedrockModel)
	if modelID == "" {
		modelID = "global.anthropic.claude-sonnet-4-5-20250929-v1:0"
	}

	return &ClaudeSonnetExecutor{
		client:  client,
		logger:  logger.Named("claude-sonnet-executor"),
		modelID: modelID,
	}
}

// NewClaudeSonnetExecutorWithClient creates a Bedrock executor from an explicit client and model (BYOK).
func NewClaudeSonnetExecutorWithClient(client *awsbedrockruntime.Client, modelID string, logger hclog.Logger) *ClaudeSonnetExecutor {
	if modelID == "" {
		modelID = "global.anthropic.claude-sonnet-4-5-20250929-v1:0"
	}
	return &ClaudeSonnetExecutor{
		client:  client,
		logger:  logger.Named("claude-sonnet-executor"),
		modelID: modelID,
	}
}

func (e *ClaudeSonnetExecutor) ProviderName() string {
	return "bedrock"
}

func (e *ClaudeSonnetExecutor) ModelName() string {
	return e.modelID
}

func (e *ClaudeSonnetExecutor) ExecutePrompt(ctx context.Context, promptConfig SystemPromptConfig, prompt string) (*ExecutionResult, error) {
	if e.client == nil {
		return nil, errors.New("bedrock client is not configured")
	}

	bodyPayload := map[string]any{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        1024,
		"temperature":       0,
		"system":            strings.TrimSpace(GenerateSystemPrompt(promptConfig)),
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": prompt},
				},
			},
		},
	}

	return e.invokeModel(ctx, bodyPayload)
}

// Complete runs a generic completion with the given system and user prompt. Used by the
// schedule executor selector; the JSON shape is requested via the prompt text.
func (e *ClaudeSonnetExecutor) Complete(ctx context.Context, systemPrompt, userPrompt string) (*ExecutionResult, error) {
	if e.client == nil {
		return nil, errors.New("bedrock client is not configured")
	}
	return e.invokeModel(ctx, map[string]any{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        1024,
		"temperature":       0,
		"system":            strings.TrimSpace(systemPrompt),
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": userPrompt},
				},
			},
		},
	})
}

func (e *ClaudeSonnetExecutor) invokeModel(ctx context.Context, bodyPayload map[string]any) (*ExecutionResult, error) {
	bodyBytes, err := json.Marshal(bodyPayload)
	if err != nil {
		e.logger.Error("Failed to marshal request body", "error", err)
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req := &awsbedrockruntime.InvokeModelInput{
		Body:        bodyBytes,
		ModelId:     aws.String(e.modelID),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
	}

	resp, err := e.client.InvokeModel(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke model: %w", err)
	}
	if len(resp.Body) == 0 {
		return nil, errors.New("empty response from bedrock")
	}

	var claudeResponse struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(resp.Body, &claudeResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if len(claudeResponse.Content) == 0 {
		return nil, errors.New("bedrock/anthropic: empty content")
	}
	if claudeResponse.Content[0].Type != "text" {
		return nil, errors.New("bedrock/anthropic: unsupported content type")
	}

	inputTokens := uint64(max(claudeResponse.Usage.InputTokens, 0))
	outputTokens := uint64(max(claudeResponse.Usage.OutputTokens, 0))
	totalTokens := inputTokens + outputTokens

	return &ExecutionResult{
		Text:         strings.TrimSpace(claudeResponse.Content[0].Text),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  totalTokens,
	}, nil
}
