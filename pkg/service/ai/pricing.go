package ai

import "strings"

// PlatformTokensPerUSD is the conversion rate from USD to platform tokens.
// 1 platform token = $0.0001 USD, so $1 = 10,000 platform tokens.
// All token balances stored in the database use this unit, making costs
// comparable across model providers regardless of their raw token pricing.
const PlatformTokensPerUSD = 10_000.0

type OpenAIPricing struct {
	InputPer1MTokens  float64
	OutputPer1MTokens float64
}

type BedrockPricing struct {
	InputPer1MTokens  float64
	OutputPer1MTokens float64
}

type AnthropicPricing struct {
	InputPer1MTokens  float64
	OutputPer1MTokens float64
}

var openAIPricingByModel = map[string]OpenAIPricing{
	"gpt-4.1":       {InputPer1MTokens: 2.00, OutputPer1MTokens: 8.00},
	"gpt-4.1-mini":  {InputPer1MTokens: 0.40, OutputPer1MTokens: 1.60},
	"gpt-4.1-nano":  {InputPer1MTokens: 0.10, OutputPer1MTokens: 0.40},
	"gpt-4o":        {InputPer1MTokens: 2.50, OutputPer1MTokens: 10.00},
	"gpt-4o-mini":   {InputPer1MTokens: 0.15, OutputPer1MTokens: 0.60},
	"gpt-5":         {InputPer1MTokens: 1.25, OutputPer1MTokens: 10.00},
	"gpt-5-mini":    {InputPer1MTokens: 0.25, OutputPer1MTokens: 2.00},
	"gpt-5-nano":    {InputPer1MTokens: 0.05, OutputPer1MTokens: 0.40},
	"gpt-5.1":       {InputPer1MTokens: 1.25, OutputPer1MTokens: 10.00},
	"gpt-5.2":       {InputPer1MTokens: 1.75, OutputPer1MTokens: 14.00},
	"gpt-5.4":       {InputPer1MTokens: 2.50, OutputPer1MTokens: 15.00},
	"gpt-5.4-mini":  {InputPer1MTokens: 0.75, OutputPer1MTokens: 4.50},
	"gpt-5.4-nano":  {InputPer1MTokens: 0.20, OutputPer1MTokens: 1.25},
	"gpt-5.5":       {InputPer1MTokens: 5.00, OutputPer1MTokens: 30.00},
	"gpt-5.6-luna":  {InputPer1MTokens: 1.00, OutputPer1MTokens: 6.00},
	"gpt-5.6-terra": {InputPer1MTokens: 2.50, OutputPer1MTokens: 15.00},
	"gpt-5.6-sol":   {InputPer1MTokens: 5.00, OutputPer1MTokens: 30.00},
	"o1":            {InputPer1MTokens: 15.00, OutputPer1MTokens: 60.00},
	"o3":            {InputPer1MTokens: 2.00, OutputPer1MTokens: 8.00},
	"o4-mini":       {InputPer1MTokens: 1.10, OutputPer1MTokens: 4.40},
	"o3-pro":        {InputPer1MTokens: 20.00, OutputPer1MTokens: 80.00},
}

var bedrockPricingByModel = map[string]BedrockPricing{
	"global.anthropic.claude-sonnet-4-5-20250929-v1:0": {InputPer1MTokens: 3.00, OutputPer1MTokens: 15.00},
	"global.anthropic.claude-sonnet-4-6":               {InputPer1MTokens: 3.00, OutputPer1MTokens: 15.00},
	"global.anthropic.claude-haiku-4-5-20251001-v1:0":  {InputPer1MTokens: 1.00, OutputPer1MTokens: 5.00},
	"global.anthropic.claude-opus-4-6":                 {InputPer1MTokens: 5.00, OutputPer1MTokens: 25.00},
	"global.anthropic.claude-opus-4-7":                 {InputPer1MTokens: 5.00, OutputPer1MTokens: 25.00},
	"global.anthropic.claude-opus-4-8":                 {InputPer1MTokens: 5.00, OutputPer1MTokens: 25.00},
	// claude-sonnet-5: introductory $2/$10 through 2026-08-31; standard $3/$15 from 2026-09-01.
	"global.anthropic.claude-sonnet-5": {InputPer1MTokens: 2.00, OutputPer1MTokens: 10.00},
	"global.anthropic.claude-opus-5":   {InputPer1MTokens: 5.00, OutputPer1MTokens: 25.00},
	"global.anthropic.claude-fable-5":  {InputPer1MTokens: 10.00, OutputPer1MTokens: 50.00},
}

// anthropicPricingByModel covers the Anthropic direct API (not via Bedrock).
var anthropicPricingByModel = map[string]AnthropicPricing{
	"claude-haiku-4-5":  {InputPer1MTokens: 1.00, OutputPer1MTokens: 5.00},
	"claude-sonnet-4-5": {InputPer1MTokens: 3.00, OutputPer1MTokens: 15.00},
	"claude-sonnet-4-6": {InputPer1MTokens: 3.00, OutputPer1MTokens: 15.00},
	"claude-opus-4-5":   {InputPer1MTokens: 5.00, OutputPer1MTokens: 25.00},
	"claude-opus-4-6":   {InputPer1MTokens: 5.00, OutputPer1MTokens: 25.00},
	"claude-opus-4-7":   {InputPer1MTokens: 5.00, OutputPer1MTokens: 25.00},
	"claude-opus-4-8":   {InputPer1MTokens: 5.00, OutputPer1MTokens: 25.00},
	"claude-opus-5":     {InputPer1MTokens: 5.00, OutputPer1MTokens: 25.00},
	// claude-sonnet-5: introductory $2/$10 through 2026-08-31; standard $3/$15 from 2026-09-01.
	"claude-sonnet-5": {InputPer1MTokens: 2.00, OutputPer1MTokens: 10.00},
	"claude-fable-5":  {InputPer1MTokens: 10.00, OutputPer1MTokens: 50.00},
}

// EstimateExecutionCostUSD returns the estimated USD cost for a single model execution.
// Add new provider cases here as new integrations are added; billing deduction uses this
// via CostToPlatformTokens and does not need to change.
func EstimateExecutionCostUSD(provider string, model string, inputTokens uint64, outputTokens uint64) float64 {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return estimateOpenAICostUSD(model, inputTokens, outputTokens)
	case "bedrock":
		return estimateBedrockCostUSD(model, inputTokens, outputTokens)
	case "anthropic":
		return estimateAnthropicCostUSD(model, inputTokens, outputTokens)
	case "openrouter":
		return estimateOpenRouterCostUSD(model, inputTokens, outputTokens)
	default:
		return 0
	}
}

// CostToPlatformTokens converts a USD cost to the platform token unit used for billing.
// ceil ensures at least 1 platform token is always deducted for any non-zero cost.
func CostToPlatformTokens(costUSD float64) uint64 {
	if costUSD <= 0 {
		return 0
	}
	raw := costUSD * PlatformTokensPerUSD
	// manual ceil for uint64 conversion
	floor := uint64(raw)
	if float64(floor) < raw {
		return floor + 1
	}
	return floor
}

func estimateOpenAICostUSD(model string, inputTokens uint64, outputTokens uint64) float64 {
	price, ok := openAIPricingByModel[strings.TrimSpace(model)]
	if !ok {
		return 0
	}
	inputCost := float64(inputTokens) * (price.InputPer1MTokens / 1_000_000)
	outputCost := float64(outputTokens) * (price.OutputPer1MTokens / 1_000_000)
	return inputCost + outputCost
}

func estimateBedrockCostUSD(model string, inputTokens uint64, outputTokens uint64) float64 {
	price, ok := bedrockPricingByModel[strings.TrimSpace(model)]
	if !ok {
		return 0
	}
	inputCost := float64(inputTokens) * (price.InputPer1MTokens / 1_000_000)
	outputCost := float64(outputTokens) * (price.OutputPer1MTokens / 1_000_000)
	return inputCost + outputCost
}

func estimateAnthropicCostUSD(model string, inputTokens uint64, outputTokens uint64) float64 {
	price, ok := anthropicPricingByModel[strings.TrimSpace(model)]
	if !ok {
		return 0
	}
	inputCost := float64(inputTokens) * (price.InputPer1MTokens / 1_000_000)
	outputCost := float64(outputTokens) * (price.OutputPer1MTokens / 1_000_000)
	return inputCost + outputCost
}
