package middlewares

import (
	"net/http"
	"scheduler0/constants/headers"
	"scheduler0/service/credential"
	"scheduler0/utils"
)

// IsServerClient returns true is the request is coming from a server side
func IsServerClient(req *http.Request) bool {
	apiKey := req.Header.Get(headers.APIKeyHeader)
	apiSecret := req.Header.Get(headers.SecretKeyHeader)
	return apiKey != "" && apiSecret != ""
}

// IsAuthorizedServerClient returns true if the credential is authorized server side
func IsAuthorizedServerClient(req *http.Request, credentialService credential.CredentialService) (bool, *utils.GenericError) {
	apiKey := req.Header.Get(headers.APIKeyHeader)
	apiSecret := req.Header.Get(headers.SecretKeyHeader)

	return credentialService.ValidateServerAPIKey(apiKey, apiSecret)
}
