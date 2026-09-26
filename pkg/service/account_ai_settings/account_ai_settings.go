package account_ai_settings

import (
	"fmt"
	"net/http"
	"strings"

	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	repo "scheduler0/pkg/repository/account_ai_settings"
	"scheduler0/pkg/service/ai"
	"scheduler0/pkg/utils"

	"github.com/hashicorp/go-hclog"
)

type AccountAISettingsService interface {
	Get(accountID uint64) (*models.AccountAISettings, *utils.GenericError)
	GetForExecution(accountID uint64) (*models.AccountAISettings, *utils.GenericError)
	Upsert(settings models.AccountAISettings) *utils.GenericError
}

type accountAISettingsService struct {
	repo   repo.AccountAISettingsRepo
	logger hclog.Logger
}

func NewAccountAISettingsService(
	logger hclog.Logger,
	r repo.AccountAISettingsRepo,
) AccountAISettingsService {
	return &accountAISettingsService{
		repo:   r,
		logger: logger.Named("account-ai-settings-service"),
	}
}

func (s *accountAISettingsService) Get(accountID uint64) (*models.AccountAISettings, *utils.GenericError) {
	if accountID == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account ID is required")
	}
	return s.repo.Get(accountID)
}

func (s *accountAISettingsService) GetForExecution(accountID uint64) (*models.AccountAISettings, *utils.GenericError) {
	if accountID == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account ID is required")
	}
	return s.repo.GetForExecution(accountID)
}

func hasStoredKey(provider string, existing *models.AccountAISettings) bool {
	if existing == nil {
		return false
	}
	switch strings.ToLower(provider) {
	case "openai":
		return existing.OpenAIAPIKey != ""
	case "anthropic":
		return existing.AnthropicAPIKey != ""
	case "bedrock":
		return existing.BedrockAccessKeyID != "" && existing.BedrockSecretKey != ""
	case "openrouter":
		return existing.OpenRouterAPIKey != ""
	}
	return false
}

func hasIncomingKey(provider string, settings models.AccountAISettings) bool {
	switch strings.ToLower(provider) {
	case "openai":
		return strings.TrimSpace(settings.OpenAIAPIKey) != ""
	case "anthropic":
		return strings.TrimSpace(settings.AnthropicAPIKey) != ""
	case "bedrock":
		return strings.TrimSpace(settings.BedrockAccessKeyID) != "" && strings.TrimSpace(settings.BedrockSecretKey) != ""
	case "openrouter":
		return strings.TrimSpace(settings.OpenRouterAPIKey) != ""
	}
	return false
}

func (s *accountAISettingsService) Upsert(settings models.AccountAISettings) *utils.GenericError {
	if settings.AccountID == 0 {
		return utils.HTTPGenericError(http.StatusBadRequest, "account ID is required")
	}

	if len(settings.ActiveModels) > 0 {
		existing, existErr := s.repo.Get(settings.AccountID)
		if existErr != nil {
			return existErr
		}

		for i, am := range settings.ActiveModels {
			p := strings.ToLower(strings.TrimSpace(am.Provider))
			if p == "" {
				return utils.HTTPGenericError(http.StatusUnprocessableEntity,
					fmt.Sprintf("active_models[%d]: provider is required", i))
			}
			m := strings.TrimSpace(am.Model)
			if m == "" {
				return utils.HTTPGenericError(http.StatusUnprocessableEntity,
					fmt.Sprintf("active_models[%d]: model is required", i))
			}
			if !ai.IsModelApproved(p, m) {
				return utils.HTTPGenericError(http.StatusUnprocessableEntity,
					fmt.Sprintf("active_models[%d]: model %q is not approved for provider %q; use GET /api/v1/ai/models for the approved list", i, m, p))
			}
			if p == constants.AIProviderPlatform {
				continue
			}
			if !hasIncomingKey(p, settings) && !hasStoredKey(p, existing) {
				return utils.HTTPGenericError(http.StatusUnprocessableEntity,
					fmt.Sprintf("active_models[%d]: no API key set for provider %q; supply the key in the same request or set it first", i, p))
			}
		}
	}

	return s.repo.Upsert(settings)
}
