package service

import (
	"context"
	"cron-server/server/db"
	"cron-server/server/db/managers"
	"cron-server/server/db/transformers"
)

type CredentialService struct {
	Pool *db.Pool
	Ctx  context.Context
}

func (credentialService *CredentialService) CreateNewCredential(HTTPReferrerRestriction string) (string, error) {
	credentialDto := transformers.Credential{HTTPReferrerRestriction: HTTPReferrerRestriction}
	credentialDomain := credentialDto.ToManager()
	return credentialDomain.CreateOne(credentialService.Pool)
}

func (credentialService *CredentialService) FindOneCredentialByID(ID string) (*transformers.Credential, error) {
	credentialDto := transformers.Credential{ID: ID}
	credentialDomain := credentialDto.ToManager()
	if _, err := credentialDomain.GetOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := transformers.Credential{}
		outboundDto.FromManager(credentialDomain)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) UpdateOneCredential(ID string) (*transformers.Credential, error) {
	credentialDto := transformers.Credential{ID: ID}
	credentialDomain := credentialDto.ToManager()
	if _, err := credentialDomain.UpdateOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := transformers.Credential{}
		outboundDto.FromManager(credentialDomain)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) DeleteOneCredential(ID string) (*transformers.Credential, error) {
	credentialDto := transformers.Credential{ID: ID}
	credentialDomain := credentialDto.ToManager()
	if _, err := credentialDomain.DeleteOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := transformers.Credential{}
		outboundDto.FromManager(credentialDomain)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) ListCredentials(offset int, limit int, orderBy string) ([]transformers.Credential, error) {
	credentialDomain := managers.CredentialManager{}
	if credentials, err := credentialDomain.GetAll(credentialService.Pool, offset, limit, orderBy); err != nil {
		return nil, err
	} else {
		outboundDto := make([]transformers.Credential, len(credentials))

		for i, credential := range credentials {
			outboundDto[i].FromManager(credential)
		}

		return outboundDto, nil
	}
}
