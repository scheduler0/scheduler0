package controllers_test

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"scheduler0/pkg/http/server/controllers"
	"scheduler0/pkg/models"
	"scheduler0/pkg/mocks"
	"scheduler0/pkg/utils"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupAccountController(t *testing.T) (interface {
	CreateOneAccount(w http.ResponseWriter, r *http.Request)
	GetOneAccount(w http.ResponseWriter, r *http.Request)
	AddFeature(w http.ResponseWriter, r *http.Request)
	RemoveFeature(w http.ResponseWriter, r *http.Request)
	AddAllFeatures(w http.ResponseWriter, r *http.Request)
	RemoveAllFeatures(w http.ResponseWriter, r *http.Request)
}, *mocks.MockAccountService) {
	logger := log.New(&bytes.Buffer{}, "", 0)
	mockService := mocks.NewMockAccountService(t)
	controller := controllers.NewAccountController(logger, mockService)
	return controller, mockService
}

func TestAccountController_CreateOneAccount(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)

		account := &models.Account{
			Name: "test-account",
		}

		createdAccount := &models.Account{
			ID:   accountID,
			Name: account.Name,
		}

		features := &[]models.AccountFeature{}

		mockService.On("CreateAccount", mock.AnythingOfType("*models.Account")).Return(accountID, nil)
		mockService.On("GetAccount", accountID).Return(createdAccount, nil)
		mockService.On("GetFeatures", accountID).Return(features, nil)

		body, _ := json.Marshal(account)
		req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		controller.CreateOneAccount(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		controller, mockService := setupAccountController(t)

		req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBuffer([]byte("invalid json")))
		w := httptest.NewRecorder()

		controller.CreateOneAccount(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "CreateAccount")
	})
}

func TestAccountController_GetOneAccount(t *testing.T) {
	t.Run("successful retrieval", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)

		account := &models.Account{
			ID:   accountID,
			Name: "test-account",
		}

		features := &[]models.AccountFeature{}

		mockService.On("GetAccount", accountID).Return(account, nil)
		mockService.On("GetFeatures", accountID).Return(features, nil)

		req := httptest.NewRequest(http.MethodGet, "/accounts/100", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.GetOneAccount(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid account ID", func(t *testing.T) {
		controller, mockService := setupAccountController(t)

		req := httptest.NewRequest(http.MethodGet, "/accounts/invalid", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		controller.GetOneAccount(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "GetAccount")
	})
}

func TestAccountController_AddFeature(t *testing.T) {
	t.Run("successful add feature", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)
		featureID := uint64(1)

		featureRequest := models.FeatureRequest{
			FeatureId: featureID,
		}

		mockService.On("AddFeature", accountID, featureID).Return(nil)

		body, _ := json.Marshal(featureRequest)
		req := httptest.NewRequest(http.MethodPut, "/accounts/100/feature", bytes.NewBuffer(body))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.AddFeature(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid account ID", func(t *testing.T) {
		controller, mockService := setupAccountController(t)

		featureRequest := models.FeatureRequest{
			FeatureId: 1,
		}

		body, _ := json.Marshal(featureRequest)
		req := httptest.NewRequest(http.MethodPut, "/accounts/invalid/feature", bytes.NewBuffer(body))
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		controller.AddFeature(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "AddFeature")
	})
}

func TestAccountController_RemoveFeature(t *testing.T) {
	t.Run("successful remove feature", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)
		featureID := uint64(1)

		featureRequest := models.FeatureRequest{
			FeatureId: featureID,
		}

		mockService.On("RemoveFeature", accountID, featureID).Return(nil)

		body, _ := json.Marshal(featureRequest)
		req := httptest.NewRequest(http.MethodDelete, "/accounts/100/feature", bytes.NewBuffer(body))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.RemoveFeature(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestAccountController_AddAllFeatures(t *testing.T) {
	t.Run("successful add all features", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)

		mockService.On("AddAllFeatures", accountID).Return(nil)

		req := httptest.NewRequest(http.MethodPut, "/accounts/100/features/all", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.AddAllFeatures(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestAccountController_RemoveAllFeatures(t *testing.T) {
	t.Run("successful remove all features", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)

		mockService.On("RemoveAllFeatures", accountID).Return(nil)

		req := httptest.NewRequest(http.MethodDelete, "/accounts/100/features/all", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.RemoveAllFeatures(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestAccountController_CreateOneAccount_ErrorPaths(t *testing.T) {
	t.Run("service create error", func(t *testing.T) {
		controller, mockService := setupAccountController(t)

		account := &models.Account{
			Name: "test-account",
		}

		genericError := &utils.GenericError{
			Message: "failed to create account",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("CreateAccount", mock.Anything).Return(uint64(0), genericError)

		body, _ := json.Marshal(account)
		req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		controller.CreateOneAccount(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("service get error after create", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)

		account := &models.Account{
			Name: "test-account",
		}

		genericError := &utils.GenericError{
			Message: "failed to get account",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("CreateAccount", mock.Anything).Return(accountID, nil)
		mockService.On("GetAccount", accountID).Return(nil, genericError)

		body, _ := json.Marshal(account)
		req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		controller.CreateOneAccount(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("service get features error", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)

		account := &models.Account{
			Name: "test-account",
		}

		createdAccount := &models.Account{
			ID:   accountID,
			Name: account.Name,
		}

		genericError := &utils.GenericError{
			Message: "failed to get features",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("CreateAccount", mock.Anything).Return(accountID, nil)
		mockService.On("GetAccount", accountID).Return(createdAccount, nil)
		mockService.On("GetFeatures", accountID).Return(nil, genericError)

		body, _ := json.Marshal(account)
		req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		controller.CreateOneAccount(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestAccountController_GetOneAccount_ErrorPaths(t *testing.T) {
	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)

		genericError := &utils.GenericError{
			Message: "account not found",
			Type:    http.StatusNotFound,
		}

		mockService.On("GetAccount", accountID).Return(nil, genericError)

		req := httptest.NewRequest(http.MethodGet, "/accounts/100", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.GetOneAccount(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("service get features error", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)

		account := &models.Account{
			ID:   accountID,
			Name: "test-account",
		}

		genericError := &utils.GenericError{
			Message: "failed to get features",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("GetAccount", accountID).Return(account, nil)
		mockService.On("GetFeatures", accountID).Return(nil, genericError)

		req := httptest.NewRequest(http.MethodGet, "/accounts/100", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.GetOneAccount(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestAccountController_AddFeature_ErrorPaths(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		controller, mockService := setupAccountController(t)

		req := httptest.NewRequest(http.MethodPut, "/accounts/100/feature", bytes.NewBuffer([]byte("invalid json")))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.AddFeature(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "AddFeature")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)
		featureID := uint64(1)

		featureRequest := models.FeatureRequest{
			FeatureId: featureID,
		}

		genericError := &utils.GenericError{
			Message: "failed to add feature",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("AddFeature", accountID, featureID).Return(genericError)

		body, _ := json.Marshal(featureRequest)
		req := httptest.NewRequest(http.MethodPut, "/accounts/100/feature", bytes.NewBuffer(body))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.AddFeature(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestAccountController_RemoveFeature_ErrorPaths(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		controller, mockService := setupAccountController(t)

		req := httptest.NewRequest(http.MethodDelete, "/accounts/100/feature", bytes.NewBuffer([]byte("invalid json")))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.RemoveFeature(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "RemoveFeature")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)
		featureID := uint64(1)

		featureRequest := models.FeatureRequest{
			FeatureId: featureID,
		}

		genericError := &utils.GenericError{
			Message: "failed to remove feature",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("RemoveFeature", accountID, featureID).Return(genericError)

		body, _ := json.Marshal(featureRequest)
		req := httptest.NewRequest(http.MethodDelete, "/accounts/100/feature", bytes.NewBuffer(body))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.RemoveFeature(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestAccountController_AddAllFeatures_ErrorPaths(t *testing.T) {
	t.Run("invalid account ID", func(t *testing.T) {
		controller, mockService := setupAccountController(t)

		req := httptest.NewRequest(http.MethodPut, "/accounts/invalid/features/all", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		controller.AddAllFeatures(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "AddAllFeatures")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)

		genericError := &utils.GenericError{
			Message: "failed to add all features",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("AddAllFeatures", accountID).Return(genericError)

		req := httptest.NewRequest(http.MethodPut, "/accounts/100/features/all", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.AddAllFeatures(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestAccountController_RemoveAllFeatures_ErrorPaths(t *testing.T) {
	t.Run("invalid account ID", func(t *testing.T) {
		controller, mockService := setupAccountController(t)

		req := httptest.NewRequest(http.MethodDelete, "/accounts/invalid/features/all", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		controller.RemoveAllFeatures(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "RemoveAllFeatures")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupAccountController(t)
		accountID := uint64(100)

		genericError := &utils.GenericError{
			Message: "failed to remove all features",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("RemoveAllFeatures", accountID).Return(genericError)

		req := httptest.NewRequest(http.MethodDelete, "/accounts/100/features/all", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.RemoveAllFeatures(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})
}

