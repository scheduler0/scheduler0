package account_ai_settings

import (
	"encoding/json"
	"fmt"
	"net/http"
	"scheduler0-private/pkg/constants"
	"scheduler0-private/pkg/fsm"
	"scheduler0-private/pkg/models"
	"scheduler0-private/pkg/secrets"
	"scheduler0-private/pkg/utils"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
)

const (
	TableName           = "account_ai_settings"
	ColAccountID        = "account_id"
	ColActiveModels     = "active_models"
	ColOpenAIAPIKey     = "openai_api_key"
	ColAnthropicAPIKey  = "anthropic_api_key"
	ColBedrockAccessKey = "bedrock_access_key_id"
	ColBedrockSecretKey = "bedrock_secret_key"
	ColBedrockRegion    = "bedrock_region"
	ColOpenRouterAPIKey = "openrouter_api_key"
	ColDateCreated      = "date_created"
	ColDateModified     = "date_modified"
)

type AccountAISettingsRepo interface {
	Get(accountID uint64) (*models.AccountAISettings, *utils.GenericError)
	GetForExecution(accountID uint64) (*models.AccountAISettings, *utils.GenericError)
	Upsert(settings models.AccountAISettings) *utils.GenericError
	ReEncryptSecrets(oldKey, newKey string) (uint64, *utils.GenericError)
}

type accountAISettingsRepo struct {
	fsmStore              fsm.Scheduler0RaftStore
	scheduler0RaftActions fsm.Scheduler0RaftActions
	scheduler0Secret      secrets.Scheduler0Secrets
	logger                hclog.Logger
}

func NewAccountAISettingsRepo(
	logger hclog.Logger,
	scheduler0RaftActions fsm.Scheduler0RaftActions,
	fsmStore fsm.Scheduler0RaftStore,
	scheduler0Secret secrets.Scheduler0Secrets,
) AccountAISettingsRepo {
	return &accountAISettingsRepo{
		fsmStore:              fsmStore,
		scheduler0RaftActions: scheduler0RaftActions,
		scheduler0Secret:      scheduler0Secret,
		logger:                logger.Named("account-ai-settings-repo"),
	}
}

func (r *accountAISettingsRepo) encryptAISecret(plaintext string) (string, *utils.GenericError) {
	if plaintext == "" {
		return "", nil
	}
	if r.scheduler0Secret == nil {
		return "", utils.HTTPGenericError(http.StatusInternalServerError, "scheduler0 secrets not configured; cannot encrypt AI credentials")
	}
	creds := r.scheduler0Secret.GetSecrets()
	if creds == nil || creds.SecretKey == "" {
		return "", utils.HTTPGenericError(http.StatusInternalServerError, "scheduler0 secret key is not set; cannot encrypt AI credentials")
	}
	return utils.Encrypt(plaintext, creds.SecretKey), nil
}

func (r *accountAISettingsRepo) decryptAISecret(ciphertext string) (plaintext string) {
	if ciphertext == "" {
		return ""
	}
	if r.scheduler0Secret == nil {
		r.logger.Error("cannot decrypt AI credential: scheduler0 secrets not configured")
		return ""
	}
	creds := r.scheduler0Secret.GetSecrets()
	if creds == nil || creds.SecretKey == "" {
		r.logger.Error("cannot decrypt AI credential: scheduler0 secret key is not set")
		return ""
	}

	defer func() {
		if rec := recover(); rec != nil {
			r.logger.Error("failed to decrypt AI credential; treating it as unset", "error", rec)
			plaintext = ""
		}
	}()
	return utils.Decrypt(ciphertext, creds.SecretKey)
}

func (r *accountAISettingsRepo) Get(accountID uint64) (*models.AccountAISettings, *utils.GenericError) {
	r.fsmStore.GetDataStore().ConnectionLock()
	defer r.fsmStore.GetDataStore().ConnectionUnlock()

	query := sq.Select(
		ColAccountID,
		ColActiveModels,
		ColOpenAIAPIKey, ColAnthropicAPIKey,
		ColBedrockAccessKey, ColBedrockSecretKey, ColBedrockRegion,
		ColOpenRouterAPIKey,
		ColDateCreated, ColDateModified,
	).From(TableName).
		Where(fmt.Sprintf("%s = ?", ColAccountID), accountID).
		RunWith(r.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := query.Query()
	if err != nil {
		r.logger.Error("Get: query failed", "error", err, "accountID", accountID)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	var s models.AccountAISettings
	var activeModelsJSON string
	if err := rows.Scan(
		&s.AccountID,
		&activeModelsJSON,
		&s.OpenAIAPIKey, &s.AnthropicAPIKey,
		&s.BedrockAccessKeyID, &s.BedrockSecretKey, &s.BedrockRegion,
		&s.OpenRouterAPIKey,
		&s.DateCreated, &s.DateModified,
	); err != nil {
		r.logger.Error("Get: scan failed", "error", err, "accountID", accountID)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	if strings.TrimSpace(activeModelsJSON) != "" {
		if jsonErr := json.Unmarshal([]byte(activeModelsJSON), &s.ActiveModels); jsonErr != nil {
			r.logger.Warn("Get: failed to unmarshal active_models", "error", jsonErr, "accountID", accountID)
		}
	}
	return &s, nil
}

func (r *accountAISettingsRepo) GetForExecution(accountID uint64) (*models.AccountAISettings, *utils.GenericError) {
	s, err := r.Get(accountID)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}
	s.OpenAIAPIKey = r.decryptAISecret(s.OpenAIAPIKey)
	s.AnthropicAPIKey = r.decryptAISecret(s.AnthropicAPIKey)
	s.BedrockAccessKeyID = r.decryptAISecret(s.BedrockAccessKeyID)
	s.BedrockSecretKey = r.decryptAISecret(s.BedrockSecretKey)
	s.OpenRouterAPIKey = r.decryptAISecret(s.OpenRouterAPIKey)
	return s, nil
}

func (r *accountAISettingsRepo) ReEncryptSecrets(oldKey, newKey string) (uint64, *utils.GenericError) {
	type row struct {
		accountID     uint64
		openAI        string
		anthropic     string
		bedrockAccess string
		bedrockSecret string
		openRouter    string
	}

	r.fsmStore.GetDataStore().ConnectionLock()
	query := sq.Select(
		ColAccountID,
		ColOpenAIAPIKey, ColAnthropicAPIKey,
		ColBedrockAccessKey, ColBedrockSecretKey,
		ColOpenRouterAPIKey,
	).From(TableName).
		RunWith(r.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := query.Query()
	if err != nil {
		r.fsmStore.GetDataStore().ConnectionUnlock()
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	pending := []row{}
	for rows.Next() {
		var rw row
		if scanErr := rows.Scan(&rw.accountID, &rw.openAI, &rw.anthropic, &rw.bedrockAccess, &rw.bedrockSecret, &rw.openRouter); scanErr != nil {
			rows.Close()
			r.fsmStore.GetDataStore().ConnectionUnlock()
			return 0, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		pending = append(pending, rw)
	}
	rowsErr := rows.Err()
	rows.Close()
	r.fsmStore.GetDataStore().ConnectionUnlock()
	if rowsErr != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, rowsErr.Error())
	}

	var rotated uint64
	for _, rw := range pending {
		updateBuilder := sq.Update(TableName)
		changed := false

		if newCipher, ok := utils.ReEncrypt(rw.openAI, oldKey, newKey); ok {
			updateBuilder = updateBuilder.Set(ColOpenAIAPIKey, newCipher)
			changed = true
		}
		if newCipher, ok := utils.ReEncrypt(rw.anthropic, oldKey, newKey); ok {
			updateBuilder = updateBuilder.Set(ColAnthropicAPIKey, newCipher)
			changed = true
		}
		if newCipher, ok := utils.ReEncrypt(rw.bedrockAccess, oldKey, newKey); ok {
			updateBuilder = updateBuilder.Set(ColBedrockAccessKey, newCipher)
			changed = true
		}
		if newCipher, ok := utils.ReEncrypt(rw.bedrockSecret, oldKey, newKey); ok {
			updateBuilder = updateBuilder.Set(ColBedrockSecretKey, newCipher)
			changed = true
		}
		if newCipher, ok := utils.ReEncrypt(rw.openRouter, oldKey, newKey); ok {
			updateBuilder = updateBuilder.Set(ColOpenRouterAPIKey, newCipher)
			changed = true
		}
		if !changed {
			continue
		}

		q, params, buildErr := updateBuilder.
			Where(fmt.Sprintf("%s = ?", ColAccountID), rw.accountID).
			ToSql()
		if buildErr != nil {
			return rotated, utils.HTTPGenericError(http.StatusInternalServerError, buildErr.Error())
		}

		_, applyErr := r.scheduler0RaftActions.WriteCommandToRaftLog(
			r.fsmStore.GetRaft(),
			constants.CommandTypeDbExecute,
			q,
			params,
			[]uint64{},
			0,
		)
		if applyErr != nil {
			return rotated, applyErr
		}
		rotated++
	}

	return rotated, nil
}

func (r *accountAISettingsRepo) Upsert(s models.AccountAISettings) *utils.GenericError {
	now := time.Now().UTC()

	existing, getErr := r.Get(s.AccountID)
	if getErr != nil {
		return getErr
	}

	openaiCipher, encErr := r.encryptAISecret(s.OpenAIAPIKey)
	if encErr != nil {
		return encErr
	}
	if openaiCipher == "" && existing != nil {
		openaiCipher = existing.OpenAIAPIKey
	}

	anthropicCipher, encErr := r.encryptAISecret(s.AnthropicAPIKey)
	if encErr != nil {
		return encErr
	}
	if anthropicCipher == "" && existing != nil {
		anthropicCipher = existing.AnthropicAPIKey
	}

	bedrockAccessCipher, encErr := r.encryptAISecret(s.BedrockAccessKeyID)
	if encErr != nil {
		return encErr
	}
	if bedrockAccessCipher == "" && existing != nil {
		bedrockAccessCipher = existing.BedrockAccessKeyID
	}

	bedrockSecretCipher, encErr := r.encryptAISecret(s.BedrockSecretKey)
	if encErr != nil {
		return encErr
	}
	if bedrockSecretCipher == "" && existing != nil {
		bedrockSecretCipher = existing.BedrockSecretKey
	}

	openRouterCipher, encErr := r.encryptAISecret(s.OpenRouterAPIKey)
	if encErr != nil {
		return encErr
	}
	if openRouterCipher == "" && existing != nil {
		openRouterCipher = existing.OpenRouterAPIKey
	}

	activeModelsJSON := ""
	if len(s.ActiveModels) > 0 {
		b, marshalErr := json.Marshal(s.ActiveModels)
		if marshalErr != nil {
			r.logger.Error("Upsert: failed to marshal active_models", "error", marshalErr, "accountID", s.AccountID)
			return utils.HTTPGenericError(http.StatusInternalServerError, "failed to serialize active_models")
		}
		activeModelsJSON = string(b)
	}

	var query string
	var params []interface{}
	var buildErr error

	if existing == nil {
		query, params, buildErr = sq.Insert(TableName).
			Columns(
				ColAccountID,
				ColActiveModels,
				ColOpenAIAPIKey, ColAnthropicAPIKey,
				ColBedrockAccessKey, ColBedrockSecretKey, ColBedrockRegion,
				ColOpenRouterAPIKey,
				ColDateCreated, ColDateModified,
			).
			Values(
				s.AccountID,
				activeModelsJSON,
				openaiCipher, anthropicCipher,
				bedrockAccessCipher, bedrockSecretCipher, s.BedrockRegion,
				openRouterCipher,
				now, now,
			).
			ToSql()
	} else {
		query, params, buildErr = sq.Update(TableName).
			Set(ColActiveModels, activeModelsJSON).
			Set(ColOpenAIAPIKey, openaiCipher).
			Set(ColAnthropicAPIKey, anthropicCipher).
			Set(ColBedrockAccessKey, bedrockAccessCipher).
			Set(ColBedrockSecretKey, bedrockSecretCipher).
			Set(ColBedrockRegion, s.BedrockRegion).
			Set(ColOpenRouterAPIKey, openRouterCipher).
			Set(ColDateModified, now).
			Where(fmt.Sprintf("%s = ?", ColAccountID), s.AccountID).
			ToSql()
	}

	if buildErr != nil {
		r.logger.Error("Upsert: failed to build query", "error", buildErr, "accountID", s.AccountID)
		return utils.HTTPGenericError(http.StatusInternalServerError, buildErr.Error())
	}

	_, applyErr := r.scheduler0RaftActions.WriteCommandToRaftLog(
		r.fsmStore.GetRaft(),
		constants.CommandTypeDbExecute,
		query,
		params,
		[]uint64{},
		0,
	)
	if applyErr != nil {
		r.logger.Error("Upsert: raft write failed", "error", applyErr, "accountID", s.AccountID)
		return applyErr
	}

	return nil
}
