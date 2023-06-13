package service

import (
	"context"
	"errors"
	"github.com/hashicorp/go-hclog"
	"net/http"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/secrets"
	"scheduler0/utils"
)

// CredentialService service layer for credentials
//go:generate mockery --name CredentialService --output ../mocks
type CredentialService interface {
	CreateNewCredential(credentialTransformer models.Credential) (uint64, *utils.GenericError)
	FindOneCredentialByID(id uint64) (*models.Credential, error)
	UpdateOneCredential(credentialTransformer models.Credential) (*models.Credential, error)
	DeleteOneCredential(id uint64) (*models.Credential, error)
	ListCredentials(offset uint64, limit uint64, orderBy string) (*models.PaginatedCredential, *utils.GenericError)
	ValidateServerAPIKey(apiKey string, apiSecret string) (bool, *utils.GenericError)
}

func NewCredentialService(Ctx context.Context, logger hclog.Logger, scheduler0Secret secrets.Scheduler0Secrets, repo repository.CredentialRepo, dispatcher *utils.Dispatcher) CredentialService {
	return &credentialService{
		CredentialRepo:   repo,
		Ctx:              Ctx,
		logger:           logger,
		dispatcher:       dispatcher,
		scheduler0Secret: scheduler0Secret,
	}
}

type credentialService struct {
	CredentialRepo   repository.CredentialRepo
	Ctx              context.Context
	logger           hclog.Logger
	dispatcher       *utils.Dispatcher
	scheduler0Secret secrets.Scheduler0Secrets
}

// CreateNewCredential creates a new credentials
func (credentialService *credentialService) CreateNewCredential(credentialTransformer models.Credential) (uint64, *utils.GenericError) {
	credentials := credentialService.scheduler0Secret.GetSecrets()

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
func (credentialService *credentialService) FindOneCredentialByID(id uint64) (*models.Credential, error) {
	credentialDto := models.Credential{ID: id}
	if err := credentialService.CredentialRepo.GetOneID(&credentialDto); err != nil {
		return nil, err
	} else {
		return &credentialDto, nil
	}
}

// UpdateOneCredential updates a single credential
func (credentialService *credentialService) UpdateOneCredential(credential models.Credential) (*models.Credential, error) {
	credentialPlaceholder := models.Credential{
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
func (credentialService *credentialService) DeleteOneCredential(id uint64) (*models.Credential, error) {
	credentialDto := models.Credential{ID: id}
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
	credentialManager := models.Credential{
		ApiKey: apiKey,
	}

	getApIError := credentialService.CredentialRepo.GetByAPIKey(&credentialManager)
	if getApIError != nil {
		return false, getApIError
	}

	return apiSecret == credentialManager.ApiSecret, nil
}
