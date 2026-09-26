package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	classify_request_repo "scheduler0/pkg/repository/classify_request"
	prompt_request_repo "scheduler0/pkg/repository/prompt_request"
	"scheduler0/pkg/service/account"
	account_ai_settings_svc "scheduler0/pkg/service/account_ai_settings"
	"scheduler0/pkg/service/ai"
	ai_credits_svc "scheduler0/pkg/service/ai_credits"
	"scheduler0/pkg/service/aischedule"
	"scheduler0/pkg/utils"
	"strconv"
	"strings"
	"time"
)

type AIHTTPController interface {
	CreateJobsFromPrompt(w http.ResponseWriter, r *http.Request)
	ClassifyPrompt(w http.ResponseWriter, r *http.Request)
	AnalyzeSuggestions(w http.ResponseWriter, r *http.Request)
	ScheduleFromPrompt(w http.ResponseWriter, r *http.Request)
	GetPromptRequests(w http.ResponseWriter, r *http.Request)
	GetModels(w http.ResponseWriter, r *http.Request)
}

type aiController struct {
	promptService       *ai.PromptService
	scheduleService     aischedule.ScheduleService
	aiSettingsService   account_ai_settings_svc.AccountAISettingsService
	creditsService      ai_credits_svc.AICreditsService
	promptRequestRepo   prompt_request_repo.PromptRequestRepo
	classifyRequestRepo classify_request_repo.ClassifyRequestRepo
	accountService      account.AccountService
	logger              *log.Logger
}

func NewAIController(logger *log.Logger, promptService *ai.PromptService, scheduleService aischedule.ScheduleService, aiSettingsService account_ai_settings_svc.AccountAISettingsService, creditsService ai_credits_svc.AICreditsService, promptRequestRepo prompt_request_repo.PromptRequestRepo, classifyRequestRepo classify_request_repo.ClassifyRequestRepo, accountService account.AccountService) AIHTTPController {
	return &aiController{
		promptService:       promptService,
		scheduleService:     scheduleService,
		aiSettingsService:   aiSettingsService,
		creditsService:      creditsService,
		promptRequestRepo:   promptRequestRepo,
		classifyRequestRepo: classifyRequestRepo,
		accountService:      accountService,
		logger:              logger,
	}
}

func (c *aiController) enforcePlatformCredit(w http.ResponseWriter, requestID string, path string, accountId uint64, settings *models.AccountAISettings) bool {
	if c.creditsService == nil || !c.promptService.PrimaryExecutorIsPlatform(settings) {
		return true
	}
	hasBalance, err := c.creditsService.HasBalance(accountId)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - credit check error, accountId=%d, error=%s", path, accountId, err.Message))
		utils.SendJSON(w, err, false, err.Type, nil)
		return false
	}
	if !hasBalance {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - platform credits exhausted, accountId=%d", path, accountId))
		utils.SendJSON(w, "platform AI credits exhausted; top up your balance or switch to your own provider key", false, http.StatusPaymentRequired, nil)
		return false
	}
	return true
}

func (c *aiController) enforcePromptQuota(w http.ResponseWriter, requestID string, path string, accountId uint64) bool {
	usage, quotaErr := c.accountService.GetAIUsage(accountId)
	if quotaErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - prompt quota check error, accountId=%d, error=%s", path, accountId, quotaErr.Message))
		utils.SendJSON(w, quotaErr, false, quotaErr.Type, nil)
		return false
	}
	dim := usage.Prompt
	if dim.Used >= dim.Limit {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - prompt quota exhausted, accountId=%d, used=%d, limit=%d", path, accountId, dim.Used, dim.Limit))
		utils.SendJSON(w, "monthly AI prompt request limit reached", false, http.StatusTooManyRequests, rateLimitHeaders(dim.Limit, 0, usage.NextResetDate))
		return false
	}

	w.Header().Set("X-RateLimit-Limit", strconv.FormatUint(dim.Limit, 10))
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatUint(dim.Remaining, 10))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(usage.NextResetDate.Unix(), 10))
	return true
}

func (c *aiController) enforceClassifyQuota(w http.ResponseWriter, requestID string, path string, accountId uint64) bool {
	usage, quotaErr := c.accountService.GetAIUsage(accountId)
	if quotaErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - classify quota check error, accountId=%d, error=%s", path, accountId, quotaErr.Message))
		utils.SendJSON(w, quotaErr, false, quotaErr.Type, nil)
		return false
	}
	dim := usage.Classify
	if dim.Used >= dim.Limit {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - classify quota exhausted, accountId=%d, used=%d, limit=%d", path, accountId, dim.Used, dim.Limit))
		utils.SendJSON(w, "monthly AI classify request limit reached", false, http.StatusTooManyRequests, rateLimitHeaders(dim.Limit, 0, usage.NextResetDate))
		return false
	}

	w.Header().Set("X-RateLimit-Limit", strconv.FormatUint(dim.Limit, 10))
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatUint(dim.Remaining, 10))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(usage.NextResetDate.Unix(), 10))
	return true
}

func (c *aiController) recordClassifySuccess(requestID string, path string, accountId uint64, kind string, prompt string, decision string) {
	if c.classifyRequestRepo == nil {
		return
	}
	if err := c.classifyRequestRepo.Record(models.AccountClassifyRequest{
		AccountID: accountId,
		Kind:      kind,
		Prompt:    prompt,
		Decision:  decision,
		Status:    models.ClassifyRequestStatusSuccess,
	}); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - failed to record classify request, accountId=%d, error=%v", path, accountId, err))
	}
}

func rateLimitHeaders(limit uint64, remaining uint64, resetAt time.Time) map[string]string {
	return map[string]string{
		"X-RateLimit-Limit":     strconv.FormatUint(limit, 10),
		"X-RateLimit-Remaining": strconv.FormatUint(remaining, 10),
		"X-RateLimit-Reset":     strconv.FormatInt(resetAt.Unix(), 10),
	}
}

func (c *aiController) CreateJobsFromPrompt(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateJobsFromPrompt entry", r.URL.Path))

	body := utils.ExtractBody(w, r)
	if body == nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateJobsFromPrompt error: empty request body", r.URL.Path))
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateJobsFromPrompt error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	var req models.PropmptJobRequest
	if err := json.Unmarshal(body, &req); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateJobsFromPrompt error: failed to unmarshal request body, accountId=%d, error=%v", r.URL.Path, accountId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	if req.Prompt == "" {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateJobsFromPrompt error: prompt is required, accountId=%d", r.URL.Path, accountId))
		utils.SendJSON(w, "prompt is required", false, http.StatusBadRequest, nil)
		return
	}

	timezone := strings.TrimSpace(req.Timezone)
	if timezone != "" {
		if _, tzErr := time.LoadLocation(timezone); tzErr != nil {
			utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateJobsFromPrompt error: invalid timezone, accountId=%d, timezone=%q, error=%v", r.URL.Path, accountId, timezone, tzErr))
			utils.SendJSON(w, fmt.Sprintf("invalid timezone %q: %s", timezone, tzErr.Error()), false, http.StatusBadRequest, nil)
			return
		}
	}

	if !c.enforcePromptQuota(w, requestID, r.URL.Path, accountId) {
		return
	}

	settings, settingsErr := c.aiSettingsService.GetForExecution(accountId)
	if settingsErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateJobsFromPrompt warning: failed to load AI settings, accountId=%d, error=%s — using global config", r.URL.Path, accountId, settingsErr.Message))
		settings = nil
	}

	if !c.enforcePlatformCredit(w, requestID, r.URL.Path, accountId, settings) {
		return
	}

	locale := strings.TrimSpace(req.Locale)
	if locale == "" {
		locale = "en"
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateJobsFromPrompt calling prompt service, accountId=%d, timezone=%q, locale=%q", r.URL.Path, accountId, timezone, locale))

	jobs, classification, _, _, err := c.promptService.CreateJobFromPromptWithSettings(
		r.Context(),
		accountId,
		settings,
		req.Prompt,
		req.Purposes,
		req.Events,
		req.Recipients,
		req.Channels,
		timezone,
		locale,
	)
	if err != nil {
		var skipped *ai.IntentSkippedError
		if errors.As(err, &skipped) {
			utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateJobsFromPrompt skipped by intent guardrail, accountId=%d, decision=%s", r.URL.Path, accountId, skipped.Decision))
			utils.SendJSON(w, map[string]any{
				"message":        skipped.Error(),
				"classification": skipped.Classification,
			}, false, http.StatusUnprocessableEntity, nil)
			return
		}
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateJobsFromPrompt error: prompt service failed, accountId=%d, error=%v", r.URL.Path, accountId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateJobsFromPrompt success, status=200, accountId=%d, providers=%d", r.URL.Path, accountId, len(jobs)))
	utils.SendJSON(w, models.PromptResult{
		Providers:      jobs,
		Classification: classification,
	}, true, http.StatusOK, nil)
}

func (c *aiController) ScheduleFromPrompt(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ScheduleFromPrompt entry", r.URL.Path))

	body := utils.ExtractBody(w, r)
	if body == nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ScheduleFromPrompt error: empty request body", r.URL.Path))
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ScheduleFromPrompt error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	var req models.SchedulePromptRequest
	if err := json.Unmarshal(body, &req); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ScheduleFromPrompt error: failed to unmarshal request body, accountId=%d, error=%v", r.URL.Path, accountId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	if strings.TrimSpace(req.Prompt) == "" {
		utils.SendJSON(w, "prompt is required", false, http.StatusBadRequest, nil)
		return
	}
	if strings.TrimSpace(req.CreatedBy) == "" {
		utils.SendJSON(w, "createdBy is required", false, http.StatusBadRequest, nil)
		return
	}

	timezone := strings.TrimSpace(req.Timezone)
	if timezone != "" {
		if _, tzErr := time.LoadLocation(timezone); tzErr != nil {
			utils.SendJSON(w, fmt.Sprintf("invalid timezone %q: %s", timezone, tzErr.Error()), false, http.StatusBadRequest, nil)
			return
		}
	}

	if !c.enforcePromptQuota(w, requestID, r.URL.Path, accountId) {
		return
	}

	settings, settingsErr := c.aiSettingsService.GetForExecution(accountId)
	if settingsErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ScheduleFromPrompt warning: failed to load AI settings, accountId=%d, error=%s — using global config", r.URL.Path, accountId, settingsErr.Message))
		settings = nil
	}

	if !c.enforcePlatformCredit(w, requestID, r.URL.Path, accountId, settings) {
		return
	}

	result, err := c.scheduleService.ScheduleFromPrompt(r.Context(), requestID, accountId, settings, req)
	if err != nil {
		var skipped *ai.IntentSkippedError
		if errors.As(err, &skipped) {
			utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ScheduleFromPrompt skipped by intent guardrail, accountId=%d, decision=%s", r.URL.Path, accountId, skipped.Decision))
			utils.SendJSON(w, map[string]any{
				"message":        skipped.Error(),
				"classification": skipped.Classification,
			}, false, http.StatusUnprocessableEntity, nil)
			return
		}
		var scheduleErr *aischedule.ScheduleError
		if errors.As(err, &scheduleErr) {
			utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ScheduleFromPrompt error: accountId=%d, status=%d, error=%s", r.URL.Path, accountId, scheduleErr.Status, scheduleErr.Message))
			utils.SendJSON(w, scheduleErr.Message, false, scheduleErr.Status, nil)
			return
		}
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ScheduleFromPrompt error: accountId=%d, error=%v", r.URL.Path, accountId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ScheduleFromPrompt success, status=201, accountId=%d, jobs=%d, executor=%d", r.URL.Path, accountId, len(result.Jobs), result.Executor.ID))
	utils.SendJSON(w, result, true, http.StatusCreated, nil)
}

func (c *aiController) ClassifyPrompt(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ClassifyPrompt entry", r.URL.Path))

	body := utils.ExtractBody(w, r)
	if body == nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ClassifyPrompt error: empty request body", r.URL.Path))
		return
	}

	var req models.ClassifyPromptRequest
	if err := json.Unmarshal(body, &req); err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}
	if req.Prompt == "" {
		utils.SendJSON(w, "prompt is required", false, http.StatusBadRequest, nil)
		return
	}

	locale := strings.TrimSpace(req.Locale)
	if !ai.IsEnglishLocale(locale) {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ClassifyPrompt error: unsupported locale %q", r.URL.Path, locale))
		utils.SendJSON(w, fmt.Sprintf("unsupported locale %q: intent classification currently only supports English (en*)", locale), false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ClassifyPrompt error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	if !c.enforceClassifyQuota(w, requestID, r.URL.Path, accountId) {
		return
	}

	classification, err := c.promptService.ClassifyPrompt(r.Context(), req.Prompt)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ClassifyPrompt error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusServiceUnavailable, nil)
		return
	}

	c.recordClassifySuccess(requestID, r.URL.Path, accountId, models.ClassifyRequestKindClassify, req.Prompt, classification.Decision)
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ClassifyPrompt success, decision=%s", r.URL.Path, classification.Decision))
	utils.SendJSON(w, map[string]any{"classification": classification}, true, http.StatusOK, nil)
}

func (c *aiController) AnalyzeSuggestions(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AnalyzeSuggestions entry", r.URL.Path))

	body := utils.ExtractBody(w, r)
	if body == nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AnalyzeSuggestions error: empty request body", r.URL.Path))
		return
	}

	var req models.SuggestionAnalyzeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}
	if len(req.Messages) == 0 {
		utils.SendJSON(w, "at least one message is required", false, http.StatusBadRequest, nil)
		return
	}

	locale := ""
	if req.Options != nil {
		locale = strings.TrimSpace(req.Options.Locale)
	}
	if !ai.IsEnglishLocale(locale) {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AnalyzeSuggestions error: unsupported locale %q", r.URL.Path, locale))
		utils.SendJSON(w, map[string]any{
			"code":    "UNSUPPORTED_LOCALE",
			"message": fmt.Sprintf("unsupported locale %q: suggestions analysis currently only supports English (en*)", locale),
		}, false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	if !c.promptService.SuggestionAnalyzerConfigured() {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AnalyzeSuggestions error: analyzer not configured", r.URL.Path))
		utils.SendJSON(w, "suggestions analysis is not available", false, http.StatusServiceUnavailable, nil)
		return
	}

	if !c.enforceClassifyQuota(w, requestID, r.URL.Path, accountId) {
		return
	}

	result, err := c.promptService.AnalyzeSuggestions(r.Context(), req)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AnalyzeSuggestions error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusServiceUnavailable, nil)
		return
	}

	c.recordClassifySuccess(requestID, r.URL.Path, accountId, models.ClassifyRequestKindAnalyze, "", "")
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AnalyzeSuggestions success, suggestions=%d", r.URL.Path, len(result.Suggestions)))
	utils.SendJSON(w, result, true, http.StatusOK, nil)
}

func (c *aiController) GetPromptRequests(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetPromptRequests entry", r.URL.Path))

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	if c.promptRequestRepo == nil {
		utils.SendJSON(w, "prompt request log is not available", false, http.StatusServiceUnavailable, nil)
		return
	}

	q := r.URL.Query()
	filter := prompt_request_repo.PromptRequestFilter{
		AccountID:      accountId,
		Provider:       strings.TrimSpace(q.Get("provider")),
		Model:          strings.TrimSpace(q.Get("model")),
		Status:         strings.TrimSpace(q.Get("status")),
		Search:         strings.TrimSpace(q.Get("search")),
		OrderDirection: strings.ToUpper(strings.TrimSpace(q.Get("order"))),
		Limit:          clampListLimit(parseUintQuery(q.Get("limit"), defaultListLimit)),
		Offset:         parseUintQuery(q.Get("offset"), 0),
	}
	if start := parseTimeQuery(q.Get("start")); start != nil {
		filter.StartDate = start
	}
	if end := parseTimeQuery(q.Get("end")); end != nil {
		filter.EndDate = end
	}
	if filter.OrderDirection != constants.OrderDirectionAsc {
		filter.OrderDirection = constants.OrderDirectionDesc
	}

	requests, err := c.promptRequestRepo.GetPromptRequestsFiltered(filter)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetPromptRequests query error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}

	total, countErr := c.promptRequestRepo.CountPromptRequests(filter)
	if countErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetPromptRequests count error: %v", r.URL.Path, countErr))
		total = uint64(len(requests))
	}

	payload := map[string]any{
		"requests": requests,
		"total":    total,
		"limit":    filter.Limit,
		"offset":   filter.Offset,
	}
	utils.SendJSON(w, payload, true, http.StatusOK, nil)
}

const defaultListLimit uint64 = 25

func clampListLimit(limit uint64) uint64 {
	if limit == 0 {
		return defaultListLimit
	}
	if limit > constants.MaxListLimit {
		return constants.MaxListLimit
	}
	return limit
}

func parseUintQuery(v string, def uint64) uint64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return def
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func parseTimeQuery(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil
	}
	return &t
}

func (c *aiController) GetModels(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetModels entry", r.URL.Path))
	utils.SendJSON(w, ai.ApprovedModelsByProvider(), true, http.StatusOK, nil)
}
