package middlewares

import (
	"net/http"
	"net/http/httptest"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/constants/headers"
	"scheduler0/pkg/mocks"
	"scheduler0/pkg/models"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRequiredScopeForRequest(t *testing.T) {
	cases := []struct {
		method   string
		path     string
		expected string
	}{
		{http.MethodGet, "/api/v1/healthcheck", ""},
		{http.MethodGet, "/api/v1/jobs", constants.CredentialScopeRead},
		{http.MethodGet, "/api/v1/jobs/123", constants.CredentialScopeRead},
		{http.MethodPost, "/api/v1/jobs", constants.CredentialScopeWrite},
		{http.MethodPut, "/api/v1/jobs/123", constants.CredentialScopeWrite},
		{http.MethodDelete, "/api/v1/jobs/123", constants.CredentialScopeWrite},
		{http.MethodPost, "/api/v1/projects", constants.CredentialScopeWrite},
		{http.MethodGet, "/api/v1/projects", constants.CredentialScopeRead},
		{http.MethodPost, "/api/v1/credentials", constants.CredentialScopeWrite},
		{http.MethodGet, "/api/v1/credentials", constants.CredentialScopeRead},
		{http.MethodPost, "/api/v1/executors", constants.CredentialScopeWrite},
		{http.MethodPost, "/api/v1/local-executors", constants.CredentialScopeWrite},
		{http.MethodGet, "/api/v1/local-executors/123/jobs", constants.CredentialScopeRead},
		{http.MethodPost, "/api/v1/local-executors/123/executions", constants.CredentialScopeExecute},
		{http.MethodGet, "/api/v1/executions/analytics", constants.CredentialScopeRead},
		{http.MethodPost, "/api/v1/executions/cleanup-old-logs", constants.CredentialScopeExecute},
		{http.MethodPost, "/api/v1/ai/prompt", constants.CredentialScopeExecute},
		{http.MethodPost, "/api/v1/ai/prompt/classify", constants.CredentialScopeExecute},
		{http.MethodPost, "/api/v1/ai/suggestions/analyze", constants.CredentialScopeExecute},
		{http.MethodPost, "/api/v1/ai/suggestions/time", constants.CredentialScopeExecute},
		{http.MethodPost, "/api/v1/ai/schedule", constants.CredentialScopeExecute},
		{http.MethodGet, "/api/v1/ai/settings", constants.CredentialScopeRead},
		{http.MethodPut, "/api/v1/ai/settings", constants.CredentialScopeWrite},
		{http.MethodGet, "/api/v1/ai/models", constants.CredentialScopeRead},
		{http.MethodGet, "/api/v1/ai/prompt-requests", constants.CredentialScopeRead},
		{http.MethodPost, "/api/v1/accounts", constants.CredentialScopeAdmin},
		{http.MethodGet, "/api/v1/accounts", constants.CredentialScopeAdmin},
		{http.MethodGet, "/api/v1/cluster/list-nodes", constants.CredentialScopeAdmin},
		{http.MethodPost, "/api/v1/cluster/backup", constants.CredentialScopeAdmin},
		{http.MethodPost, "/api/v1/account/rotate-secret", constants.CredentialScopeAdmin},
		{http.MethodGet, "/api/v1/peer-handshake", ""},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			assert.Equal(t, tc.expected, requiredScopeForRequest(req))
		})
	}
}

func TestAuthMiddleware_ExpiredCredentialReturns401(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	expired := time.Now().Add(-1 * time.Hour)
	cred := &models.Credential{
		ID:        7,
		ApiKey:    "test-api-key",
		ApiSecret: "test-api-secret",
		AccountId: 123,
		Scopes:    []string{"read", "write", "execute"},
		ExpiresAt: &expired,
	}
	mockCredentialService.On("ValidateServerAPIKey", "test-api-key", "test-api-secret", uint64(123)).
		Return(true, cred, nil)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.APIKeyHeader, "test-api-key")
	req.Header.Set(headers.SecretKeyHeader, "test-api-secret")
	req.Header.Set(headers.AccountIDHeader, "123")
	rr := httptest.NewRecorder()

	handler.AuthMiddleware(mockCredentialService)(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "expired credential should return 401")
	assert.False(t, nextCalled, "next handler should not be called")
}

func TestAuthMiddleware_MissingScopeReturns403(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	expires := time.Now().Add(time.Hour)
	cred := &models.Credential{
		ID:        7,
		ApiKey:    "test-api-key",
		ApiSecret: "test-api-secret",
		AccountId: 123,
		Scopes:    []string{constants.CredentialScopeRead},
		ExpiresAt: &expires,
	}
	mockCredentialService.On("ValidateServerAPIKey", "test-api-key", "test-api-secret", uint64(123)).
		Return(true, cred, nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
	req.Header.Set(headers.APIKeyHeader, "test-api-key")
	req.Header.Set(headers.SecretKeyHeader, "test-api-secret")
	req.Header.Set(headers.AccountIDHeader, "123")
	rr := httptest.NewRecorder()

	handler.AuthMiddleware(mockCredentialService)(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "missing scope should return 403")
}

func TestAuthMiddleware_ExecuteScopeRequiredForPrompt(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	expires := time.Now().Add(time.Hour)
	cred := &models.Credential{
		ID:        7,
		ApiKey:    "test-api-key",
		ApiSecret: "test-api-secret",
		AccountId: 123,
		Scopes:    []string{constants.CredentialScopeRead, constants.CredentialScopeWrite},
		ExpiresAt: &expires,
	}
	mockCredentialService.On("ValidateServerAPIKey", "test-api-key", "test-api-secret", uint64(123)).
		Return(true, cred, nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/prompt", nil)
	req.Header.Set(headers.APIKeyHeader, "test-api-key")
	req.Header.Set(headers.SecretKeyHeader, "test-api-secret")
	req.Header.Set(headers.AccountIDHeader, "123")
	rr := httptest.NewRecorder()

	handler.AuthMiddleware(mockCredentialService)(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "prompt should require execute scope")
}

func TestAuthMiddleware_AdminScopeReachesAccountsRoute(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	expires := time.Now().Add(time.Hour)
	cred := &models.Credential{
		ID:        7,
		ApiKey:    "admin-key",
		ApiSecret: "admin-secret",
		AccountId: 123,
		Scopes:    []string{constants.CredentialScopeAdmin},
		ExpiresAt: &expires,
	}
	mockCredentialService.On("ValidateServerAPIKey", "admin-key", "admin-secret", uint64(123)).
		Return(true, cred, nil)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", nil)
	req.Header.Set(headers.APIKeyHeader, "admin-key")
	req.Header.Set(headers.SecretKeyHeader, "admin-secret")
	req.Header.Set(headers.AccountIDHeader, "123")
	rr := httptest.NewRecorder()

	handler.AuthMiddleware(mockCredentialService)(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "admin credential should reach the accounts route")
	assert.True(t, nextCalled, "next handler should be called for an admin credential")
}

func TestAuthMiddleware_AdminScopeIsSuperscope(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	expires := time.Now().Add(time.Hour)
	// Credential carries ONLY the admin scope — it must still satisfy read/write/execute.
	cred := &models.Credential{
		ID:        7,
		ApiKey:    "admin-key",
		ApiSecret: "admin-secret",
		AccountId: 123,
		Scopes:    []string{constants.CredentialScopeAdmin},
		ExpiresAt: &expires,
	}
	mockCredentialService.On("ValidateServerAPIKey", "admin-key", "admin-secret", uint64(123)).
		Return(true, cred, nil)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// GET /jobs requires the read scope; an admin-only credential should pass.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.APIKeyHeader, "admin-key")
	req.Header.Set(headers.SecretKeyHeader, "admin-secret")
	req.Header.Set(headers.AccountIDHeader, "123")
	rr := httptest.NewRecorder()

	handler.AuthMiddleware(mockCredentialService)(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "admin scope should satisfy a read-scoped route")
	assert.True(t, nextCalled)
}

func TestAuthMiddleware_NonAdminBlockedFromClusterRoute(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	expires := time.Now().Add(time.Hour)
	cred := &models.Credential{
		ID:        7,
		ApiKey:    "user-key",
		ApiSecret: "user-secret",
		AccountId: 123,
		Scopes:    []string{constants.CredentialScopeRead, constants.CredentialScopeWrite, constants.CredentialScopeExecute},
		ExpiresAt: &expires,
	}
	mockCredentialService.On("ValidateServerAPIKey", "user-key", "user-secret", uint64(123)).
		Return(true, cred, nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/backup", nil)
	req.Header.Set(headers.APIKeyHeader, "user-key")
	req.Header.Set(headers.SecretKeyHeader, "user-secret")
	req.Header.Set(headers.AccountIDHeader, "123")
	rr := httptest.NewRecorder()

	handler.AuthMiddleware(mockCredentialService)(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "non-admin credential must be blocked from cluster routes")
}

func TestIsAuthorizedServerClient_ReturnsCredential(t *testing.T) {
	mockCredentialService := mocks.NewMockCredentialService(t)
	expires := time.Now().Add(time.Hour)
	expected := &models.Credential{
		ID:        9,
		ApiKey:    "k",
		ApiSecret: "s",
		AccountId: 5,
		Scopes:    []string{constants.CredentialScopeRead},
		ExpiresAt: &expires,
	}
	mockCredentialService.On("ValidateServerAPIKey", "k", "s", uint64(5)).
		Return(true, expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.APIKeyHeader, "k")
	req.Header.Set(headers.SecretKeyHeader, "s")
	req.Header.Set(headers.AccountIDHeader, "5")

	ok, got, err := IsAuthorizedServerClient(req, mockCredentialService)
	assert.True(t, ok)
	assert.Nil(t, err)
	assert.Equal(t, expected, got)
}
