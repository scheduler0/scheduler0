package service

import (
	"net/http"
	"scheduler0/server/managers/credential"
	"scheduler0/server/transformers"
	"scheduler0/utils"
)

type CredentialService Service

func (credentialService *CredentialService) CreateNewCredential(credentialTransformer transformers.Credential) (string, *utils.GenericError) {
	credentialManager := credentialTransformer.ToManager()
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

func (credentialService *CredentialService) UpdateOneCredential(credentialTransformer transformers.Credential) (*transformers.Credential, error) {
	credentialManager := credentialTransformer.ToManager()
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

func (credentialService *CredentialService) ListCredentials(offset int, limit int, orderBy string) (*transformers.PaginatedCredential, *utils.GenericError) {
	credentialManager := credential.Manager{}

	total, err := credentialManager.Count(credentialService.Pool)
	if err != nil {
		return nil, err
	}

	if total < 1 {
		return  nil, utils.HTTPGenericError(http.StatusNotFound, "there no credentials")
	}

	if credentialManagers, err := credentialManager.GetAll(credentialService.Pool, offset, limit, orderBy); err != nil {
		return nil, err
	} else {
		credentialTransformers := make([]transformers.Credential, len(credentialManagers))

		for i, credentialManager := range credentialManagers {
			credentialTransformers[i].FromManager(credentialManager)
		}

		return &transformers.PaginatedCredential{
			Data: credentialTransformers,
			Total: total,
			Offset: offset,
			Limit: limit,
		}, nil
	}
}
