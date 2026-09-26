package middlewares

import (
	"net/http"
	"net/http/httptest"
	"scheduler0/pkg/constants/headers"
	"scheduler0/pkg/models"
	"scheduler0/pkg/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// newActAsRequest builds an authorized peer request that asks to act on behalf of
// the given api key within account 123.
func newActAsRequest(method, path, apiKey string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set(headers.PeerHeader, headers.PeerHeaderCMDValue)
	req.SetBasicAuth("testuser", "testpass")
	req.Header.Set(headers.AccountIDHeader, "123")
	req.Header.Set(headers.ActAsAPIKeyHeader, apiKey)
	return req
}

func TestAuthMiddleware_ActAs_ValidCredentialSetsContext(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	fixture := validCredentialFixture()
	mockCredentialService.On("FindOneCredentialByAPIKey", "test-api-key", uint64(123)).
		Return(fixture, nil)

	var seen *models.Credential
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = r.Context().Value(utils.CredentialContextKey()).(*models.Credential)
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	handler.AuthMiddleware(mockCredentialService)(nextHandler).ServeHTTP(rr, newActAsRequest(http.MethodGet, "/api/v1/jobs", "test-api-key"))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotNil(t, seen, "acted-as credential should be placed in the request context")
	assert.Equal(t, fixture.ID, seen.ID)
	mockCredentialService.AssertExpectations(t)
}

func TestAuthMiddleware_ActAs_UnknownCredentialRejected(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	mockCredentialService.On("FindOneCredentialByAPIKey", "missing", uint64(123)).
		Return(nil, utils.HTTPGenericError(http.StatusNotFound, "credential doesn't exist"))

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run for an unknown act-as credential")
	})

	rr := httptest.NewRecorder()
	handler.AuthMiddleware(mockCredentialService)(nextHandler).ServeHTTP(rr, newActAsRequest(http.MethodGet, "/api/v1/jobs", "missing"))

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthMiddleware_ActAs_MissingScopeIsForbidden(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	readOnly := validCredentialFixture()
	readOnly.Scopes = []string{"read"}
	mockCredentialService.On("FindOneCredentialByAPIKey", "read-only", uint64(123)).
		Return(readOnly, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run when the acted-as credential lacks the scope")
	})

	rr := httptest.NewRecorder()
	handler.AuthMiddleware(mockCredentialService)(nextHandler).ServeHTTP(rr, newActAsRequest(http.MethodPost, "/api/v1/projects", "read-only"))

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAuthMiddleware_ActAs_ArchivedAndExpiredRejected(t *testing.T) {
	cases := map[string]func(c *models.Credential){
		"archived": func(c *models.Credential) { c.Archived = true },
		"expired": func(c *models.Credential) {
			past := time.Now().Add(-time.Hour)
			c.ExpiresAt = &past
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)
			cred := validCredentialFixture()
			mutate(cred)
			mockCredentialService.On("FindOneCredentialByAPIKey", "test-api-key", uint64(123)).Return(cred, nil)

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("next handler must not run for %s credential", name)
			})

			rr := httptest.NewRecorder()
			handler.AuthMiddleware(mockCredentialService)(nextHandler).ServeHTTP(rr, newActAsRequest(http.MethodGet, "/api/v1/jobs", "test-api-key"))
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}

func TestAuthMiddleware_ActAs_RequiresAccountID(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run without an account id")
	})

	req := newActAsRequest(http.MethodGet, "/api/v1/jobs", "test-api-key")
	req.Header.Del(headers.AccountIDHeader)
	rr := httptest.NewRecorder()
	handler.AuthMiddleware(mockCredentialService)(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthMiddleware_ActAs_IgnoredWithoutPeerAuth(t *testing.T) {
	// The header must never grant access on its own: without valid peer basic
	// auth the request is rejected before the credential is even looked up.
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run for an unauthenticated act-as request")
	})

	req := newActAsRequest(http.MethodGet, "/api/v1/jobs", "test-api-key")
	req.SetBasicAuth("wronguser", "wrongpass")
	rr := httptest.NewRecorder()
	handler.AuthMiddleware(mockCredentialService)(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
