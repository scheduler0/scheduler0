package middlewares

import (
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/constants/headers"
	"scheduler0/pkg/models"
	"scheduler0/pkg/service/credential"
	"scheduler0/pkg/utils"
	"strconv"
	"strings"
	"time"
)

func IsServerClient(req *http.Request) bool {
	apiKey := req.Header.Get(headers.APIKeyHeader)
	apiSecret := req.Header.Get(headers.SecretKeyHeader)
	return apiKey != "" && apiSecret != ""
}

func IsAuthorizedServerClient(req *http.Request, credentialService credential.CredentialService) (bool, *models.Credential, *utils.GenericError) {
	apiKey := req.Header.Get(headers.APIKeyHeader)
	apiSecret := req.Header.Get(headers.SecretKeyHeader)
	accountId := req.Header.Get(headers.AccountIDHeader)

	if accountId == "" {
		return false, nil, utils.HTTPGenericError(http.StatusUnauthorized, "account id is required")
	}

	accountIdUint, err := strconv.ParseUint(accountId, 10, 64)
	if err != nil {
		return false, nil, utils.HTTPGenericError(http.StatusUnauthorized, "invalid account id")
	}

	return credentialService.ValidateServerAPIKey(apiKey, apiSecret, accountIdUint)
}

// resolveActAsCredential resolves the credential a trusted peer wants to act as
// (see headers.ActAsAPIKeyHeader) and applies the same account, archived, expiry
// and scope checks a direct api-key request goes through. On failure it returns a
// nil credential plus the HTTP status and message to send.
func resolveActAsCredential(r *http.Request, apiKey string, credentialService credential.CredentialService) (*models.Credential, int, string) {
	accountIdHeader := strings.TrimSpace(r.Header.Get(headers.AccountIDHeader))
	if accountIdHeader == "" {
		return nil, http.StatusUnauthorized, "account id is required to act as a credential"
	}
	accountId, err := strconv.ParseUint(accountIdHeader, 10, 64)
	if err != nil || accountId == 0 {
		return nil, http.StatusUnauthorized, "invalid account id"
	}

	cred, findErr := credentialService.FindOneCredentialByAPIKey(apiKey, accountId)
	if findErr != nil || cred == nil {
		return nil, http.StatusUnauthorized, "credential not found for account"
	}
	if cred.Archived {
		return nil, http.StatusUnauthorized, "credential is archived"
	}
	if cred.IsExpired(time.Now()) {
		return nil, http.StatusUnauthorized, "credential expired"
	}
	required := requiredScopeForRequest(r)
	if !credentialSatisfiesRequiredScope(cred, required) {
		return nil, http.StatusForbidden, "credential missing required scope: " + required
	}
	return cred, http.StatusOK, ""
}

func credentialSatisfiesRequiredScope(cred *models.Credential, required string) bool {
	if required == "" {
		return true
	}
	return cred.HasScope(required) || cred.HasScope(constants.CredentialScopeAdmin)
}

func requiredScopeForRequest(r *http.Request) string {
	paths := strings.Split(r.URL.Path, "/")
	if len(paths) < 4 {
		return ""
	}
	endpoint := paths[3]
	subpath := ""
	if len(paths) >= 5 {
		subpath = paths[4]
	}

	switch endpoint {
	case "healthcheck", "peer-handshake":
		return ""
	case "accounts", "cluster":
		return constants.CredentialScopeAdmin
	case "internal":
		return constants.CredentialScopeAdmin
	case "account":
		if subpath == "rotate-secret" {
			return constants.CredentialScopeAdmin
		}
	case "ai":
		switch subpath {
		case "prompt", "suggestions", "schedule":
			return constants.CredentialScopeExecute
		case "settings":
			if r.Method == http.MethodGet {
				return constants.CredentialScopeRead
			}
			return constants.CredentialScopeWrite
		case "models", "prompt-requests":
			return constants.CredentialScopeRead
		}
	case "executors":
		if r.Method == http.MethodPost && len(paths) >= 6 && paths[5] == "test-invoke" {
			return constants.CredentialScopeExecute
		}
	case "local-executors":
		if r.Method == http.MethodPost && len(paths) >= 6 && paths[5] == "executions" {
			return constants.CredentialScopeExecute
		}
		if r.Method == http.MethodGet {
			return constants.CredentialScopeRead
		}
		return constants.CredentialScopeWrite
	case "executions":
		if r.Method == http.MethodPost && subpath == "cleanup-old-logs" {
			return constants.CredentialScopeExecute
		}
		return constants.CredentialScopeRead
	}

	switch r.Method {
	case http.MethodGet:
		return constants.CredentialScopeRead
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return constants.CredentialScopeWrite
	}

	return ""
}
