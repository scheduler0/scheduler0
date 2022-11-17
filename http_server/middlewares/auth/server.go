package auth

import (
	"net/http"
	"scheduler0/service"
	"scheduler0/utils"
)

// IsServerClient returns true is the request is coming from a server side
func IsServerClient(req *http.Request) bool {
	apiKey := req.Header.Get(APIKeyHeader)
	apiSecret := req.Header.Get(SecretKeyHeader)
	return apiKey != "" && apiSecret != ""
}

// IsAuthorizedServerClient returns true if the credential is authorized server side
func IsAuthorizedServerClient(req *http.Request, credentialService service.Credential) (bool, *utils.GenericError) {
	apiKey := req.Header.Get(APIKeyHeader)
	apiSecret := req.Header.Get(SecretKeyHeader)

	return credentialService.ValidateServerAPIKey(apiKey, apiSecret)
}
