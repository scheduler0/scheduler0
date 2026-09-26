package ai

import (
	"strings"
)

// OpenRouter is OpenAI-compatible (POST /chat/completions), so the OpenAIExecutor is reused
// with this base URL plus the attribution headers OpenRouter recommends.
const (
	OpenRouterBaseURL      = "https://openrouter.ai/api/v1"
	OpenRouterDefaultModel = "openai/gpt-4.1-mini"
)

// openRouterPricingByModel is the pricing table for approved OpenRouter models,
// keyed by OpenRouter model id (USD per 1M tokens). Every model in the openrouter
// catalog (models_catalog.go) should have an entry here so cost estimation works;
// both are refreshed together via scripts/update_ai_model_catalog.sh. Used for
// cost estimation only; the approved-model catalog is the source of truth for
// which models are accepted.
var openRouterPricingByModel = map[string]OpenAIPricing{
	// OpenAI
	"openai/gpt-4.1-mini": {InputPer1MTokens: 0.40, OutputPer1MTokens: 1.60},
	"openai/gpt-5.6-sol":  {InputPer1MTokens: 5.00, OutputPer1MTokens: 30.00},
	"openai/gpt-5.6-luna": {InputPer1MTokens: 0.50, OutputPer1MTokens: 3.00},
	// Anthropic
	"anthropic/claude-opus-5":    {InputPer1MTokens: 5.00, OutputPer1MTokens: 25.00},
	"anthropic/claude-sonnet-5":  {InputPer1MTokens: 2.00, OutputPer1MTokens: 10.00},
	"anthropic/claude-haiku-4.5": {InputPer1MTokens: 1.00, OutputPer1MTokens: 5.00},
	// Google
	"google/gemini-3.1-pro-preview": {InputPer1MTokens: 2.00, OutputPer1MTokens: 12.00},
	"google/gemini-3.6-flash":       {InputPer1MTokens: 1.50, OutputPer1MTokens: 7.50},
	// Meta
	"meta-llama/llama-4-maverick": {InputPer1MTokens: 0.20, OutputPer1MTokens: 0.80},
	"meta-llama/llama-4-scout":    {InputPer1MTokens: 0.10, OutputPer1MTokens: 0.30},
	// DeepSeek
	"deepseek/deepseek-v4-pro":   {InputPer1MTokens: 0.435, OutputPer1MTokens: 0.87},
	"deepseek/deepseek-v4-flash": {InputPer1MTokens: 0.14, OutputPer1MTokens: 0.28},
	// Qwen
	"qwen/qwen3.7-max":   {InputPer1MTokens: 1.475, OutputPer1MTokens: 4.425},
	"qwen/qwen3.6-flash": {InputPer1MTokens: 0.1875, OutputPer1MTokens: 1.125},
	// Mistral
	"mistralai/mistral-large-2512":             {InputPer1MTokens: 0.50, OutputPer1MTokens: 1.50},
	"mistralai/mistral-small-3.2-24b-instruct": {InputPer1MTokens: 0.10, OutputPer1MTokens: 0.30},
	// xAI
	"x-ai/grok-4.5": {InputPer1MTokens: 2.00, OutputPer1MTokens: 6.00},
	"x-ai/grok-4.3": {InputPer1MTokens: 1.25, OutputPer1MTokens: 2.50},
	// Z-AI (GLM)
	"z-ai/glm-5.2":       {InputPer1MTokens: 0.677, OutputPer1MTokens: 2.127},
	"z-ai/glm-4.7-flash": {InputPer1MTokens: 0.06, OutputPer1MTokens: 0.40},
	// Moonshot (Kimi)
	"moonshotai/kimi-k3":   {InputPer1MTokens: 3.00, OutputPer1MTokens: 15.00},
	"moonshotai/kimi-k2.6": {InputPer1MTokens: 0.646, OutputPer1MTokens: 2.72},
}

// IsOpenRouterModelAllowed reports whether the given OpenRouter model id is on the
// approved catalog. Delegates to the central catalog (models_catalog.go).
func IsOpenRouterModelAllowed(model string) bool {
	return IsModelApproved("openrouter", strings.TrimSpace(model))
}

// OpenRouterAllowedModelIDs returns the supported OpenRouter model ids, sorted.
// Delegates to the central catalog.
func OpenRouterAllowedModelIDs() []string {
	return ApprovedModelIDs("openrouter")
}

func estimateOpenRouterCostUSD(model string, inputTokens uint64, outputTokens uint64) float64 {
	price, ok := openRouterPricingByModel[strings.TrimSpace(model)]
	if !ok {
		return 0
	}
	inputCost := float64(inputTokens) * (price.InputPer1MTokens / 1_000_000)
	outputCost := float64(outputTokens) * (price.OutputPer1MTokens / 1_000_000)
	return inputCost + outputCost
}
