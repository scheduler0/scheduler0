package service

import (
	"context"
	"errors"
	"log"
	"net/http"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/secrets"
	"scheduler0/utils"
	"scheduler0/utils/workers"
)

// Credential service layer for credentials
type Credential interface {
	CreateNewCredential(credentialTransformer models.CredentialModel) (uint64, *utils.GenericError)
	FindOneCredentialByID(id uint64) (*models.CredentialModel, error)
	UpdateOneCredential(credentialTransformer models.CredentialModel) (*models.CredentialModel, error)
	DeleteOneCredential(id uint64) (*models.CredentialModel, error)
	ListCredentials(offset uint64, limit uint64, orderBy string) (*models.PaginatedCredential, *utils.GenericError)
	ValidateServerAPIKey(apiKey string, apiSecret string) (bool, *utils.GenericError)
}

func NewCredentialService(Ctx context.Context, logger *log.Logger, repo repository.Credential, dispatcher *workers.Dispatcher) Credential {
	return &credentialService{
		CredentialRepo: repo,
		Ctx:            Ctx,
		logger:         logger,
		dispatcher:     dispatcher,
	}
}

type credentialService struct {
	CredentialRepo repository.Credential
	Ctx            context.Context
	logger         *log.Logger
	dispatcher     *workers.Dispatcher
}

// CreateNewCredential creates a new credentials
func (credentialService *credentialService) CreateNewCredential(credentialTransformer models.CredentialModel) (uint64, *utils.GenericError) {
	credentials := secrets.GetSecrets(credentialService.logger)

	apiKey, apiSecret := utils.GenerateApiAndSecretKey(credentials.SecretKey)
	credentialTransformer.ApiKey = apiKey
	credentialTransformer.ApiSecret = apiSecret

	successData, errorData := credentialService.dispatcher.BlockQueue(func(successChannel chan any, errorChannel chan any) {
		newCredentialId, err := credentialService.CredentialRepo.CreateOne(credentialTransformer)
		if err != nil {
			errorChannel <- err
			return
		}
		successChannel <- newCredentialId
	})

	newCredentialId, successOk := successData.(uint64)
	if successOk {
		return newCredentialId, nil
	}
	errM := errorData.(utils.GenericError)
	return 0, &errM
}

// FindOneCredentialByID searches for credential by uuid
func (credentialService *credentialService) FindOneCredentialByID(id uint64) (*models.CredentialModel, error) {
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
func (credentialService *credentialService) DeleteOneCredential(id uint64) (*models.CredentialModel, error) {
	credentialDto := models.CredentialModel{ID: id}
	if _, err := credentialService.CredentialRepo.DeleteOneByID(credentialDto); err != nil {
		return nil, err
	} else {
		return &credentialDto, nil
	}
}

// ListCredentials returns paginated list of credentials
func (credentialService *credentialService) ListCredentials(offset uint64, limit uint64, orderBy string) (*models.PaginatedCredential, *utils.GenericError) {
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
			Total:  total,
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
