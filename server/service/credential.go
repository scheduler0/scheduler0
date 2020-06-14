package service

import (
	"context"
	"cron-server/server/managers"
	"cron-server/server/data"
	"cron-server/server/db"
)

type CredentialService struct {
	Pool *db.Pool
	Ctx  context.Context
}

func (credentialService *CredentialService) CreateNewCredential(HTTPReferrerRestriction string) (string, error) {
	credentialDto := data.Credential{HTTPReferrerRestriction: HTTPReferrerRestriction}
	credentialDomain := credentialDto.ToManager()
	return credentialDomain.CreateOne(credentialService.Pool)
}

func (credentialService *CredentialService) FindOneCredentialByID(ID string) (*data.Credential, error) {
	credentialDto := data.Credential{ID: ID}
	credentialDomain := credentialDto.ToManager()
	if _, err := credentialDomain.GetOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := data.Credential{}
		outboundDto.FromManager(credentialDomain)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) UpdateOneCredential(ID string) (*data.Credential, error) {
	credentialDto := data.Credential{ID: ID}
	credentialDomain := credentialDto.ToManager()
	if _, err := credentialDomain.UpdateOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := data.Credential{}
		outboundDto.FromManager(credentialDomain)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) DeleteOneCredential(ID string) (*data.Credential, error) {
	credentialDto := data.Credential{ID: ID}
	credentialDomain := credentialDto.ToManager()
	if _, err := credentialDomain.DeleteOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := data.Credential{}
		outboundDto.FromManager(credentialDomain)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) ListCredentials(offset int, limit int, orderBy string) ([]data.Credential, error) {
	credentialDomain := managers.CredentialManager{}
	if credentials, err := credentialDomain.GetAll(credentialService.Pool, offset, limit, orderBy); err != nil {
		return nil, err
	} else {
		outboundDto := make([]data.Credential, len(credentials))

		for i, credential := range credentials {
			outboundDto[i].FromManager(credential)
		}

		return outboundDto, nil
	}
}
