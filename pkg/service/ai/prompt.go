package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	"strconv"
	"strings"
	"sync"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscredentials "github.com/aws/aws-sdk-go-v2/credentials"
	awsbedrockruntime "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/hashicorp/go-hclog"
)

type PromptRequestRecorder interface {
	Record(models.AccountPromptRequest) error
}

type PlatformCreditBiller interface {
	ChargePlatformRun(accountID uint64, provider string, model string, costUSD float64)
}

type PromptService struct {
	globalExecutors    []ModelExecutor
	globalCfg          *config.Scheduler0Configurations
	bedrockClient      *awsbedrockruntime.Client
	recorder           PromptRequestRecorder
	creditBiller       PlatformCreditBiller
	classifier         IntentClassifier
	suggestionAnalyzer SuggestionAnalyzer
	lastExecutions     []ExecutionMetrics
	logger             hclog.Logger
	mu                 sync.RWMutex
}

func (s *PromptService) SetCreditBiller(biller PlatformCreditBiller) {
	s.creditBiller = biller
}

func NewPromptService(logger hclog.Logger, cfg *config.Scheduler0Configurations, bedrockClient *awsbedrockruntime.Client, recorder PromptRequestRecorder) *PromptService {
	executors := buildModelExecutors(cfg, bedrockClient, logger)
	return &PromptService{
		globalExecutors:    executors,
		globalCfg:          cfg,
		bedrockClient:      bedrockClient,
		recorder:           recorder,
		classifier:         NewHTTPIntentClassifier(cfg.AIIntentClassifierURL, logger),
		suggestionAnalyzer: NewHTTPSuggestionAnalyzer(cfg.AIIntentClassifierURL, logger),
		logger:             logger,
	}
}

func (s *PromptService) SuggestionAnalyzerConfigured() bool {
	return s.suggestionAnalyzer != nil
}

func (s *PromptService) AnalyzeSuggestions(ctx context.Context, req models.SuggestionAnalyzeRequest) (models.SuggestionAnalyzeResult, error) {
	if s.suggestionAnalyzer == nil {
		return models.SuggestionAnalyzeResult{}, errors.New("suggestion analyzer is not configured")
	}
	return s.suggestionAnalyzer.Analyze(ctx, req)
}

func validatePromptInputs(purposes []string, events []string, recipients []string, channels []string, timezone string) error {
	if len(purposes) > constants.PromptListMaxItems {
		return fmt.Errorf("purposes must have %d or fewer items", constants.PromptListMaxItems)
	}
	if len(events) > constants.PromptListMaxItems {
		return fmt.Errorf("events must have %d or fewer items", constants.PromptListMaxItems)
	}
	if len(recipients) > constants.PromptListMaxItems {
		return fmt.Errorf("recipients must have %d or fewer items", constants.PromptListMaxItems)
	}
	if len(channels) > constants.PromptListMaxItems {
		return fmt.Errorf("channels must have %d or fewer items", constants.PromptListMaxItems)
	}

	for _, purpose := range purposes {
		if len(purpose) > constants.PromptListItemMaxLength {
			return fmt.Errorf("purpose must be less than %d characters long", constants.PromptListItemMaxLength)
		}
	}
	for _, event := range events {
		if len(event) > constants.PromptListItemMaxLength {
			return fmt.Errorf("event must be less than %d characters long", constants.PromptListItemMaxLength)
		}
	}
	for _, recipient := range recipients {
		if len(recipient) > constants.PromptListItemMaxLength {
			return fmt.Errorf("recipient must be less than %d characters long", constants.PromptListItemMaxLength)
		}
	}
	for _, channel := range channels {
		if len(channel) > constants.PromptListItemMaxLength {
			return fmt.Errorf("channel must be less than %d characters long", constants.PromptListItemMaxLength)
		}
	}

	if tz := strings.TrimSpace(timezone); tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return fmt.Errorf("invalid timezone %q: %w", tz, err)
		}
	}

	return nil
}

func (s *PromptService) CreateJobFromPrompt(ctx context.Context, accountID uint64, prompt string, purposes []string, events []string, recipients []string, channels []string, timezone string, locale string) ([]models.PromptProviderResult, *models.IntentClassification, uint64, float64, error) {
	return s.createJobFromPromptWithExecutors(ctx, accountID, s.globalExecutors, false, prompt, purposes, events, recipients, channels, timezone, locale)
}

func (s *PromptService) CreateJobFromPromptWithSettings(ctx context.Context, accountID uint64, settings *models.AccountAISettings, prompt string, purposes []string, events []string, recipients []string, channels []string, timezone string, locale string) ([]models.PromptProviderResult, *models.IntentClassification, uint64, float64, error) {
	executors := s.resolveExecutors(settings)
	return s.createJobFromPromptWithExecutors(ctx, accountID, executors, true, prompt, purposes, events, recipients, channels, timezone, locale)
}

func (s *PromptService) ClassifyPrompt(ctx context.Context, text string) (models.IntentClassification, error) {
	if s.classifier == nil {
		return models.IntentClassification{}, fmt.Errorf("intent classifier is not configured")
	}
	return s.classifier.Classify(ctx, text)
}

func (s *PromptService) resolveExecutors(settings *models.AccountAISettings) []ModelExecutor {
	active := activeModelsFromSettings(settings)
	if len(active) == 0 {
		if pe := s.newPlatformExecutor(""); pe != nil {
			return []ModelExecutor{pe}
		}
		return s.globalExecutors
	}

	executors := make([]ModelExecutor, 0, len(active))
	for _, am := range active {
		if strings.ToLower(strings.TrimSpace(am.Provider)) == constants.AIProviderPlatform {
			if pe := s.newPlatformExecutor(am.Model); pe != nil {
				executors = append(executors, pe)
			} else {
				s.logger.Warn("resolveExecutors: platform provider selected but hosted model unavailable — skipping entry")
			}
			continue
		}
		executors = append(executors, buildExecutorForEntry(am, settings, s.logger)...)
	}
	return executors
}

func (s *PromptService) newPlatformExecutor(model string) ModelExecutor {
	if !PlatformAIEnabled(s.globalCfg, s.bedrockClient) {
		return nil
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = PlatformModelID(s.globalCfg)
	}
	inner := NewClaudeSonnetExecutorWithClient(s.bedrockClient, model, s.logger)
	return platformModelExecutor{ModelExecutor: inner}
}

func (s *PromptService) PrimaryExecutorIsPlatform(settings *models.AccountAISettings) bool {
	execs := s.resolveExecutors(settings)
	if len(execs) == 0 {
		return false
	}
	_, ok := execs[0].(PlatformBillable)
	return ok
}

func activeModelsFromSettings(settings *models.AccountAISettings) []models.ActiveModel {
	if settings == nil {
		return nil
	}
	return settings.ActiveModels
}

func buildExecutorsFromSettings(settings *models.AccountAISettings, logger hclog.Logger) []ModelExecutor {
	active := activeModelsFromSettings(settings)
	executors := make([]ModelExecutor, 0, len(active))
	for _, am := range active {
		executors = append(executors, buildExecutorForEntry(am, settings, logger)...)
	}
	return executors
}

func buildExecutorForEntry(am models.ActiveModel, settings *models.AccountAISettings, logger hclog.Logger) []ModelExecutor {
	provider := strings.ToLower(strings.TrimSpace(am.Provider))
	model := strings.TrimSpace(am.Model)

	switch provider {
	case constants.AIProviderOpenAI:
		key := strings.TrimSpace(settings.OpenAIAPIKey)
		if key == "" {
			logger.Warn("buildExecutorForEntry: openai entry skipped — no API key set")
			return nil
		}
		return []ModelExecutor{NewOpenAIExecutorWithKey(key, model, logger)}

	case constants.AIProviderAnthropic:
		key := strings.TrimSpace(settings.AnthropicAPIKey)
		if key == "" {
			logger.Warn("buildExecutorForEntry: anthropic entry skipped — no API key set")
			return nil
		}
		return []ModelExecutor{NewAnthropicExecutorWithKey(key, model, logger)}

	case constants.AIProviderOpenRouter:
		key := strings.TrimSpace(settings.OpenRouterAPIKey)
		if key == "" {
			logger.Warn("buildExecutorForEntry: openrouter entry skipped — no API key set")
			return nil
		}
		return []ModelExecutor{NewOpenRouterExecutorWithKey(key, model, logger)}

	case constants.AIProviderBedrock:
		accessKey := strings.TrimSpace(settings.BedrockAccessKeyID)
		secretKey := strings.TrimSpace(settings.BedrockSecretKey)
		region := strings.TrimSpace(settings.BedrockRegion)
		if accessKey == "" || secretKey == "" {
			logger.Warn("buildExecutorForEntry: bedrock entry skipped — credentials incomplete")
			return nil
		}
		if region == "" {
			region = constants.AIDefaultBedrockRegion
		}
		awsCfg, err := awsconfig.LoadDefaultConfig(
			context.Background(),
			awsconfig.WithRegion(region),
			awsconfig.WithCredentialsProvider(awscredentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		)
		if err != nil {
			logger.Error("buildExecutorForEntry: failed to build bedrock AWS config", "error", err)
			return nil
		}
		client := awsbedrockruntime.NewFromConfig(awsCfg)
		return []ModelExecutor{NewClaudeSonnetExecutorWithClient(client, model, logger)}

	default:
		logger.Warn("buildExecutorForEntry: unknown provider, skipping", "provider", provider)
		return nil
	}
}

func (s *PromptService) createJobFromPromptWithExecutors(ctx context.Context, accountID uint64, executors []ModelExecutor, failover bool, prompt string, purposes []string, events []string, recipients []string, channels []string, timezone string, locale string) ([]models.PromptProviderResult, *models.IntentClassification, uint64, float64, error) {
	prompt = strings.TrimSpace(prompt)
	timezone = strings.TrimSpace(timezone)
	locale = strings.TrimSpace(locale)
	if locale == "" {
		locale = "en"
	}

	if len(prompt) == 0 {
		return nil, nil, 0, 0, errors.New("prompt is required")
	}

	if len(prompt) > constants.PromptMaxLength {
		return nil, nil, 0, 0, fmt.Errorf("prompt must be less than %d characters", constants.PromptMaxLength)
	}

	if err := validatePromptInputs(purposes, events, recipients, channels, timezone); err != nil {
		s.logger.Warn("Invalid prompt inputs", "error", err)
		return nil, nil, 0, 0, err
	}

	var classification *models.IntentClassification
	if s.classifier != nil && IsEnglishLocale(locale) {
		decision, classifyErr := s.classifier.Classify(ctx, prompt)
		if classifyErr != nil {
			s.logger.Warn("intent classifier failed, proceeding (fail-open)", "error", classifyErr)
		} else {
			classification = &decision
			if decision.Decision == IntentDecisionReject || decision.Decision == IntentDecisionClarify {
				s.logger.Info("intent guardrail skipped model execution",
					"decision", decision.Decision, "reason", decision.Reason)
				s.recordSkippedIntent(accountID, prompt, decision)
				return nil, classification, 0, 0, &IntentSkippedError{Decision: decision.Decision, Reason: decision.Reason, Classification: classification}
			}
		}
	} else if s.classifier != nil {
		s.logger.Info("intent guardrail skipped: non-English locale", "locale", locale)
	}

	s.logger.Debug("Creating jobs from prompt",
		"purposes_count", len(purposes),
		"events_count", len(events),
		"recipients_count", len(recipients),
		"channels_count", len(channels),
		"timezone", timezone,
		"locale", locale,
	)

	promptConfig := SystemPromptConfig{
		Recipients: recipients,
		Channels:   channels,
		Events:     events,
		Purposes:   purposes,
		Timezone:   timezone,
		Locale:     locale,
	}

	var (
		lastErr         error
		totalUsed       uint64
		totalCostUSD    float64
		providerResults []models.PromptProviderResult
	)
	if len(executors) == 0 {
		return nil, classification, 0, 0, errors.New("no prompt executors configured")
	}

	executionMetrics := make([]ExecutionMetrics, 0, len(executors))

	for i, executor := range executors {
		s.logger.Debug("Executing prompt with executor", "executor_index", i, "executor_count", len(executors))
		startedAt := time.Now()
		result, execErr := executor.ExecutePrompt(ctx, promptConfig, prompt)
		durationMs := uint64(time.Since(startedAt).Milliseconds())

		if execErr != nil {
			s.logger.Error("Executor failed", "error", execErr, "executor_index", i)
			executionMetrics = append(executionMetrics, ExecutionMetrics{
				Provider:   executor.ProviderName(),
				Model:      executor.ModelName(),
				DurationMs: durationMs,
				Success:    false,
				Error:      execErr.Error(),
			})
			lastErr = execErr
			continue
		}

		cost := EstimateExecutionCostUSD(executor.ProviderName(), executor.ModelName(), result.InputTokens, result.OutputTokens)
		if result.ActualCostUSD != nil && *result.ActualCostUSD > 0 {
			cost = *result.ActualCostUSD
		}
		totalUsed += result.TotalTokens
		totalCostUSD += cost
		executionMetrics = append(executionMetrics, ExecutionMetrics{
			Provider:         executor.ProviderName(),
			Model:            executor.ModelName(),
			InputTokens:      result.InputTokens,
			OutputTokens:     result.OutputTokens,
			TotalTokens:      result.TotalTokens,
			DurationMs:       durationMs,
			EstimatedCostUSD: cost,
			Success:          true,
		})

		s.logger.Info("Executor completed",
			"provider", executor.ProviderName(),
			"model", executor.ModelName(),
			"input_tokens", result.InputTokens,
			"output_tokens", result.OutputTokens,
			"total_tokens", result.TotalTokens,
			"duration_ms", durationMs,
			"estimated_cost_usd", fmt.Sprintf("%.8f", cost),
		)

		if _, isPlatform := executor.(PlatformBillable); isPlatform && s.creditBiller != nil {
			s.creditBiller.ChargePlatformRun(accountID, executor.ProviderName(), executor.ModelName(), cost)
		}

		jobs, parseErr := parsePromptJobResponses(result.Text)
		if parseErr != nil {
			lastErr = parseErr
			s.logger.Warn("Executor response parse failed",
				"provider", executor.ProviderName(),
				"model", executor.ModelName(),
				"error", parseErr,
			)
			continue
		}

		providerResults = append(providerResults, models.PromptProviderResult{
			Provider:     executor.ProviderName(),
			Model:        executor.ModelName(),
			Jobs:         jobs,
			InputTokens:  result.InputTokens,
			OutputTokens: result.OutputTokens,
			TotalTokens:  result.TotalTokens,
			DurationMs:   durationMs,
		})

		if failover {
			break
		}
	}

	s.setLastExecutionMetrics(executionMetrics)
	if len(providerResults) > 0 {
		s.recordPromptRequest(accountID, prompt, executionMetrics, providerResults, totalUsed, totalCostUSD, nil)
		return providerResults, classification, totalUsed, totalCostUSD, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no prompt executors configured")
	}
	finalErr := fmt.Errorf("all prompt executors failed: %w", lastErr)
	s.recordPromptRequest(accountID, prompt, executionMetrics, nil, totalUsed, totalCostUSD, finalErr)
	return nil, classification, totalUsed, totalCostUSD, finalErr
}

func (s *PromptService) recordPromptRequest(
	accountID uint64,
	prompt string,
	metrics []ExecutionMetrics,
	providerResults []models.PromptProviderResult,
	totalTokens uint64,
	totalCostUSD float64,
	execErr error,
) {
	if s.recorder == nil {
		return
	}

	record := models.AccountPromptRequest{
		AccountID:        accountID,
		Prompt:           prompt,
		TotalTokens:      totalTokens,
		EstimatedCostUSD: totalCostUSD,
		DateCreated:      time.Now().UTC(),
	}

	if len(metrics) > 0 {
		record.Provider = metrics[0].Provider
		record.Model = metrics[0].Model
		for _, m := range metrics {
			record.InputTokens += m.InputTokens
			record.OutputTokens += m.OutputTokens
			record.DurationMs += m.DurationMs
		}
	}

	if execErr != nil {
		record.Status = models.PromptRequestStatusFailed
		record.Error = execErr.Error()
	} else {
		record.Status = models.PromptRequestStatusSuccess
		if out, marshalErr := json.Marshal(providerResults); marshalErr == nil {
			record.Output = string(out)
		}
	}

	if err := s.recorder.Record(record); err != nil {
		s.logger.Warn("failed to record prompt request", "error", err, "accountID", accountID)
	}
}

func (s *PromptService) recordSkippedIntent(accountID uint64, prompt string, decision models.IntentClassification) {
	if s.recorder == nil {
		return
	}
	if err := s.recorder.Record(models.AccountPromptRequest{
		AccountID:   accountID,
		Prompt:      prompt,
		Status:      models.PromptRequestStatusSkippedIntent,
		Output:      decision.Reason,
		Error:       fmt.Sprintf("intent=%s", decision.Decision),
		DateCreated: time.Now().UTC(),
	}); err != nil {
		s.logger.Warn("failed to record skipped-intent prompt request", "error", err, "accountID", accountID)
	}
}

func createCronExpressionFromRecurrence(recurrence models.PromptJobResponseRecurrence, nextRunAt time.Time) string {

	normalized := strings.ToLower(string(recurrence))

	switch normalized {
	case "minute", "every minute":
		return "* * * * *"
	case "hourly", "every hour":
		return fmt.Sprintf("%d * * * *", nextRunAt.Minute())
	case "daily", "every day":
		return fmt.Sprintf("%d %d * * *", nextRunAt.Minute(), nextRunAt.Hour())
	case "weekly", "every week":
		weekday := int(nextRunAt.Weekday())
		return fmt.Sprintf("%d %d * * %d", nextRunAt.Minute(), nextRunAt.Hour(), weekday)
	case "monthly", "every month":
		return fmt.Sprintf("%d %d %d * *", nextRunAt.Minute(), nextRunAt.Hour(), nextRunAt.Day())
	case "yearly", "every year":
		return fmt.Sprintf("%d %d %d %d *", nextRunAt.Minute(), nextRunAt.Hour(), nextRunAt.Day(), int(nextRunAt.Month()))
	case "none", "no":
		return fmt.Sprintf("%d %d %d %d %d",
			nextRunAt.Minute(),
			nextRunAt.Hour(),
			nextRunAt.Day(),
			int(nextRunAt.Month()),
			int(nextRunAt.Weekday()))
	}
	return ""
}

func (s *PromptService) LastExecutionMetrics() []ExecutionMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]ExecutionMetrics, len(s.lastExecutions))
	copy(cp, s.lastExecutions)
	return cp
}

func (s *PromptService) setLastExecutionMetrics(metrics []ExecutionMetrics) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastExecutions = make([]ExecutionMetrics, len(metrics))
	copy(s.lastExecutions, metrics)
}

func assignCronExpressions(jobs []models.PromptJobResponse) []models.PromptJobResponse {
	for i, job := range jobs {
		if job.NextRunAt != nil {
			jobs[i].CronExpression = createCronExpressionFromRecurrence(job.Recurrence, *job.NextRunAt)
		}
	}
	return jobs
}

func parsePromptJobResponses(text string) ([]models.PromptJobResponse, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("empty model output")
	}

	stripFences := func(v string) string {
		v = strings.TrimSpace(v)
		v = strings.TrimPrefix(v, "```json")
		v = strings.TrimPrefix(v, "```JSON")
		v = strings.TrimPrefix(v, "```")
		v = strings.TrimSuffix(v, "```")
		return strings.TrimSpace(v)
	}

	clean := stripFences(text)

	var envelope struct {
		Jobs []models.PromptJobResponse `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(clean), &envelope); err == nil && len(envelope.Jobs) > 0 {
		return assignCronExpressions(envelope.Jobs), nil
	}

	var jobs []models.PromptJobResponse
	if err := json.Unmarshal([]byte(clean), &jobs); err == nil {
		return assignCronExpressions(jobs), nil
	}

	re := regexp.MustCompile(`(?s)\[\s*{.*}\s*\]`)
	if m := re.FindString(clean); m != "" {
		var tempJobs []models.PromptJobResponse
		if err := json.Unmarshal([]byte(m), &tempJobs); err == nil {
			return assignCronExpressions(tempJobs), nil
		}
	}

	preview := clean[:min(200, len(clean))]
	return nil, fmt.Errorf("failed to parse model output into []models.PromptJobResponse (preview=%s)", strconv.Quote(preview))
}

func buildModelExecutors(cfg *config.Scheduler0Configurations, bedrockClient *awsbedrockruntime.Client, logger hclog.Logger) []ModelExecutor {
	providers := strings.TrimSpace(cfg.AIPromptProviders)
	if providers == "" {
		providers = constants.AIProviderOpenAI
	}

	executors := make([]ModelExecutor, 0, 2)
	for _, provider := range strings.Split(providers, ",") {
		name := strings.ToLower(strings.TrimSpace(provider))
		switch name {
		case constants.AIProviderOpenAI:
			executors = append(executors, NewOpenAIExecutor(cfg, logger))
		case constants.AIProviderBedrock, constants.AIProviderAnthropic:
			if bedrockClient == nil {
				logger.Warn("Skipping bedrock provider: client unavailable")
				continue
			}
			executors = append(executors, NewClaudeSonnetExecutor(bedrockClient, cfg, logger))
		case constants.AIProviderOpenRouter:
			key := strings.TrimSpace(cfg.OpenRouterAPIKey)
			if key == "" {
				logger.Warn("Skipping openrouter provider: SCHEDULER0_OPENROUTER_API_KEY not set")
				continue
			}
			executors = append(executors, NewOpenRouterExecutorWithKey(key, strings.TrimSpace(cfg.AIPreferredModel), logger))
		default:
			logger.Warn("Skipping unknown AI prompt provider", "provider", name)
		}
	}

	if len(executors) == 0 {
		executors = append(executors, NewOpenAIExecutor(cfg, logger))
	}

	return executors
}
