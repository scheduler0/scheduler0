package web

import (
	"net/http"
	"scheduler0/http_server/middlewares/auth"
	"scheduler0/service"
	"scheduler0/utils"
)

// IsWebClient returns true is the request is coming from a web client
func IsWebClient(req *http.Request) bool {
	apiKey := req.Header.Get(auth.APIKeyHeader)
	return len(apiKey) > 9
}

// IsAuthorizedWebClient returns true if the credential is an authorized web client
func IsAuthorizedWebClient(req *http.Request, credentialService service.Credential) (bool, *utils.GenericError) {
	apiKey := req.Header.Get(auth.APIKeyHeader)

	return credentialService.ValidateWebAPIKeyHTTPReferrerRestriction(apiKey, req.URL.Host)
}
