package controllers_test

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"scheduler0/pkg/http/server/controllers"
	"scheduler0/pkg/mocks"
	"scheduler0/pkg/models"
	"scheduler0/pkg/utils"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// accountScopeController is the subset of accountController used by account-scope tests.
type accountScopeController interface {
	GetAIUsage(w http.ResponseWriter, r *http.Request)
	GetTokens(w http.ResponseWriter, r *http.Request)
	GetOneAccount(w http.ResponseWriter, r *http.Request)
}

func setupUsageController(t *testing.T) (accountScopeController, *mocks.MockAccountService) {
	logger := log.New(&bytes.Buffer{}, "", 0)
	mockService := mocks.NewMockAccountService(t)
	return controllers.NewAccountController(logger, mockService), mockService
}

func withContextAccount(r *http.Request, accountID uint64) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), utils.AccountIDContextKey(), accountID))
}

func sampleUsage(accountID uint64) *models.AIUsage {
	return &models.AIUsage{
		AccountId: accountID,
		Prompt:    models.AIUsageDimension{Limit: 1000, Used: 42, Remaining: 958},
		Classify:  models.AIUsageDimension{Limit: 1000, Used: 10, Remaining: 990},
	}
}

// Test_GetAIUsage_AllowsMatchingContextAccount verifies that when the authenticated context
// account matches the path account, the request is served.
func Test_GetAIUsage_AllowsMatchingContextAccount(t *testing.T) {
	controller, mockService := setupUsageController(t)
	mockService.On("GetAIUsage", uint64(100)).Return(sampleUsage(100), nil)

	req := httptest.NewRequest(http.MethodGet, "/accounts/100/ai/usage", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "100"})
	req = withContextAccount(req, 100)
	w := httptest.NewRecorder()

	controller.GetAIUsage(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}

// Test_GetAIUsage_BlocksCrossAccountRead verifies a context account cannot read another
// account's usage; the service is never consulted.
func Test_GetAIUsage_BlocksCrossAccountRead(t *testing.T) {
	controller, mockService := setupUsageController(t)

	req := httptest.NewRequest(http.MethodGet, "/accounts/100/ai/usage", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "100"})
	req = withContextAccount(req, 999)
	w := httptest.NewRecorder()

	controller.GetAIUsage(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockService.AssertNotCalled(t, "GetAIUsage", mock.Anything)
}

// Test_GetAIUsage_AllowsTrustedPeerWithoutContextAccount verifies server-to-server calls that
// carry no account context (e.g. admin/billing) still pass through unchanged.
func Test_GetAIUsage_AllowsTrustedPeerWithoutContextAccount(t *testing.T) {
	controller, mockService := setupUsageController(t)
	mockService.On("GetAIUsage", uint64(100)).Return(sampleUsage(100), nil)

	req := httptest.NewRequest(http.MethodGet, "/accounts/100/ai/usage", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "100"})
	w := httptest.NewRecorder()

	controller.GetAIUsage(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}

// Test_AccountPathHandlers_BlockCrossAccountAccess verifies tenant-scoped API-key callers
// cannot hit another account's /accounts/{id} routes (features/tokens/etc.).
func Test_AccountPathHandlers_BlockCrossAccountAccess(t *testing.T) {
	controller, mockService := setupUsageController(t)

	req := httptest.NewRequest(http.MethodGet, "/accounts/100/tokens", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "100"})
	req = withContextAccount(req, 999)
	w := httptest.NewRecorder()

	controller.GetTokens(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockService.AssertNotCalled(t, "GetTokens", mock.Anything)
}

func Test_GetOneAccount_BlocksCrossAccountRead(t *testing.T) {
	controller, mockService := setupUsageController(t)

	req := httptest.NewRequest(http.MethodGet, "/accounts/100", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "100"})
	req = withContextAccount(req, 999)
	w := httptest.NewRecorder()

	controller.GetOneAccount(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockService.AssertNotCalled(t, "GetAccount", mock.Anything)
}
