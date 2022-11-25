package auth

import (
	"net/http"
	"scheduler0/headers"
	"scheduler0/service"
	"scheduler0/utils"
)

// IsAndroidClient returns true is the request is coming from an android app
func IsAndroidClient(req *http.Request) bool {
	apiKey := req.Header.Get(headers.APIKeyHeader)
	bundleID := req.Header.Get(headers.AndroidPackageIDHeader)
	return apiKey != "" && bundleID != ""
}

// IsAuthorizedAndroidClient returns true if the credential is authorized android app
func IsAuthorizedAndroidClient(req *http.Request, credentialService service.Credential) (bool, *utils.GenericError) {
	apiKey := req.Header.Get(headers.APIKeyHeader)
	androidPackageName := req.Header.Get(headers.AndroidPackageIDHeader)

	return credentialService.ValidateAndroidAPIKey(apiKey, androidPackageName)
}
