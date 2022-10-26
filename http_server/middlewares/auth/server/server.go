package server

import (
	"net/http"
	"scheduler0/http_server/middlewares/auth"
	"scheduler0/service"
	"scheduler0/utils"
)

// IsServerClient returns true is the request is coming from a server side
func IsServerClient(req *http.Request) bool {
	apiKey := req.Header.Get(auth.APIKeyHeader)
	apiSecret := req.Header.Get(auth.SecretKeyHeader)
	return len(apiKey) > 9 && len(apiSecret) > 9
}

// IsAuthorizedServerClient returns true if the credential is authorized server side
func IsAuthorizedServerClient(req *http.Request, credentialService service.Credential) (bool, *utils.GenericError) {
	apiKey := req.Header.Get(auth.APIKeyHeader)
	apiSecret := req.Header.Get(auth.SecretKeyHeader)

	return credentialService.ValidateServerAPIKey(apiKey, apiSecret)
}