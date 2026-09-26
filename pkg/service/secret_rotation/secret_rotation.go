package secret_rotation

import (
	"net/http"
	account_ai_settings_repo "scheduler0/pkg/repository/account_ai_settings"
	credential_repo "scheduler0/pkg/repository/credential"
	executor_repo "scheduler0/pkg/repository/executor"
	"scheduler0/pkg/secrets"
	"scheduler0/pkg/utils"

	"github.com/hashicorp/go-hclog"
)

// RotateSecretResult reports how many rows were re-encrypted per subsystem.
type RotateSecretResult struct {
	CredentialsRotated uint64 `json:"credentialsRotated"`
	ExecutorsRotated   uint64 `json:"executorsRotated"`
	AISettingsRotated  uint64 `json:"aiSettingsRotated"`
}

// SecretRotationService re-encrypts every secret that is stored under the server's
// SecretKey (credential api secrets, executor cloud credentials, executor webhook secrets,
// and per-account AI provider keys) when an operator rotates that key.
//
// A credential's api_secret is stored encrypted and verified by decrypt-then-compare, so
// it is re-encrypted here just like the other secrets. The api_key is an opaque lookup
// identifier presented verbatim and is left unchanged, so rotation does not invalidate any
// client's credential.
type SecretRotationService interface {
	// RotateSecret re-encrypts all managed secrets from oldSecretKey to the server's
	// currently-loaded SecretKey. The operator is expected to have already updated the
	// SecretKey in their secrets source (and reloaded/restarted the server) so the loaded
	// key is the new one; oldSecretKey is the previous key needed to decrypt existing rows.
	RotateSecret(oldSecretKey string) (*RotateSecretResult, *utils.GenericError)
}

type secretRotationService struct {
	logger           hclog.Logger
	scheduler0Secret secrets.Scheduler0Secrets
	credentialRepo   credential_repo.CredentialRepo
	executorRepo     executor_repo.JobExecutorRepo
	aiSettingsRepo   account_ai_settings_repo.AccountAISettingsRepo
}

func NewSecretRotationService(
	logger hclog.Logger,
	scheduler0Secret secrets.Scheduler0Secrets,
	credentialRepo credential_repo.CredentialRepo,
	executorRepo executor_repo.JobExecutorRepo,
	aiSettingsRepo account_ai_settings_repo.AccountAISettingsRepo,
) SecretRotationService {
	return &secretRotationService{
		logger:           logger.Named("secret-rotation-service"),
		scheduler0Secret: scheduler0Secret,
		credentialRepo:   credentialRepo,
		executorRepo:     executorRepo,
		aiSettingsRepo:   aiSettingsRepo,
	}
}

func (s *secretRotationService) RotateSecret(oldSecretKey string) (*RotateSecretResult, *utils.GenericError) {
	if s.scheduler0Secret == nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, "scheduler0 secrets not configured")
	}
	// Skip the cache: the operator may have updated the SecretKey in the secrets
	// source after this process started, so we must read the freshly-loaded key
	// rather than a value cached at startup.
	creds := s.scheduler0Secret.GetSecretsSkipCache()
	if creds == nil || creds.SecretKey == "" {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, "scheduler0 secret key is not set; cannot rotate")
	}
	newSecretKey := creds.SecretKey

	if oldSecretKey == "" {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "oldSecretKey is required")
	}
	if !utils.IsValidAESHexKey(oldSecretKey) {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "oldSecretKey must be a hex-encoded AES key (16, 24 or 32 bytes)")
	}
	if !utils.IsValidAESHexKey(newSecretKey) {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, "loaded SecretKey is not a valid hex-encoded AES key")
	}

	// If the operator hasn't actually changed the key, there is nothing to migrate.
	// Re-encrypting would churn every row (new GCM nonce) for no security benefit.
	if oldSecretKey == newSecretKey {
		s.logger.Warn("rotate-secret called with oldSecretKey equal to the loaded SecretKey; nothing to rotate")
		return &RotateSecretResult{}, nil
	}

	credentialsRotated, err := s.credentialRepo.ReEncryptSecrets(oldSecretKey, newSecretKey)
	if err != nil {
		s.logger.Error("failed to re-encrypt credential secrets", "error", err.Message, "credentialsRotated", credentialsRotated)
		return nil, err
	}

	executorsRotated, err := s.executorRepo.ReEncryptSecrets(oldSecretKey, newSecretKey)
	if err != nil {
		s.logger.Error("failed to re-encrypt executor secrets", "error", err.Message, "credentialsRotated", credentialsRotated, "executorsRotated", executorsRotated)
		return nil, err
	}

	aiRotated, err := s.aiSettingsRepo.ReEncryptSecrets(oldSecretKey, newSecretKey)
	if err != nil {
		s.logger.Error("failed to re-encrypt AI settings secrets", "error", err.Message, "credentialsRotated", credentialsRotated, "executorsRotated", executorsRotated, "aiSettingsRotated", aiRotated)
		return nil, err
	}

	s.logger.Info("secret rotation complete", "credentialsRotated", credentialsRotated, "executorsRotated", executorsRotated, "aiSettingsRotated", aiRotated)
	return &RotateSecretResult{
		CredentialsRotated: credentialsRotated,
		ExecutorsRotated:   executorsRotated,
		AISettingsRotated:  aiRotated,
	}, nil
}
