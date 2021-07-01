package ios

import (
	"github.com/go-pg/pg"
	"net/http"
	"scheduler0/server/http_server/middlewares/auth"
	"scheduler0/server/service"
	"scheduler0/utils"
)

// IsIOSClient returns true is the request is coming from an ios app
func IsIOSClient(req *http.Request) bool {
	apiKey := req.Header.Get(auth.APIKeyHeader)
	bundleID := req.Header.Get(auth.IOSBundleHeader)
	return len(apiKey) > 9 && len(bundleID) > 9
}

// IsAuthorizedIOSClient returns true if the credential is authorized ios app
func IsAuthorizedIOSClient(req *http.Request, dbConnection *pg.DB) (bool, *utils.GenericError) {
	apiKey := req.Header.Get(auth.APIKeyHeader)
	IOSBundleID := req.Header.Get(auth.IOSBundleHeader)

	credentialService := service.Credential{
		DBConnection: dbConnection,
	}

	return credentialService.ValidateIOSAPIKey(apiKey, IOSBundleID)
}
