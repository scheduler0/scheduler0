package auth

import (
	"net/http"
	"scheduler0/headers"
	"scheduler0/service"
	"scheduler0/utils"
)

// IsWebClient returns true is the request is coming from a web client
func IsWebClient(req *http.Request) bool {
	apiKey := req.Header.Get(headers.APIKeyHeader)
	return apiKey != "" && (req.Header.Get(headers.HTTPReferrerIDHeader) != "" || req.Header.Get(headers.IPReferrerIDHeader) != "")
}

// IsAuthorizedWebClient returns true if the credential is an authorized web client
func IsAuthorizedWebClient(req *http.Request, credentialService service.Credential) (bool, *utils.GenericError) {
	apiKey := req.Header.Get(headers.APIKeyHeader)
	return credentialService.ValidateWebAPIKeyHTTPReferrerRestriction(apiKey, req.Header.Get(headers.HTTPReferrerIDHeader), req.Header.Get(headers.IPReferrerIDHeader))
}
