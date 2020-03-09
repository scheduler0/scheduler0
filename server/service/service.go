package service

import (
	"context"
	"cron-server/server/dtos"
	"cron-server/server/migrations"
)

type CredentialService struct {
	Pool migrations.Pool
	Ctx  context.Context
}

func (credentialService *CredentialService) CreateNewCredential(HTTPReferrerRestriction string) (string, error) {
	credentialDto := dtos.CredentialDto{ HTTPReferrerRestriction: HTTPReferrerRestriction }
	credentialDomain := credentialDto.ToDomain()
	return credentialDomain.CreateOne(&credentialService.Pool, &credentialService.Ctx)
}