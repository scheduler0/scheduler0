package auth

import (
	"net/http"
	"scheduler0/server/service"
	"scheduler0/utils"
)

// IsWebClient returns true is the request is coming from a web client
func IsWebClient(req *http.Request) bool {
	apiKey := req.Header.Get(APIKeyHeader)
	return  len(apiKey) > 9
}

// IsAuthorizedWebClient returns true if the credential is an authorized web client
func IsAuthorizedWebClient(req *http.Request, pool *utils.Pool) (bool, *utils.GenericError) {
	apiKey := req.Header.Get(APIKeyHeader)

	credentialService := service.Credential{
		Pool: pool,
	}

	return credentialService.ValidateWebAPIKeyHTTPReferrerRestriction(apiKey, req.URL.Host)
}