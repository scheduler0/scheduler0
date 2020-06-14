package service

import (
	"context"
	"cron-server/server/domains"
	"cron-server/server/dtos"
	"cron-server/server/db"
)

type CredentialService struct {
	Pool *db.Pool
	Ctx  context.Context
}

func (credentialService *CredentialService) CreateNewCredential(HTTPReferrerRestriction string) (string, error) {
	credentialDto := dtos.CredentialDto{HTTPReferrerRestriction: HTTPReferrerRestriction}
	credentialDomain := credentialDto.ToDomain()
	return credentialDomain.CreateOne(credentialService.Pool)
}

func (credentialService *CredentialService) FindOneCredentialByID(ID string) (*dtos.CredentialDto, error) {
	credentialDto := dtos.CredentialDto{ID: ID}
	credentialDomain := credentialDto.ToDomain()
	if _, err := credentialDomain.GetOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := dtos.CredentialDto{}
		outboundDto.FromDomain(credentialDomain)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) UpdateOneCredential(ID string) (*dtos.CredentialDto, error) {
	credentialDto := dtos.CredentialDto{ID: ID}
	credentialDomain := credentialDto.ToDomain()
	if _, err := credentialDomain.UpdateOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := dtos.CredentialDto{}
		outboundDto.FromDomain(credentialDomain)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) DeleteOneCredential(ID string) (*dtos.CredentialDto, error) {
	credentialDto := dtos.CredentialDto{ID: ID}
	credentialDomain := credentialDto.ToDomain()
	if _, err := credentialDomain.DeleteOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := dtos.CredentialDto{}
		outboundDto.FromDomain(credentialDomain)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) ListCredentials(offset int, limit int, orderBy string) ([]dtos.CredentialDto, error) {
	credentialDomain := domains.CredentialDomain{}
	if credentials, err := credentialDomain.GetAll(credentialService.Pool, offset, limit, orderBy); err != nil {
		return nil, err
	} else {
		outboundDto := make([]dtos.CredentialDto, len(credentials))

		for i, credential := range credentials {
			outboundDto[i].FromDomain(credential)
		}

		return outboundDto, nil
	}
}
