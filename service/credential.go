package service

import (
	"errors"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/utils"
)

// Credential service layer for credentials
type Credential interface {
	CreateNewCredential(credentialTransformer models.CredentialModel) (int64, *utils.GenericError)
	FindOneCredentialByID(id int64) (*models.CredentialModel, error)
	UpdateOneCredential(credentialTransformer models.CredentialModel) (*models.CredentialModel, error)
	DeleteOneCredential(id int64) (*models.CredentialModel, error)
	ListCredentials(offset int64, limit int64, orderBy string) (*models.PaginatedCredential, *utils.GenericError)
	ValidateServerAPIKey(apiKey string, apiSecret string) (bool, *utils.GenericError)
	ValidateIOSAPIKey(apiKey string, IOSBundle string) (bool, *utils.GenericError)
	ValidateAndroidAPIKey(apiKey string, androidPackageName string) (bool, *utils.GenericError)
	ValidateWebAPIKeyHTTPReferrerRestriction(apiKey string, callerUrl string) (bool, *utils.GenericError)
}

func NewCredentialService(logger *log.Logger, repo repository.Credential, Ctx context.Context) Credential {
	return &credentialService{
		CredentialRepo: repo,
		Ctx:            Ctx,
		logger:         logger,
	}
}

type credentialService struct {
	CredentialRepo repository.Credential
	Ctx            context.Context
	logger         *log.Logger
}

// CreateNewCredential creates a new credentials
func (credentialService *credentialService) CreateNewCredential(credentialTransformer models.CredentialModel) (int64, *utils.GenericError) {
	if len(credentialTransformer.Platform) < 1 {
		return -1, utils.HTTPGenericError(http.StatusBadRequest, "credential should have a platform")
	}

	if credentialTransformer.Platform != repository.AndroidPlatform &&
		credentialTransformer.Platform != repository.WebPlatform &&
		credentialTransformer.Platform != repository.IOSPlatform &&
		credentialTransformer.Platform != repository.ServerPlatform {
		return -1, utils.HTTPGenericError(http.StatusBadRequest, "credential platform should be one of server, web, android, or ios")
	}

	switch credentialTransformer.Platform {
	case repository.AndroidPlatform:
		if len(credentialTransformer.AndroidPackageNameRestriction) < 1 {
			return -1, utils.HTTPGenericError(http.StatusBadRequest, "android credentials should have a package name restriction")
		}
	case repository.IOSPlatform:
		if len(credentialTransformer.IOSBundleIDRestriction) < 1 {
			return -1, utils.HTTPGenericError(http.StatusBadRequest, "ios credentials should have a bundle restriction")
		}
	case repository.WebPlatform:
		if len(credentialTransformer.HTTPReferrerRestriction) < 1 && len(credentialTransformer.IPRestriction) < 1 {
			return -1, utils.HTTPGenericError(http.StatusBadRequest, "web credentials should either an ip restriction or a url restriction")
		}
	}

	configs := utils.GetScheduler0Configurations(credentialService.logger)

	if credentialTransformer.Platform == repository.ServerPlatform {
		apiKey, apiSecret := utils.GenerateApiAndSecretKey(configs.SecretKey)
		credentialTransformer.ApiKey = apiKey
		credentialTransformer.ApiSecret = apiSecret
	}

	newCredentialId, err := credentialService.CredentialRepo.CreateOne(credentialTransformer)
	if err != nil {
		return -1, err
	}

	return newCredentialId, nil
}

// FindOneCredentialByID searches for credential by uuid
func (credentialService *credentialService) FindOneCredentialByID(id int64) (*models.CredentialModel, error) {
	credentialDto := models.CredentialModel{ID: id}
	if err := credentialService.CredentialRepo.GetOneID(&credentialDto); err != nil {
		return nil, err
	} else {
		return &credentialDto, nil
	}
}

// UpdateOneCredential updates a single credential
func (credentialService *credentialService) UpdateOneCredential(credential models.CredentialModel) (*models.CredentialModel, error) {
	credentialPlaceholder := models.CredentialModel{
		ID: credential.ID,
	}
	err := credentialService.CredentialRepo.GetOneID(&credentialPlaceholder)
	if err != nil {
		return nil, err
	}
	if credentialPlaceholder.ApiKey != credential.ApiKey && len(credential.ApiKey) > 1 {
		return nil, errors.New("cannot update api key")
	}

	if credentialPlaceholder.ApiSecret != credential.ApiSecret && len(credential.ApiSecret) > 1 {
		return nil, errors.New("cannot update api secret")
	}

	credential.ApiKey = credentialPlaceholder.ApiKey
	credential.ApiSecret = credentialPlaceholder.ApiSecret
	credential.DateCreated = credentialPlaceholder.DateCreated

	if _, err := credentialService.CredentialRepo.UpdateOneByID(credential); err != nil {
		return nil, err
	} else {
		return &credential, nil
	}
}

// DeleteOneCredential deletes a single credential
func (credentialService *credentialService) DeleteOneCredential(id int64) (*models.CredentialModel, error) {
	credentialDto := models.CredentialModel{ID: id}
	if _, err := credentialService.CredentialRepo.DeleteOneByID(credentialDto); err != nil {
		return nil, err
	} else {
		return &credentialDto, nil
	}
}

// ListCredentials returns paginated list of credentials
func (credentialService *credentialService) ListCredentials(offset int64, limit int64, orderBy string) (*models.PaginatedCredential, *utils.GenericError) {
	total, err := credentialService.CredentialRepo.Count()
	if err != nil {
		return nil, err
	}

	if total < 1 {
		return nil, utils.HTTPGenericError(http.StatusNotFound, "there no credentials")
	}

	if credentialManagers, err := credentialService.CredentialRepo.List(offset, limit, orderBy); err != nil {
		return nil, err
	} else {
		return &models.PaginatedCredential{
			Data:   credentialManagers,
			Total:  int64(total),
			Offset: offset,
			Limit:  limit,
		}, nil
	}
}

// ValidateServerAPIKey authenticates incoming request from servers
func (credentialService *credentialService) ValidateServerAPIKey(apiKey string, apiSecret string) (bool, *utils.GenericError) {
	credentialManager := models.CredentialModel{
		ApiKey: apiKey,
	}

	getApIError := credentialService.CredentialRepo.GetByAPIKey(&credentialManager)
	if getApIError != nil {
		return false, getApIError
	}

	return apiSecret == credentialManager.ApiSecret, nil
}

// ValidateIOSAPIKey authenticates incoming request from iOS app s
func (credentialService *credentialService) ValidateIOSAPIKey(apiKey string, IOSBundle string) (bool, *utils.GenericError) {
	credentialManager := models.CredentialModel{
		ApiKey: apiKey,
	}

	getApIError := credentialService.CredentialRepo.GetByAPIKey(&credentialManager)
	if getApIError != nil {
		return false, getApIError
	}

	return credentialManager.IOSBundleIDRestriction == IOSBundle, nil
}

// ValidateAndroidAPIKey authenticates incoming request from android app
func (credentialService *credentialService) ValidateAndroidAPIKey(apiKey string, androidPackageName string) (bool, *utils.GenericError) {
	credentialManager := models.CredentialModel{
		ApiKey: apiKey,
	}

	getApIError := credentialService.CredentialRepo.GetByAPIKey(&credentialManager)
	if getApIError != nil {
		return false, getApIError
	}

	return credentialManager.AndroidPackageNameRestriction == androidPackageName, nil
}

// ValidateWebAPIKeyHTTPReferrerRestriction authenticates incoming request from web clients
func (credentialService *credentialService) ValidateWebAPIKeyHTTPReferrerRestriction(apiKey string, callerUrl string) (bool, *utils.GenericError) {
	credentialManager := models.CredentialModel{
		ApiKey: apiKey,
	}

	getApIError := credentialService.CredentialRepo.GetByAPIKey(&credentialManager)
	if getApIError != nil {
		return false, getApIError
	}

	if callerUrl == credentialManager.HTTPReferrerRestriction {
		return true, nil
	}

	return false, utils.HTTPGenericError(http.StatusUnauthorized, "the user is not authorized")
}
