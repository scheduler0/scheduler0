package auth

import (
	"net/http"
	"scheduler0/service"
	"scheduler0/utils"
)

// IsIOSClient returns true is the request is coming from an ios app
func IsIOSClient(req *http.Request) bool {
	apiKey := req.Header.Get(APIKeyHeader)
	bundleID := req.Header.Get(IOSBundleHeader)
	return apiKey != "" && bundleID != ""
}

// IsAuthorizedIOSClient returns true if the credential is authorized ios app
func IsAuthorizedIOSClient(req *http.Request, credentialService service.Credential) (bool, *utils.GenericError) {
	apiKey := req.Header.Get(APIKeyHeader)
	IOSBundleID := req.Header.Get(IOSBundleHeader)

	return credentialService.ValidateIOSAPIKey(apiKey, IOSBundleID)
}
