package ai

import (
	"sort"
	"strings"
)

type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Default     bool   `json:"default,omitempty"`
}

var approvedModelsByProvider = map[string][]ModelInfo{
	"platform": {
		{ID: "global.anthropic.claude-sonnet-4-5-20250929-v1:0", DisplayName: "Scheduler0 (Claude Sonnet 4.5)", Default: true},
	},
	"openai": {
		{ID: "gpt-4o", DisplayName: "GPT-4o"},
		{ID: "gpt-4o-mini", DisplayName: "GPT-4o Mini"},
		{ID: "gpt-4.1", DisplayName: "GPT-4.1"},
		{ID: "gpt-4.1-mini", DisplayName: "GPT-4.1 Mini", Default: true},
		{ID: "gpt-4.1-nano", DisplayName: "GPT-4.1 Nano"},
		{ID: "gpt-5", DisplayName: "GPT-5"},
		{ID: "gpt-5-mini", DisplayName: "GPT-5 Mini"},
		{ID: "gpt-5-nano", DisplayName: "GPT-5 Nano"},
		{ID: "gpt-5.1", DisplayName: "GPT-5.1"},
		{ID: "gpt-5.2", DisplayName: "GPT-5.2"},
		{ID: "gpt-5.4", DisplayName: "GPT-5.4"},
		{ID: "gpt-5.4-mini", DisplayName: "GPT-5.4 Mini"},
		{ID: "gpt-5.4-nano", DisplayName: "GPT-5.4 Nano"},
		{ID: "gpt-5.5", DisplayName: "GPT-5.5"},
		{ID: "gpt-5.6-luna", DisplayName: "GPT-5.6 Luna"},
		{ID: "gpt-5.6-terra", DisplayName: "GPT-5.6 Terra"},
		{ID: "gpt-5.6-sol", DisplayName: "GPT-5.6 Sol"},
		{ID: "o1", DisplayName: "o1"},
		{ID: "o3", DisplayName: "o3"},
		{ID: "o4-mini", DisplayName: "o4-mini"},
		{ID: "o3-pro", DisplayName: "o3-pro"},
	},
	"anthropic": {
		{ID: "claude-haiku-4-5", DisplayName: "Claude Haiku 4.5"},
		{ID: "claude-sonnet-4-5", DisplayName: "Claude Sonnet 4.5"},
		{ID: "claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6"},
		{ID: "claude-opus-4-5", DisplayName: "Claude Opus 4.5"},
		{ID: "claude-opus-4-6", DisplayName: "Claude Opus 4.6"},
		{ID: "claude-opus-4-7", DisplayName: "Claude Opus 4.7"},
		{ID: "claude-opus-4-8", DisplayName: "Claude Opus 4.8"},
		{ID: "claude-opus-5", DisplayName: "Claude Opus 5"},
		{ID: "claude-sonnet-5", DisplayName: "Claude Sonnet 5", Default: true},
		{ID: "claude-fable-5", DisplayName: "Claude Fable 5"},
	},
	"bedrock": {
		{ID: "global.anthropic.claude-sonnet-4-5-20250929-v1:0", DisplayName: "Claude Sonnet 4.5 (Bedrock)"},
		{ID: "global.anthropic.claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6 (Bedrock)"},
		{ID: "global.anthropic.claude-haiku-4-5-20251001-v1:0", DisplayName: "Claude Haiku 4.5 (Bedrock)"},
		{ID: "global.anthropic.claude-opus-4-6", DisplayName: "Claude Opus 4.6 (Bedrock)"},
		{ID: "global.anthropic.claude-opus-4-7", DisplayName: "Claude Opus 4.7 (Bedrock)"},
		{ID: "global.anthropic.claude-opus-4-8", DisplayName: "Claude Opus 4.8 (Bedrock)"},
		{ID: "global.anthropic.claude-sonnet-5", DisplayName: "Claude Sonnet 5 (Bedrock)", Default: true},
		{ID: "global.anthropic.claude-opus-5", DisplayName: "Claude Opus 5 (Bedrock)"},
		{ID: "global.anthropic.claude-fable-5", DisplayName: "Claude Fable 5 (Bedrock)"},
	},
	"openrouter": {
		{ID: "openai/gpt-4.1-mini", DisplayName: "GPT-4.1 Mini (OpenRouter)", Default: true},
		{ID: "openai/gpt-5.6-sol", DisplayName: "GPT-5.6 Sol (OpenRouter)"},
		{ID: "openai/gpt-5.6-luna", DisplayName: "GPT-5.6 Luna (OpenRouter)"},
		{ID: "anthropic/claude-opus-5", DisplayName: "Claude Opus 5 (OpenRouter)"},
		{ID: "anthropic/claude-sonnet-5", DisplayName: "Claude Sonnet 5 (OpenRouter)"},
		{ID: "anthropic/claude-haiku-4.5", DisplayName: "Claude Haiku 4.5 (OpenRouter)"},
		{ID: "google/gemini-3.1-pro-preview", DisplayName: "Gemini 3.1 Pro Preview (OpenRouter)"},
		{ID: "google/gemini-3.6-flash", DisplayName: "Gemini 3.6 Flash (OpenRouter)"},
		{ID: "meta-llama/llama-4-maverick", DisplayName: "Llama 4 Maverick (OpenRouter)"},
		{ID: "meta-llama/llama-4-scout", DisplayName: "Llama 4 Scout (OpenRouter)"},
		{ID: "deepseek/deepseek-v4-pro", DisplayName: "DeepSeek V4 Pro (OpenRouter)"},
		{ID: "deepseek/deepseek-v4-flash", DisplayName: "DeepSeek V4 Flash (OpenRouter)"},
		{ID: "qwen/qwen3.7-max", DisplayName: "Qwen3.7 Max (OpenRouter)"},
		{ID: "qwen/qwen3.6-flash", DisplayName: "Qwen3.6 Flash (OpenRouter)"},
		{ID: "mistralai/mistral-large-2512", DisplayName: "Mistral Large 2512 (OpenRouter)"},
		{ID: "mistralai/mistral-small-3.2-24b-instruct", DisplayName: "Mistral Small 3.2 24B (OpenRouter)"},
		{ID: "x-ai/grok-4.5", DisplayName: "Grok 4.5 (OpenRouter)"},
		{ID: "x-ai/grok-4.3", DisplayName: "Grok 4.3 (OpenRouter)"},
		{ID: "z-ai/glm-5.2", DisplayName: "GLM 5.2 (OpenRouter)"},
		{ID: "z-ai/glm-4.7-flash", DisplayName: "GLM 4.7 Flash (OpenRouter)"},
		{ID: "moonshotai/kimi-k3", DisplayName: "Kimi K3 (OpenRouter)"},
		{ID: "moonshotai/kimi-k2.6", DisplayName: "Kimi K2.6 (OpenRouter)"},
	},
}

func ApprovedModels(provider string) []ModelInfo {
	return approvedModelsByProvider[strings.ToLower(strings.TrimSpace(provider))]
}

func ApprovedModelsByProvider() map[string][]ModelInfo {
	out := make(map[string][]ModelInfo, len(approvedModelsByProvider))
	for k, v := range approvedModelsByProvider {
		cp := make([]ModelInfo, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func IsModelApproved(provider, model string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	m := strings.TrimSpace(model)
	for _, info := range approvedModelsByProvider[p] {
		if strings.EqualFold(info.ID, m) {
			return true
		}
	}
	return false
}

func DefaultModel(provider string) string {
	for _, info := range approvedModelsByProvider[strings.ToLower(strings.TrimSpace(provider))] {
		if info.Default {
			return info.ID
		}
	}
	return ""
}

func ApprovedModelIDs(provider string) []string {
	models := approvedModelsByProvider[strings.ToLower(strings.TrimSpace(provider))]
	ids := make([]string, 0, len(models))
	for _, m := range models {
		ids = append(ids, m.ID)
	}
	sort.Strings(ids)
	return ids
}
