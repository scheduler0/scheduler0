package service

import (
	"context"
	"cron-server/server/models"
	"cron-server/server/repository"
)

type CredentialService struct {
	Pool repository.Pool
	Ctx context.Context
}

func (credentialService *CredentialService) CreateNewCredential(HTTPReferrerRestriction string) (string, error) {
	credential := models.Credential{ HTTPReferrerRestriction: HTTPReferrerRestriction }
	return credential.CreateOne(&credentialService.Pool, &credentialService.Ctx)
}