package auth

import (
	"net/http"
	"scheduler0/headers"
	"scheduler0/service"
	"scheduler0/utils"
)

// IsIOSClient returns true is the request is coming from an ios app
func IsIOSClient(req *http.Request) bool {
	apiKey := req.Header.Get(headers.APIKeyHeader)
	bundleID := req.Header.Get(headers.IOSBundleHeader)
	return apiKey != "" && bundleID != ""
}

// IsAuthorizedIOSClient returns true if the credential is authorized ios app
func IsAuthorizedIOSClient(req *http.Request, credentialService service.Credential) (bool, *utils.GenericError) {
	apiKey := req.Header.Get(headers.APIKeyHeader)
	IOSBundleID := req.Header.Get(headers.IOSBundleHeader)

	return credentialService.ValidateIOSAPIKey(apiKey, IOSBundleID)
}
