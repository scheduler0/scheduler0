package android

import (
	"github.com/go-pg/pg"
	"net/http"
	"scheduler0/server/http_server/middlewares/auth"
	"scheduler0/server/service"
	"scheduler0/utils"
)

// IsAndroidClient returns true is the request is coming from an android app
func IsAndroidClient(req *http.Request) bool {
	apiKey := req.Header.Get(auth.APIKeyHeader)
	bundleID := req.Header.Get(auth.AndroidPackageIDHeader)
	return len(apiKey) > 9 && len(bundleID) > 9
}

// IsAuthorizedAndroidClient returns true if the credential is authorized android app
func IsAuthorizedAndroidClient(req *http.Request, dbConnection *pg.DB) (bool, *utils.GenericError) {
	apiKey := req.Header.Get(auth.APIKeyHeader)
	androidPackageName := req.Header.Get(auth.AndroidPackageIDHeader)

	credentialService := service.Credential{
		DBConnection: dbConnection,
	}

	return credentialService.ValidateAndroidAPIKey(apiKey, androidPackageName)
}
