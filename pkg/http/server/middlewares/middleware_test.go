package middlewares

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"scheduler0-private/pkg/config"
	"scheduler0-private/pkg/constants/headers"
	"scheduler0-private/pkg/mocks"
	"scheduler0-private/pkg/models"
	"scheduler0-private/pkg/secrets"
	"scheduler0-private/pkg/service/etcd"
	"scheduler0-private/pkg/service/node"
	"scheduler0-private/pkg/utils"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// createTestSecrets creates a test secrets implementation by setting environment variables
func createTestSecrets(authUsername, authPassword, secretKey string) secrets.Scheduler0Secrets {
	os.Setenv("SCHEDULER0_AUTH_USERNAME", authUsername)
	os.Setenv("SCHEDULER0_AUTH_PASSWORD", authPassword)
	if secretKey != "" {
		os.Setenv("SCHEDULER0_SECRET_KEY", secretKey)
	}
	return secrets.NewScheduler0Secrets()
}

// validCredentialFixture returns a valid (non-expired, full-scope) credential used by
// auth-middleware tests that don't care about expiry/scope handling specifically.
func validCredentialFixture() *models.Credential {
	expires := time.Now().Add(24 * time.Hour)
	return &models.Credential{
		ID:        1,
		ApiKey:    "test-api-key",
		ApiSecret: "test-api-secret",
		AccountId: 123,
		Scopes:    []string{"read", "write", "execute"},
		ExpiresAt: &expires,
	}
}

// mockScheduler0Config is a mock implementation of Scheduler0Config
type mockScheduler0Config struct {
	configs *config.Scheduler0Configurations
}

func (m *mockScheduler0Config) GetConfigurations() *config.Scheduler0Configurations {
	return m.configs
}

func setupMiddlewareHandler(t *testing.T) (*middlewareHandler, *mocks.MockCredentialService, *node.MockNodeService, *etcd.MockEtcdService) {
	logger := log.New(&strings.Builder{}, "", 0)
	testSecrets := createTestSecrets("testuser", "testpass", "")
	mockConfig := &mockScheduler0Config{
		configs: &config.Scheduler0Configurations{
			NodeId: 1,
		},
	}
	mockCredentialService := mocks.NewMockCredentialService(t)
	mockNodeService := node.NewMockNodeService(t)
	mockEtcdService := etcd.NewMockEtcdService(t)

	handler := NewMiddlewareHandler(logger, testSecrets, mockConfig, mockEtcdService)
	return handler.(*middlewareHandler), mockCredentialService, mockNodeService, mockEtcdService
}

func TestContextMiddleware(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Context().Value(utils.RequestIDContextKey())
		assert.NotNil(t, requestID, "Request ID should be set in context")
		assert.IsType(t, "", requestID, "Request ID should be a string")
		assert.NotEmpty(t, requestID.(string), "Request ID should not be empty")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rr := httptest.NewRecorder()

	middleware := handler.ContextMiddleware(nextHandler)
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Status code should be OK")
}

func TestContextMiddleware_UniqueRequestIDs(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	requestIDs := make(map[string]bool)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Context().Value(utils.RequestIDContextKey()).(string)
		assert.False(t, requestIDs[requestID], "Request ID should be unique")
		requestIDs[requestID] = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.ContextMiddleware(nextHandler)

	// Make multiple requests
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
	}

	assert.Equal(t, 10, len(requestIDs), "Should have 10 unique request IDs")
}

func TestAuthMiddleware_HealthcheckBypass(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthcheck", nil)
	rr := httptest.NewRecorder()

	middleware := handler.AuthMiddleware(mockCredentialService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Healthcheck should bypass authentication")
}

func TestAuthMiddleware_PeerClient_ValidAuth(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
	req.SetBasicAuth("testuser", "testpass")
	rr := httptest.NewRecorder()

	middleware := handler.AuthMiddleware(nil)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Valid peer client should be authorized")
}

func TestAuthMiddleware_PeerClient_InvalidAuth(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
	req.SetBasicAuth("wronguser", "wrongpass")
	rr := httptest.NewRecorder()

	middleware := handler.AuthMiddleware(nil)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Invalid peer client should be unauthorized")
}

func TestAuthMiddleware_ServerClient_ValidAuth(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	mockCredentialService.On("ValidateServerAPIKey", "test-api-key", "test-api-secret", uint64(123)).
		Return(true, validCredentialFixture(), nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.APIKeyHeader, "test-api-key")
	req.Header.Set(headers.SecretKeyHeader, "test-api-secret")
	req.Header.Set(headers.AccountIDHeader, "123")
	rr := httptest.NewRecorder()

	middleware := handler.AuthMiddleware(mockCredentialService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Valid server client should be authorized")
	mockCredentialService.AssertExpectations(t)
}

func TestAuthMiddleware_ServerClient_InvalidAuth(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	mockCredentialService.On("ValidateServerAPIKey", "test-api-key", "test-api-secret", uint64(123)).
		Return(false, nil, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.APIKeyHeader, "test-api-key")
	req.Header.Set(headers.SecretKeyHeader, "test-api-secret")
	req.Header.Set(headers.AccountIDHeader, "123")
	rr := httptest.NewRecorder()

	middleware := handler.AuthMiddleware(mockCredentialService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Invalid server client should be unauthorized")
	mockCredentialService.AssertExpectations(t)
}

func TestAuthMiddleware_ServerClient_AccountsEndpoint(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// The accounts endpoint accepts peer/basic auth (operator bootstrap). It also
	// accepts an admin-scoped api-key credential — see
	// TestAuthMiddleware_AdminScopeReachesAccountsRoute for that positive path.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	req.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
	req.SetBasicAuth("testuser", "testpass")
	rr := httptest.NewRecorder()

	middleware := handler.AuthMiddleware(mockCredentialService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Accounts endpoint should allow peer clients")

	// An api-key request that omits the X-Account-ID header cannot be validated as a
	// server client and, with no peer header, is unauthorized.
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	req2.Header.Set(headers.APIKeyHeader, "test-api-key")
	req2.Header.Set(headers.SecretKeyHeader, "test-api-secret")
	rr2 := httptest.NewRecorder()

	middleware(nextHandler).ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusUnauthorized, rr2.Code, "Accounts endpoint with an unvalidatable api-key request should be unauthorized")
}

func TestAuthMiddleware_Unauthorized(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	// No auth headers
	rr := httptest.NewRecorder()

	middleware := handler.AuthMiddleware(mockCredentialService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Request without auth should be unauthorized")
}

func TestEnsureRaftLeaderMiddleware_ClusterEndpointBypass(t *testing.T) {
	handler, _, mockNodeService, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/test", nil)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Cluster endpoint should bypass raft leader check")
}

func TestEnsureRaftLeaderMiddleware_ClusterBackupPOST_RequiresLeaderRedirect(t *testing.T) {
	handler, _, mockNodeService, mockEtcdService := setupMiddlewareHandler(t)

	mockNodeService.On("CanAcceptRequest").Return(true)
	mockNodeService.On("CanAcceptClientWriteRequest").Return(false)
	mockNodeService.On("GetRaftLeaderWithId").Return(raft.ServerAddress("leader:8080"), raft.ServerID("2"))

	peers := []config.RaftNode{{NodeId: 2, ClientAddress: "http://leader:8080"}}
	mockEtcdService.On("GetPeers", mock.Anything).Return(peers, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/backup", nil)
	req.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMovedPermanently, rr.Code)
	assert.Equal(t, "http://leader:8080/api/v1/cluster/backup", rr.Header().Get("Location"))
	mockNodeService.AssertExpectations(t)
	mockEtcdService.AssertExpectations(t)
}

func TestEnsureRaftLeaderMiddleware_ClusterListNodesGET_Bypass(t *testing.T) {
	handler, _, mockNodeService, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/list-nodes", nil)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Cluster GET should bypass raft leader check")
}

func TestEnsureRaftLeaderMiddleware_PeerHandshakeBypass(t *testing.T) {
	handler, _, mockNodeService, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/peer-handshake", nil)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Peer handshake endpoint should bypass raft leader check")
}

func TestEnsureRaftLeaderMiddleware_CannotAcceptRequest(t *testing.T) {
	handler, _, mockNodeService, _ := setupMiddlewareHandler(t)

	mockNodeService.On("CanAcceptRequest").Return(false)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "Should return service unavailable if cannot accept request")
	mockNodeService.AssertExpectations(t)
}

func TestEnsureRaftLeaderMiddleware_CanAcceptClientWriteRequest_GET(t *testing.T) {
	handler, _, mockNodeService, _ := setupMiddlewareHandler(t)

	mockNodeService.On("CanAcceptRequest").Return(true)
	mockNodeService.On("CanAcceptClientWriteRequest").Return(false)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	// GET requests should pass through even if CanAcceptClientWriteRequest is false
	assert.Equal(t, http.StatusOK, rr.Code, "GET requests should pass through")
	mockNodeService.AssertExpectations(t)
}

func TestEnsureRaftLeaderMiddleware_CannotAcceptClientWriteRequest_POST(t *testing.T) {
	handler, _, mockNodeService, mockEtcdService := setupMiddlewareHandler(t)

	mockNodeService.On("CanAcceptRequest").Return(true)
	mockNodeService.On("CanAcceptClientWriteRequest").Return(false)
	mockNodeService.On("GetRaftLeaderWithId").Return(raft.ServerAddress("leader:8080"), raft.ServerID("2"))

	// Mock etcd service to return peer info
	peers := []config.RaftNode{
		{
			NodeId:        2,
			ClientAddress: "http://leader:8080",
		},
	}
	mockEtcdService.On("GetPeers", mock.Anything).Return(peers, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
	req.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMovedPermanently, rr.Code, "Should redirect to leader")
	assert.Equal(t, "http://leader:8080/api/v1/jobs", rr.Header().Get("Location"), "Should set redirect location")
	mockNodeService.AssertExpectations(t)
	mockEtcdService.AssertExpectations(t)
}

func TestEnsureRaftLeaderMiddleware_LeaderIsSelf(t *testing.T) {
	handler, _, mockNodeService, _ := setupMiddlewareHandler(t)

	mockNodeService.On("CanAcceptRequest").Return(true)
	mockNodeService.On("CanAcceptClientWriteRequest").Return(false)
	mockNodeService.On("GetRaftLeaderWithId").Return(raft.ServerAddress("self:8080"), raft.ServerID("1"))

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "Should return service unavailable if leader is self but cannot accept requests")
	mockNodeService.AssertExpectations(t)
}

func TestAccountIDMiddleware_NoAccountIDRequired(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthcheck", nil)
	rr := httptest.NewRecorder()

	middleware := handler.AccountIDMiddleware(nextHandler)
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Endpoints that don't require account ID should pass through")
}

func TestAccountIDMiddleware_JobsEndpoint_ValidAccountID(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := r.Context().Value(utils.AccountIDContextKey()).(uint64)
		assert.True(t, ok, "Account ID should be in context")
		assert.Equal(t, uint64(123), accountID, "Account ID should match")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.AccountIDHeader, "123")
	rr := httptest.NewRecorder()

	middleware := handler.AccountIDMiddleware(nextHandler)
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Valid account ID should pass through")
}

func TestAccountIDMiddleware_JobsEndpoint_MissingAccountID(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	// No account ID header
	rr := httptest.NewRecorder()

	middleware := handler.AccountIDMiddleware(nextHandler)
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Missing account ID should return bad request")
}

func TestAccountIDMiddleware_JobsEndpoint_InvalidAccountID(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.AccountIDHeader, "invalid")
	rr := httptest.NewRecorder()

	middleware := handler.AccountIDMiddleware(nextHandler)
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Invalid account ID should return bad request")
}

func TestAccountIDMiddleware_JobsEndpoint_ZeroAccountID(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.AccountIDHeader, "0")
	rr := httptest.NewRecorder()

	middleware := handler.AccountIDMiddleware(nextHandler)
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Zero account ID should return bad request")
}

func TestAccountIDMiddleware_AllRequiredEndpoints(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	endpoints := []string{"jobs", "projects", "credentials", "executors", "async-tasks", "executions"}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				accountID, ok := r.Context().Value(utils.AccountIDContextKey()).(uint64)
				assert.True(t, ok, "Account ID should be in context for %s", endpoint)
				assert.Equal(t, uint64(456), accountID, "Account ID should match for %s", endpoint)
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/"+endpoint, nil)
			req.Header.Set(headers.AccountIDHeader, "456")
			rr := httptest.NewRecorder()

			middleware := handler.AccountIDMiddleware(nextHandler)
			middleware.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code, "Endpoint %s should accept valid account ID", endpoint)
		})
	}
}

func TestIsPeerClient(t *testing.T) {
	tests := []struct {
		name           string
		peerHeader     string
		expectedResult bool
	}{
		{
			name:           "Peer header with 'peer' value",
			peerHeader:     headers.PeerHeaderValue,
			expectedResult: true,
		},
		{
			name:           "Peer header with 'cmd' value",
			peerHeader:     headers.PeerHeaderCMDValue,
			expectedResult: true,
		},
		{
			name:           "Peer header with other value",
			peerHeader:     "other",
			expectedResult: false,
		},
		{
			name:           "No peer header",
			peerHeader:     "",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
			if tt.peerHeader != "" {
				req.Header.Set(headers.PeerHeader, tt.peerHeader)
			}

			result := IsPeerClient(req)
			assert.Equal(t, tt.expectedResult, result, "IsPeerClient should return %v for peer header '%s'", tt.expectedResult, tt.peerHeader)
		})
	}
}

func TestIsAuthorizedPeerClient(t *testing.T) {
	tests := []struct {
		name           string
		username       string
		password       string
		expectedResult bool
	}{
		{
			name:           "Valid credentials",
			username:       "testuser",
			password:       "testpass",
			expectedResult: true,
		},
		{
			name:           "Invalid username",
			username:       "wronguser",
			password:       "testpass",
			expectedResult: false,
		},
		{
			name:           "Invalid password",
			username:       "testuser",
			password:       "wrongpass",
			expectedResult: false,
		},
		{
			name:           "No basic auth",
			username:       "",
			password:       "",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
			if tt.username != "" && tt.password != "" {
				req.SetBasicAuth(tt.username, tt.password)
			}

			testSecrets := createTestSecrets("testuser", "testpass", "")
			result := IsAuthorizedPeerClient(req, testSecrets)
			assert.Equal(t, tt.expectedResult, result, "IsAuthorizedPeerClient should return %v for username '%s' and password '%s'", tt.expectedResult, tt.username, tt.password)
		})
	}
}

func TestIsServerClient(t *testing.T) {
	tests := []struct {
		name           string
		apiKey         string
		apiSecret      string
		expectedResult bool
	}{
		{
			name:           "Both headers present",
			apiKey:         "test-api-key",
			apiSecret:      "test-api-secret",
			expectedResult: true,
		},
		{
			name:           "Only API key present",
			apiKey:         "test-api-key",
			apiSecret:      "",
			expectedResult: false,
		},
		{
			name:           "Only API secret present",
			apiKey:         "",
			apiSecret:      "test-api-secret",
			expectedResult: false,
		},
		{
			name:           "Neither header present",
			apiKey:         "",
			apiSecret:      "",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
			if tt.apiKey != "" {
				req.Header.Set(headers.APIKeyHeader, tt.apiKey)
			}
			if tt.apiSecret != "" {
				req.Header.Set(headers.SecretKeyHeader, tt.apiSecret)
			}

			result := IsServerClient(req)
			assert.Equal(t, tt.expectedResult, result, "IsServerClient should return %v for apiKey '%s' and apiSecret '%s'", tt.expectedResult, tt.apiKey, tt.apiSecret)
		})
	}
}

func TestIsAuthorizedServerClient(t *testing.T) {
	tests := []struct {
		name           string
		apiKey         string
		apiSecret      string
		accountID      string
		mockReturn     bool
		mockError      *utils.GenericError
		expectedResult bool
		expectedError  bool
	}{
		{
			name:           "Valid credentials",
			apiKey:         "test-api-key",
			apiSecret:      "test-api-secret",
			accountID:      "123",
			mockReturn:     true,
			mockError:      nil,
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:           "Invalid credentials",
			apiKey:         "test-api-key",
			apiSecret:      "test-api-secret",
			accountID:      "123",
			mockReturn:     false,
			mockError:      nil,
			expectedResult: false,
			expectedError:  false,
		},
		{
			name:           "Missing account ID",
			apiKey:         "test-api-key",
			apiSecret:      "test-api-secret",
			accountID:      "",
			mockReturn:     false,
			mockError:      nil,
			expectedResult: false,
			expectedError:  true,
		},
		{
			name:           "Invalid account ID",
			apiKey:         "test-api-key",
			apiSecret:      "test-api-secret",
			accountID:      "invalid",
			mockReturn:     false,
			mockError:      nil,
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCredentialService := mocks.NewMockCredentialService(t)
			if tt.accountID != "" {
				accountIDUint, _ := strconv.ParseUint(tt.accountID, 10, 64)
				if !tt.expectedError {
					var cred *models.Credential
					if tt.mockReturn {
						cred = validCredentialFixture()
					}
					mockCredentialService.On("ValidateServerAPIKey", tt.apiKey, tt.apiSecret, accountIDUint).
						Return(tt.mockReturn, cred, tt.mockError)
				}
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
			req.Header.Set(headers.APIKeyHeader, tt.apiKey)
			req.Header.Set(headers.SecretKeyHeader, tt.apiSecret)
			if tt.accountID != "" {
				req.Header.Set(headers.AccountIDHeader, tt.accountID)
			}

			result, _, err := IsAuthorizedServerClient(req, mockCredentialService)

			assert.Equal(t, tt.expectedResult, result, "IsAuthorizedServerClient should return %v", tt.expectedResult)
			if tt.expectedError {
				assert.NotNil(t, err, "Should return error when account ID is missing or invalid")
			} else {
				assert.Nil(t, err, "Should not return error for valid input")
			}

			if !tt.expectedError {
				mockCredentialService.AssertExpectations(t)
			}
		})
	}
}

// Helper function to create a test request with Basic Auth
func createRequestWithBasicAuth(method, url, username, password string) *http.Request {
	req := httptest.NewRequest(method, url, nil)
	auth := username + ":" + password
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", "Basic "+encodedAuth)
	return req
}

func TestIsAuthorizedPeerClient_WithBasicAuth(t *testing.T) {
	testSecrets := createTestSecrets("testuser", "testpass", "")

	// Test with valid credentials
	req := createRequestWithBasicAuth(http.MethodGet, "/api/v1/test", "testuser", "testpass")
	result := IsAuthorizedPeerClient(req, testSecrets)
	assert.True(t, result, "Should authorize valid peer client")

	// Test with invalid credentials
	req = createRequestWithBasicAuth(http.MethodGet, "/api/v1/test", "wronguser", "wrongpass")
	result = IsAuthorizedPeerClient(req, testSecrets)
	assert.False(t, result, "Should not authorize invalid peer client")
}

// ========== Edge Case Tests ==========
// Note: Path-related edge cases (empty paths, root paths, short paths) would be
// handled by the HTTP router returning 404 before reaching the middleware.
// We focus on edge cases that can occur with valid routes.

func TestAuthMiddleware_EdgeCase_ServerClientWithError(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	genericError := utils.HTTPGenericError(http.StatusInternalServerError, "database error")
	mockCredentialService.On("ValidateServerAPIKey", "test-api-key", "test-api-secret", uint64(123)).
		Return(false, nil, genericError)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.APIKeyHeader, "test-api-key")
	req.Header.Set(headers.SecretKeyHeader, "test-api-secret")
	req.Header.Set(headers.AccountIDHeader, "123")
	rr := httptest.NewRecorder()

	middleware := handler.AuthMiddleware(mockCredentialService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Server client with validation error should be unauthorized")
	mockCredentialService.AssertExpectations(t)
}

func TestAuthMiddleware_EdgeCase_ClusterEndpointWithServerClient(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/test", nil)
	req.Header.Set(headers.APIKeyHeader, "test-api-key")
	req.Header.Set(headers.SecretKeyHeader, "test-api-secret")
	rr := httptest.NewRecorder()

	middleware := handler.AuthMiddleware(mockCredentialService)
	middleware(nextHandler).ServeHTTP(rr, req)

	// Cluster endpoints require an admin-scoped credential (or peer auth). This api-key
	// request omits the X-Account-ID header so it cannot be validated, and with no peer
	// header it is unauthorized. The admin-scope positive/negative paths are covered by
	// TestAuthMiddleware_AdminScopeReachesAccountsRoute and _NonAdminBlockedFromClusterRoute.
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Cluster endpoint with an unvalidatable api-key request should be unauthorized")
}

func TestAuthMiddleware_EdgeCase_BothServerAndPeerClient(t *testing.T) {
	handler, mockCredentialService, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.APIKeyHeader, "test-api-key")
	req.Header.Set(headers.SecretKeyHeader, "test-api-secret")
	req.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
	req.SetBasicAuth("testuser", "testpass")
	rr := httptest.NewRecorder()

	middleware := handler.AuthMiddleware(mockCredentialService)
	middleware(nextHandler).ServeHTTP(rr, req)

	// Peer client check comes after server client, so peer should take precedence
	assert.Equal(t, http.StatusOK, rr.Code, "Peer client should take precedence when both are present")
}

// Removed EmptyPath and ShortPath tests for EnsureRaftLeaderMiddleware
// as these would be handled by the HTTP router (404) before reaching middleware

func TestEnsureRaftLeaderMiddleware_EdgeCase_EtcdServiceNil(t *testing.T) {
	logger := log.New(&strings.Builder{}, "", 0)
	testSecrets := createTestSecrets("testuser", "testpass", "")
	mockConfig := &mockScheduler0Config{
		configs: &config.Scheduler0Configurations{
			NodeId: 1,
		},
	}
	mockNodeService := node.NewMockNodeService(t)

	// Create handler with nil etcdService
	handler := NewMiddlewareHandler(logger, testSecrets, mockConfig, nil)
	middlewareHandler := handler.(*middlewareHandler)

	mockNodeService.On("CanAcceptRequest").Return(true)
	mockNodeService.On("CanAcceptClientWriteRequest").Return(false)
	mockNodeService.On("GetRaftLeaderWithId").Return(raft.ServerAddress("leader:8080"), raft.ServerID("2"))

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
	rr := httptest.NewRecorder()

	middleware := middlewareHandler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "Should return service unavailable when etcd service is nil")
	mockNodeService.AssertExpectations(t)
}

func TestEnsureRaftLeaderMiddleware_EdgeCase_EtcdGetPeersError(t *testing.T) {
	handler, _, mockNodeService, mockEtcdService := setupMiddlewareHandler(t)

	mockNodeService.On("CanAcceptRequest").Return(true)
	mockNodeService.On("CanAcceptClientWriteRequest").Return(false)
	mockNodeService.On("GetRaftLeaderWithId").Return(raft.ServerAddress("leader:8080"), raft.ServerID("2"))

	mockEtcdService.On("GetPeers", mock.Anything).Return([]config.RaftNode{}, fmt.Errorf("etcd connection failed"))

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "Should return service unavailable when etcd GetPeers fails")
	mockNodeService.AssertExpectations(t)
	mockEtcdService.AssertExpectations(t)
}

func TestEnsureRaftLeaderMiddleware_EdgeCase_LeaderNodeIdParseError(t *testing.T) {
	handler, _, mockNodeService, mockEtcdService := setupMiddlewareHandler(t)

	mockNodeService.On("CanAcceptRequest").Return(true)
	mockNodeService.On("CanAcceptClientWriteRequest").Return(false)
	// Return invalid server ID that can't be parsed as uint64
	mockNodeService.On("GetRaftLeaderWithId").Return(raft.ServerAddress("leader:8080"), raft.ServerID("invalid-id"))

	mockEtcdService.On("GetPeers", mock.Anything).Return([]config.RaftNode{}, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "Should return service unavailable when leader node ID parsing fails")
	mockNodeService.AssertExpectations(t)
	mockEtcdService.AssertExpectations(t)
}

func TestEnsureRaftLeaderMiddleware_EdgeCase_LeaderNotFoundInPeers(t *testing.T) {
	handler, _, mockNodeService, mockEtcdService := setupMiddlewareHandler(t)

	mockNodeService.On("CanAcceptRequest").Return(true)
	mockNodeService.On("CanAcceptClientWriteRequest").Return(false)
	mockNodeService.On("GetRaftLeaderWithId").Return(raft.ServerAddress("leader:8080"), raft.ServerID("999"))

	// Return peers that don't include the leader
	peers := []config.RaftNode{
		{
			NodeId:        1,
			ClientAddress: "http://node1:8080",
		},
		{
			NodeId:        2,
			ClientAddress: "http://node2:8080",
		},
	}
	mockEtcdService.On("GetPeers", mock.Anything).Return(peers, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "Should return service unavailable when leader not found in peers")
	mockNodeService.AssertExpectations(t)
	mockEtcdService.AssertExpectations(t)
}

func TestEnsureRaftLeaderMiddleware_EdgeCase_LeaderWithEmptyClientAddress(t *testing.T) {
	handler, _, mockNodeService, mockEtcdService := setupMiddlewareHandler(t)

	mockNodeService.On("CanAcceptRequest").Return(true)
	mockNodeService.On("CanAcceptClientWriteRequest").Return(false)
	mockNodeService.On("GetRaftLeaderWithId").Return(raft.ServerAddress("leader:8080"), raft.ServerID("2"))

	// Return peer with empty ClientAddress
	peers := []config.RaftNode{
		{
			NodeId:        2,
			ClientAddress: "", // Empty address
		},
	}
	mockEtcdService.On("GetPeers", mock.Anything).Return(peers, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "Should return service unavailable when leader has empty client address")
	mockNodeService.AssertExpectations(t)
	mockEtcdService.AssertExpectations(t)
}

func TestEnsureRaftLeaderMiddleware_EdgeCase_NonPeerClientWriteRequest(t *testing.T) {
	handler, _, mockNodeService, mockEtcdService := setupMiddlewareHandler(t)

	mockNodeService.On("CanAcceptRequest").Return(true)
	mockNodeService.On("CanAcceptClientWriteRequest").Return(false)
	mockNodeService.On("GetRaftLeaderWithId").Return(raft.ServerAddress("leader:8080"), raft.ServerID("2"))

	peers := []config.RaftNode{
		{
			NodeId:        2,
			ClientAddress: "http://leader:8080",
		},
	}
	mockEtcdService.On("GetPeers", mock.Anything).Return(peers, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Request without peer header (regular client)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	// Non-peer clients should get 302 Found, not 301 Moved Permanently
	assert.Equal(t, http.StatusFound, rr.Code, "Non-peer client should get 302 Found")
	assert.Equal(t, "http://leader:8080/api/v1/jobs", rr.Header().Get("Location"), "Should set redirect location")
	mockNodeService.AssertExpectations(t)
	mockEtcdService.AssertExpectations(t)
}

func TestEnsureRaftLeaderMiddleware_EdgeCase_PATCHMethod(t *testing.T) {
	handler, _, mockNodeService, _ := setupMiddlewareHandler(t)

	mockNodeService.On("CanAcceptRequest").Return(true)
	mockNodeService.On("CanAcceptClientWriteRequest").Return(true)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// PATCH is a write method but not explicitly checked in the middleware
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/jobs/123", nil)
	rr := httptest.NewRecorder()

	middleware := handler.EnsureRaftLeaderMiddleware(mockNodeService)
	middleware(nextHandler).ServeHTTP(rr, req)

	// PATCH is not in the list (POST, DELETE, PUT), so it should pass through
	assert.Equal(t, http.StatusOK, rr.Code, "PATCH method should pass through")
	mockNodeService.AssertExpectations(t)
}

// Removed EmptyPath and ShortPath tests for AccountIDMiddleware
// as these would be handled by the HTTP router (404) before reaching middleware
// The middleware correctly handles len(paths) < 4 by passing through, which is good defensive programming

func TestAccountIDMiddleware_EdgeCase_MaxUint64AccountID(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := r.Context().Value(utils.AccountIDContextKey()).(uint64)
		assert.True(t, ok, "Account ID should be in context")
		assert.Equal(t, uint64(18446744073709551615), accountID, "Account ID should be max uint64")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.AccountIDHeader, "18446744073709551615") // Max uint64
	rr := httptest.NewRecorder()

	middleware := handler.AccountIDMiddleware(nextHandler)
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Max uint64 account ID should be valid")
}

func TestAccountIDMiddleware_EdgeCase_OverflowAccountID(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.AccountIDHeader, "18446744073709551616") // Overflow uint64
	rr := httptest.NewRecorder()

	middleware := handler.AccountIDMiddleware(nextHandler)
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Account ID overflow should return bad request")
}

func TestAccountIDMiddleware_EdgeCase_NegativeNumberString(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.AccountIDHeader, "-123")
	rr := httptest.NewRecorder()

	middleware := handler.AccountIDMiddleware(nextHandler)
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Negative account ID should return bad request")
}

func TestAccountIDMiddleware_EdgeCase_NonNumericAccountID(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.AccountIDHeader, "abc123")
	rr := httptest.NewRecorder()

	middleware := handler.AccountIDMiddleware(nextHandler)
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Non-numeric account ID should return bad request")
}

func TestAccountIDMiddleware_EdgeCase_AccountIDWithSpaces(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := r.Context().Value(utils.AccountIDContextKey()).(uint64)
		assert.True(t, ok, "Account ID should be in context")
		assert.Equal(t, uint64(123), accountID, "Account ID should be trimmed and parsed correctly")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(headers.AccountIDHeader, " 123 ")
	rr := httptest.NewRecorder()

	middleware := handler.AccountIDMiddleware(nextHandler)
	middleware.ServeHTTP(rr, req)

	// After trimming, spaces should be handled gracefully
	assert.Equal(t, http.StatusOK, rr.Code, "Account ID with spaces should be trimmed and accepted")
}

func TestAccountIDMiddleware_EdgeCase_AccountIDWithSpecialChars(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	testCases := []string{
		"123.456",
		"123e10",
		"0x123",
		"123abc",
		"12 34",
	}

	for _, accountID := range testCases {
		t.Run(accountID, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
			req.Header.Set(headers.AccountIDHeader, accountID)
			rr := httptest.NewRecorder()

			middleware := handler.AccountIDMiddleware(nextHandler)
			middleware.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code, "Account ID with special chars should return bad request")
		})
	}
}

func TestIsAuthorizedPeerClient_EdgeCase_MalformedBasicAuth(t *testing.T) {
	testSecrets := createTestSecrets("testuser", "testpass", "")

	// Test with malformed Authorization header
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "Basic invalid-base64")
	result := IsAuthorizedPeerClient(req, testSecrets)
	assert.False(t, result, "Malformed Basic Auth should return false")
}

func TestIsAuthorizedPeerClient_EdgeCase_EmptyCredentials(t *testing.T) {
	testSecrets := createTestSecrets("testuser", "testpass", "")

	// Test with empty username and password
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.SetBasicAuth("", "")
	result := IsAuthorizedPeerClient(req, testSecrets)
	assert.False(t, result, "Empty credentials should return false")
}

func TestIsAuthorizedPeerClient_EdgeCase_SpecialCharacters(t *testing.T) {
	// Create secrets instance and update cache with special character credentials
	// This works around the cachedSecrets issue in the secrets package
	testSecrets := secrets.NewScheduler0Secrets()

	// Get the secrets struct (which we can modify since fields are exported)
	// Then update it with our test credentials and save it back
	secretsData := testSecrets.GetSecrets()
	if secretsData == nil {
		// If cache is nil, we need to initialize it first by setting env vars
		os.Setenv("SCHEDULER0_AUTH_USERNAME", "user@domain.com")
		os.Setenv("SCHEDULER0_AUTH_PASSWORD", "p@ssw0rd!123")
		secretsData = testSecrets.GetSecrets()
	}

	// Update the cached secrets with our test credentials
	secretsData.AuthUsername = "user@domain.com"
	secretsData.AuthPassword = "p@ssw0rd!123"
	testSecrets.SaveSecrets(secretsData)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.SetBasicAuth("user@domain.com", "p@ssw0rd!123")
	result := IsAuthorizedPeerClient(req, testSecrets)
	assert.True(t, result, "Special characters in credentials should work")
}

func TestIsAuthorizedServerClient_EdgeCase_EmptyAccountID(t *testing.T) {
	mockCredentialService := mocks.NewMockCredentialService(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set(headers.APIKeyHeader, "test-api-key")
	req.Header.Set(headers.SecretKeyHeader, "test-api-secret")
	// No account ID header

	result, _, err := IsAuthorizedServerClient(req, mockCredentialService)

	assert.False(t, result, "Should return false for empty account ID")
	assert.NotNil(t, err, "Should return error for empty account ID")
}

func TestIsAuthorizedServerClient_EdgeCase_InvalidAccountIDFormat(t *testing.T) {
	mockCredentialService := mocks.NewMockCredentialService(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set(headers.APIKeyHeader, "test-api-key")
	req.Header.Set(headers.SecretKeyHeader, "test-api-secret")
	req.Header.Set(headers.AccountIDHeader, "not-a-number")

	result, _, err := IsAuthorizedServerClient(req, mockCredentialService)

	assert.False(t, result, "Should return false for invalid account ID format")
	assert.NotNil(t, err, "Should return error for invalid account ID format")
}

func TestIsAuthorizedServerClient_EdgeCase_ZeroAccountID(t *testing.T) {
	mockCredentialService := mocks.NewMockCredentialService(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set(headers.APIKeyHeader, "test-api-key")
	req.Header.Set(headers.SecretKeyHeader, "test-api-secret")
	req.Header.Set(headers.AccountIDHeader, "0")

	// Zero account ID should parse successfully but validation might fail
	// Let's see what the actual behavior is - it should parse to 0
	accountIDUint, parseErr := strconv.ParseUint("0", 10, 64)
	assert.NoError(t, parseErr, "Zero should parse successfully")
	assert.Equal(t, uint64(0), accountIDUint, "Zero should parse to 0")

	mockCredentialService.On("ValidateServerAPIKey", "test-api-key", "test-api-secret", uint64(0)).
		Return(false, nil, nil)

	result, _, err := IsAuthorizedServerClient(req, mockCredentialService)

	// Zero account ID should parse but validation might fail
	assert.False(t, result, "Should return false for zero account ID")
	assert.Nil(t, err, "Should not return error for zero account ID (parses successfully)")
	mockCredentialService.AssertExpectations(t)
}

func TestContextMiddleware_EdgeCase_ContextPreservation(t *testing.T) {
	handler, _, _, _ := setupMiddlewareHandler(t)

	originalValue := "original-value"
	originalKey := "original-key"

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that original context value is preserved
		originalVal := r.Context().Value(originalKey)
		assert.Equal(t, originalValue, originalVal, "Original context value should be preserved")

		// Check that request ID is also set
		requestID := r.Context().Value(utils.RequestIDContextKey())
		assert.NotNil(t, requestID, "Request ID should be set in context")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	ctx := context.WithValue(req.Context(), originalKey, originalValue)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	middleware := handler.ContextMiddleware(nextHandler)
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Status code should be OK")
}
