package auth

import (
	"net/http"
	"scheduler0/service"
	"scheduler0/utils"
)

// IsAndroidClient returns true is the request is coming from an android app
func IsAndroidClient(req *http.Request) bool {
	apiKey := req.Header.Get(APIKeyHeader)
	bundleID := req.Header.Get(AndroidPackageIDHeader)
	return len(apiKey) > 9 && len(bundleID) > 9
}

// IsAuthorizedAndroidClient returns true if the credential is authorized android app
func IsAuthorizedAndroidClient(req *http.Request, credentialService service.Credential) (bool, *utils.GenericError) {
	apiKey := req.Header.Get(APIKeyHeader)
	androidPackageName := req.Header.Get(AndroidPackageIDHeader)

	return credentialService.ValidateAndroidAPIKey(apiKey, androidPackageName)
}
