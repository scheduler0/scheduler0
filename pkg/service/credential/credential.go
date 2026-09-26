package credential

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	"scheduler0/pkg/repository/credential"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/secrets"
	"scheduler0/pkg/utils"
	"time"

	"github.com/hashicorp/go-hclog"
)

// CredentialService service layer for credentials
type CredentialService interface {
	// CreateNewCredential creates a credential and returns its id along with the
	// plaintext api secret. The plaintext is returned to the caller exactly once (it is
	// never persisted in plaintext) so it can be surfaced to the client at creation time.
	CreateNewCredential(credentialModel models.Credential) (uint64, string, *utils.GenericError)
	FindOneCredentialByID(id uint64, accountId uint64) (*models.Credential, error)
	// FindOneCredentialByAPIKey resolves a non-deleted credential by its api key within
	// an account. It performs no secret validation; callers must already be authorized.
	FindOneCredentialByAPIKey(apiKey string, accountId uint64) (*models.Credential, *utils.GenericError)
	UpdateOneCredential(credentialModel models.Credential) (*models.Credential, error)
	DeleteOneCredential(id uint64, accountId uint64, deletedBy string) (*models.Credential, error)
	ArchiveOneCredential(id uint64, accountId uint64, archivedBy string) (*models.Credential, *utils.GenericError)
	ListCredentials(offset uint64, limit uint64, orderByColumn string, orderByDirection string, accountId uint64) (*models.PaginatedCredential, *utils.GenericError)
	ValidateServerAPIKey(apiKey string, apiSecret string, accountId uint64) (bool, *models.Credential, *utils.GenericError)
	SweepExpiredCredentials() (uint64, *utils.GenericError)
}

func NewCredentialService(Ctx context.Context, logger hclog.Logger, scheduler0Secret secrets.Scheduler0Secrets, repo credential.CredentialRepo, dispatcher *utils.Dispatcher) CredentialService {
	return &credentialService{
		CredentialRepo:   repo,
		Ctx:              Ctx,
		logger:           logger,
		dispatcher:       dispatcher,
		scheduler0Secret: scheduler0Secret,
	}
}

type credentialService struct {
	CredentialRepo   credential.CredentialRepo
	Ctx              context.Context
	logger           hclog.Logger
	dispatcher       *utils.Dispatcher
	scheduler0Secret secrets.Scheduler0Secrets
}

// validScopes is the canonical set of scopes accepted by the API.
var validScopes = map[string]struct{}{
	constants.CredentialScopeRead:    {},
	constants.CredentialScopeWrite:   {},
	constants.CredentialScopeExecute: {},
	constants.CredentialScopeAdmin:   {},
}

// scopesContainAdmin reports whether the requested scope set includes the admin scope.
func scopesContainAdmin(scopes []string) bool {
	for _, s := range scopes {
		if s == constants.CredentialScopeAdmin {
			return true
		}
	}
	return false
}

// validateScopes ensures Scopes is non-empty and only contains values from the canonical
// set. It does not mutate the input.
func validateScopes(scopes []string) *utils.GenericError {
	if len(scopes) == 0 {
		return utils.HTTPGenericError(http.StatusBadRequest, "at least one scope is required")
	}
	seen := map[string]struct{}{}
	for _, s := range scopes {
		if _, ok := validScopes[s]; !ok {
			return utils.HTTPGenericError(http.StatusBadRequest, "invalid scope: "+s)
		}
		if _, dup := seen[s]; dup {
			return utils.HTTPGenericError(http.StatusBadRequest, "duplicate scope: "+s)
		}
		seen[s] = struct{}{}
	}
	return nil
}

// CreateNewCredential creates a new credentials. The server is the source of truth for the
// api key, api secret, and expiry. Scopes must be supplied by the caller.
func (credentialService *credentialService) CreateNewCredential(credential models.Credential) (uint64, string, *utils.GenericError) {
	if validationErr := validateScopes(credential.Scopes); validationErr != nil {
		return 0, "", validationErr
	}

	credentials := credentialService.scheduler0Secret.GetSecrets()
	if credentials == nil || credentials.SecretKey == "" {
		return 0, "", utils.HTTPGenericError(http.StatusInternalServerError, "scheduler0 secret key is not set; cannot create credential")
	}

	// The api key is an opaque, stable lookup identifier. The api secret is stored
	// encrypted; the plaintext is returned to the caller once and never persisted.
	credential.ApiKey = utils.GenerateApiKey(credentials.SecretKey)
	plaintextSecret, ciphertextSecret := utils.GenerateApiSecret(credentials.SecretKey)
	credential.ApiSecret = ciphertextSecret

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())
	// The server is the source of truth for expiry. Callers may request a shorter
	// TTL (e.g. the CLI login flow) via ExpiresInSeconds; we clamp it to
	// [CredentialMinExpirySeconds, CredentialExpiryDays] so untrusted input can
	// never widen the window. A nil value uses the default 90-day expiry.
	expires := now.Add(resolveCredentialTTL(credential.ExpiresInSeconds))
	credential.ExpiresAt = &expires

	successData, errorData := credentialService.dispatcher.BlockQueue(func(successChannel chan any, errorChannel chan any) {
		newCredentialId, err := credentialService.CredentialRepo.CreateOne(credential)
		if err != nil {
			errorChannel <- err
			return
		}
		successChannel <- newCredentialId
	})

	newCredentialId, successOk := successData.(uint64)
	if successOk {
		return newCredentialId, plaintextSecret, nil
	}

	errM := errorData.(*utils.GenericError)
	return 0, "", errM
}

// resolveCredentialTTL turns an optional caller-requested TTL (in seconds) into a
// duration, defaulting to CredentialExpiryDays when unset and clamping any request
// to [CredentialMinExpirySeconds, CredentialExpiryDays].
func resolveCredentialTTL(expiresInSeconds *int64) time.Duration {
	maxDuration := time.Duration(constants.CredentialExpiryDays) * 24 * time.Hour
	if expiresInSeconds == nil {
		return maxDuration
	}
	requested := time.Duration(*expiresInSeconds) * time.Second
	minDuration := time.Duration(constants.CredentialMinExpirySeconds) * time.Second
	if requested < minDuration {
		return minDuration
	}
	if requested > maxDuration {
		return maxDuration
	}
	return requested
}

// FindOneCredentialByID searches for credential by uuid
func (credentialService *credentialService) FindOneCredentialByID(id uint64, accountId uint64) (*models.Credential, error) {
	credentialDto := models.Credential{ID: id, AccountId: accountId}
	if err := credentialService.CredentialRepo.GetOneID(&credentialDto); err != nil {
		return nil, err
	} else {
		return &credentialDto, nil
	}
}

// FindOneCredentialByAPIKey resolves a credential by api key scoped to an account.
// The stored (encrypted) secret is cleared from the returned model so callers cannot
// leak it; this lookup exists for trusted peers acting on behalf of a credential.
func (credentialService *credentialService) FindOneCredentialByAPIKey(apiKey string, accountId uint64) (*models.Credential, *utils.GenericError) {
	if apiKey == "" {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "api key is required")
	}
	credentialDto := models.Credential{ApiKey: apiKey, AccountId: accountId}
	if err := credentialService.CredentialRepo.GetByAPIKey(&credentialDto); err != nil {
		return nil, err
	}
	if credentialDto.ID == 0 {
		return nil, utils.HTTPGenericError(http.StatusNotFound, "credential not found")
	}
	credentialDto.ApiSecret = ""
	return &credentialDto, nil
}

// UpdateOneCredential updates the mutable fields of a credential (archived,
// modifiedBy). The api key and api secret are server-owned: they are never taken
// from the request and any attempt to change them is rejected. The stored values
// are always carried over so the repository update cannot blank them.
func (credentialService *credentialService) UpdateOneCredential(credential models.Credential) (*models.Credential, error) {
	credentialPlaceholder := models.Credential{
		ID:        credential.ID,
		AccountId: credential.AccountId,
	}
	err := credentialService.CredentialRepo.GetOneID(&credentialPlaceholder)
	if err != nil {
		return nil, err
	}

	if credential.ApiKey != "" && credential.ApiKey != credentialPlaceholder.ApiKey {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "api key cannot be updated")
	}

	if credential.ApiSecret != "" && credential.ApiSecret != credentialPlaceholder.ApiSecret {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "api secret cannot be updated")
	}

	credential.ApiKey = credentialPlaceholder.ApiKey
	credential.ApiSecret = credentialPlaceholder.ApiSecret
	credential.DateCreated = credentialPlaceholder.DateCreated

	if _, err := credentialService.CredentialRepo.UpdateOneByID(credential); err != nil {
		return nil, err
	}

	// Re-read so the caller gets the persisted record (scopes, expiry, dates)
	// rather than the partial request payload; the secret is never returned.
	updated := models.Credential{ID: credential.ID, AccountId: credential.AccountId}
	if err := credentialService.CredentialRepo.GetOneID(&updated); err != nil {
		return nil, err
	}
	updated.ApiSecret = ""
	return &updated, nil
}

// DeleteOneCredential deletes a single credential
func (credentialService *credentialService) DeleteOneCredential(id uint64, accountId uint64, deletedBy string) (*models.Credential, error) {
	if deletedBy == "" {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "deletedBy is required")
	}
	credentialDto := models.Credential{ID: id, AccountId: accountId, DeletedBy: &deletedBy}
	rowsAffected, err := credentialService.CredentialRepo.DeleteOneByID(credentialDto)
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, utils.HTTPGenericError(http.StatusNotFound, "credential not found")
	}
	return &credentialDto, nil
}

// ArchiveOneCredential archives a single credential
func (credentialService *credentialService) ArchiveOneCredential(id uint64, accountId uint64, archivedBy string) (*models.Credential, *utils.GenericError) {
	// Validate that archivedBy is provided
	if archivedBy == "" {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "archivedBy is required")
	}

	// Validate that account ID is not 1 (system user)
	if accountId == 1 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "system credentials cannot be archived")
	}

	credential := models.Credential{
		ID:         id,
		AccountId:  accountId,
		Archived:   true,
		ArchivedBy: &archivedBy,
	}

	// Archive the credential
	rowsAffected, err := credentialService.CredentialRepo.ArchiveOneByID(credential)
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, utils.HTTPGenericError(http.StatusNotFound, "credential not found")
	}

	credentialService.logger.Info("Successfully archived credential", "credentialID", id, "archivedBy", archivedBy)
	return &credential, nil
}

// ListCredentials returns paginated list of credentials
func (credentialService *credentialService) ListCredentials(offset uint64, limit uint64, orderByColumn string, orderByDirection string, accountId uint64) (*models.PaginatedCredential, *utils.GenericError) {
	total, err := credentialService.CredentialRepo.Count(accountId)
	if err != nil {
		return nil, err
	}

	if total < 1 {
		return &models.PaginatedCredential{
			Data:   []models.Credential{},
			Total:  0,
			Limit:  limit,
			Offset: offset,
		}, nil
	}

	if limit > constants.MaxListLimit {
		return nil, utils.HTTPGenericError(http.StatusTooManyRequests, fmt.Sprintf("too many credentials. limit should be less than %d", constants.MaxListLimit))
	}

	if limit < 1 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "limit should be greater than 0")
	}

	if credentialManagers, err := credentialService.CredentialRepo.List(offset, limit, orderByColumn, orderByDirection, accountId); err != nil {
		return nil, err
	} else {
		return &models.PaginatedCredential{
			Data:   credentialManagers,
			Total:  total,
			Offset: offset,
			Limit:  limit,
		}, nil
	}
}

// ValidateServerAPIKey authenticates incoming request from servers. The matched credential is
// returned alongside the validity flag so callers (the auth middleware) can apply expiry +
// scope checks without a second lookup.
//
// The credential is located by its api key (an opaque identifier). The stored api secret is
// held encrypted under the server SecretKey; validation decrypts it and compares the result
// to the presented secret in constant time. Storing the secret encrypted rather than as the
// verbatim token is what lets SecretKey rotation re-encrypt it without changing the secret
// the client holds.
func (credentialService *credentialService) ValidateServerAPIKey(apiKey string, apiSecret string, accountId uint64) (bool, *models.Credential, *utils.GenericError) {
	credentialManager := models.Credential{
		ApiKey:    apiKey,
		AccountId: accountId,
	}

	getApIError := credentialService.CredentialRepo.GetByAPIKey(&credentialManager)
	if getApIError != nil {
		return false, nil, getApIError
	}

	credentials := credentialService.scheduler0Secret.GetSecrets()
	if credentials == nil || credentials.SecretKey == "" {
		return false, &credentialManager, utils.HTTPGenericError(http.StatusInternalServerError, "scheduler0 secret key is not set; cannot validate credential")
	}

	storedSecret, ok := utils.DecryptSafe(credentialManager.ApiSecret, credentials.SecretKey)
	if !ok {
		// The stored secret can't be decrypted with the current SecretKey (corrupt row, or
		// a pending/failed key rotation). Treat as an auth failure rather than crashing.
		return false, &credentialManager, nil
	}

	if subtle.ConstantTimeCompare([]byte(storedSecret), []byte(apiSecret)) != 1 {
		return false, &credentialManager, nil
	}

	return true, &credentialManager, nil
}

// SweepExpiredCredentials archives every credential whose expires_at has passed. Intended to
// be invoked periodically from the raft leader so the write is replicated cluster-wide.
func (credentialService *credentialService) SweepExpiredCredentials() (uint64, *utils.GenericError) {
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())
	rowsAffected, err := credentialService.CredentialRepo.ArchiveExpired(now)
	if err != nil {
		return 0, err
	}
	if rowsAffected > 0 {
		credentialService.logger.Info("Archived expired credentials", "count", rowsAffected)
	}
	return rowsAffected, nil
}
