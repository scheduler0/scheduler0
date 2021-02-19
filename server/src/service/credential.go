package service

import (
	"scheduler0/server/src/managers/credential"
	"scheduler0/server/src/transformers"
)

type CredentialService Service

func (credentialService *CredentialService) CreateNewCredential(HTTPReferrerRestriction string) (string, error) {
	credentialDto := transformers.Credential{HTTPReferrerRestriction: HTTPReferrerRestriction}
	credentialManager := credentialDto.ToManager()
	return credentialManager.CreateOne(credentialService.Pool)
}

func (credentialService *CredentialService) FindOneCredentialByUUID(UUID string) (*transformers.Credential, error) {
	credentialDto := transformers.Credential{UUID: UUID}
	credentialManager := credentialDto.ToManager()
	if err := credentialManager.GetOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := transformers.Credential{}
		outboundDto.FromManager(credentialManager)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) UpdateOneCredential(UUID string, HTTPReferrerRestriction string) (*transformers.Credential, error) {
	credentialDto := transformers.Credential{UUID: UUID, HTTPReferrerRestriction: HTTPReferrerRestriction}
	credentialManager := credentialDto.ToManager()
	if _, err := credentialManager.UpdateOne(credentialService.Pool); err != nil {
		return nil, err
	} else {
		outboundDto := transformers.Credential{}
		outboundDto.FromManager(credentialManager)
		return &outboundDto, nil
	}
}

func (credentialService *CredentialService) DeleteOneCredential(UUID string) (*transformers.Credential, error) {
	credentialDto := transformers.Credential{UUID: UUID}
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
	credentialManager := credential.CredentialManager{}
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
