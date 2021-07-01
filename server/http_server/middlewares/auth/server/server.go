package server

import (
	"github.com/go-pg/pg"
	"net/http"
	"scheduler0/server/http_server/middlewares/auth"
	"scheduler0/server/service"
	"scheduler0/utils"
)

// IsServerClient returns true is the request is coming from a server side
func IsServerClient(req *http.Request) bool {
	apiKey := req.Header.Get(auth.APIKeyHeader)
	apiSecret := req.Header.Get(auth.SecretKeyHeader)
	return len(apiKey) > 9 && len(apiSecret) > 9
}

// IsAuthorizedServerClient returns true if the credential is authorized server side
func IsAuthorizedServerClient(req *http.Request, dbConnection *pg.DB) (bool, *utils.GenericError) {
	apiKey := req.Header.Get(auth.APIKeyHeader)
	apiSecret := req.Header.Get(auth.SecretKeyHeader)

	credentialService := service.Credential{
		DBConnection: dbConnection,
	}

	return credentialService.ValidateServerAPIKey(apiKey, apiSecret)
}
