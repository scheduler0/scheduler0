package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"scheduler0/pkg/models"
	svc "scheduler0/pkg/service/account_ai_settings"
	"scheduler0/pkg/utils"
)

type AccountAISettingsController interface {
	GetAISettings(w http.ResponseWriter, r *http.Request)
	UpsertAISettings(w http.ResponseWriter, r *http.Request)
}

type accountAISettingsController struct {
	service svc.AccountAISettingsService
	logger  *log.Logger
}

func NewAccountAISettingsController(logger *log.Logger, service svc.AccountAISettingsService) AccountAISettingsController {
	return &accountAISettingsController{
		service: service,
		logger:  logger,
	}
}

func (c *accountAISettingsController) GetAISettings(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetAISettings entry", r.URL.Path))
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - headers: X-Account-ID=%q X-Peer=%q", r.URL.Path, r.Header.Get("X-Account-ID"), r.Header.Get("X-Peer")))

	accountID, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetAISettings: account ID not found in context (X-Account-ID header=%q)", r.URL.Path, r.Header.Get("X-Account-ID")))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetAISettings: resolved accountID=%d", r.URL.Path, accountID))

	settings, err := c.service.Get(accountID)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetAISettings error: %s", r.URL.Path, err.Message))
		utils.SendJSON(w, err, false, err.Type, nil)
		return
	}

	if settings == nil {
		utils.SendJSON(w, models.AccountAISettings{AccountID: accountID}, true, http.StatusOK, nil)
		return
	}

	redacted := redactAISecrets(*settings)
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetAISettings success, accountID=%d", r.URL.Path, accountID))
	utils.SendJSON(w, redacted, true, http.StatusOK, nil)
}

func redactAISecrets(s models.AccountAISettings) models.AccountAISettings {
	mask := func(v string) string {
		if v != "" {
			return "•"
		}
		return ""
	}
	s.OpenAIAPIKey = mask(s.OpenAIAPIKey)
	s.AnthropicAPIKey = mask(s.AnthropicAPIKey)
	s.BedrockAccessKeyID = mask(s.BedrockAccessKeyID)
	s.BedrockSecretKey = mask(s.BedrockSecretKey)
	s.OpenRouterAPIKey = mask(s.OpenRouterAPIKey)
	return s
}

func (c *accountAISettingsController) UpsertAISettings(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpsertAISettings entry", r.URL.Path))
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - headers: X-Account-ID=%q X-Peer=%q", r.URL.Path, r.Header.Get("X-Account-ID"), r.Header.Get("X-Peer")))

	accountID, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpsertAISettings: account ID not found in context (X-Account-ID header=%q)", r.URL.Path, r.Header.Get("X-Account-ID")))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpsertAISettings: resolved accountID=%d", r.URL.Path, accountID))

	var body struct {
		ActiveModels       []models.ActiveModel  `json:"active_models"`
		OpenAIAPIKey       string                `json:"openai_api_key"`
		AnthropicAPIKey    string                `json:"anthropic_api_key"`
		BedrockAccessKeyID string                `json:"bedrock_access_key_id"`
		BedrockSecretKey   string                `json:"bedrock_secret_key"`
		BedrockRegion      string                `json:"bedrock_region"`
		OpenRouterAPIKey   string                `json:"openrouter_api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpsertAISettings decode error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	req := models.AccountAISettings{
		AccountID:          accountID,
		ActiveModels:       body.ActiveModels,
		OpenAIAPIKey:       body.OpenAIAPIKey,
		AnthropicAPIKey:    body.AnthropicAPIKey,
		BedrockAccessKeyID: body.BedrockAccessKeyID,
		BedrockSecretKey:   body.BedrockSecretKey,
		BedrockRegion:      body.BedrockRegion,
		OpenRouterAPIKey:   body.OpenRouterAPIKey,
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpsertAISettings: decoded body active_models=%d", r.URL.Path, len(req.ActiveModels)))

	if upsertErr := c.service.Upsert(req); upsertErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpsertAISettings error: %s (type=%d)", r.URL.Path, upsertErr.Message, upsertErr.Type))
		utils.SendJSON(w, upsertErr, false, upsertErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpsertAISettings success, accountID=%d active_models=%d", r.URL.Path, accountID, len(req.ActiveModels)))
	utils.SendJSON(w, map[string]string{"message": "AI settings saved"}, true, http.StatusOK, nil)
}
