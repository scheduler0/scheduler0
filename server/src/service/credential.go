package service

import (
	"context"
	"cron-server/server/src/managers"
	"cron-server/server/src/misc"
	"cron-server/server/src/transformers"
)

type CredentialService struct {
	Pool *misc.Pool
	Ctx  context.Context
}

func (credentialService *CredentialService) CreateNewCredential(HTTPReferrerRestriction string) (string, error) {
	credentialDto := transformers.Credential{HTTPReferrerRestriction: HTTPReferrerRestriction}
	credentialManager := credentialDto.ToManager()
	return credentialManager.CreateOne(credentialService.Pool)
}

func (credentialService *CredentialService) FindOneCredentialByID(ID string) (*transformers.Credential, error) {
	credentialDto := transformers.Credential{ID: ID}
	credentialManager := credentialDto.ToManager()
	if err := credentialManager.GetOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := transformers.Credential{}
		outboundDto.FromManager(credentialManager)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) UpdateOneCredential(ID string, HTTPReferrerRestriction string) (*transformers.Credential, error) {
	credentialDto := transformers.Credential{ID: ID, HTTPReferrerRestriction: HTTPReferrerRestriction }
	credentialManager := credentialDto.ToManager()
	if _, err := credentialManager.UpdateOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := transformers.Credential{}
		outboundDto.FromManager(credentialManager)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) DeleteOneCredential(ID string) (*transformers.Credential, error) {
	credentialDto := transformers.Credential{ID: ID}
	credentialManager := credentialDto.ToManager()
	if _, err := credentialManager.DeleteOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := transformers.Credential{}
		outboundDto.FromManager(credentialManager)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) ListCredentials(offset int, limit int, orderBy string) ([]transformers.Credential, error) {
	credentialManager := managers.CredentialManager{}
	if credentials, err := credentialManager.GetAll(credentialService.Pool, offset, limit, orderBy); err != nil {
		return nil, err
	} else {
		outboundDto := make([]transformers.Credential, len(credentials))

		for i, credential := range credentials {
			outboundDto[i].FromManager(credential)
		}

		return outboundDto, nil
	}
}
