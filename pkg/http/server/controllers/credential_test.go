package controllers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"scheduler0/pkg/http/server/controllers"
	"scheduler0/pkg/mocks"
	"scheduler0/pkg/models"
	"scheduler0/pkg/utils"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupCredentialController(t *testing.T) (controllers.CredentialHTTPController, *mocks.MockCredentialService) {
	logger := log.New(&bytes.Buffer{}, "", 0)
	mockService := mocks.NewMockCredentialService(t)
	controller := controllers.NewCredentialController(logger, mockService)
	return controller, mockService
}

func TestCredentialController_CreateOneCredential(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)
		credentialID := uint64(100)

		credential := models.Credential{
			CreatedBy: "test-user",
		}

		createdCredential := models.Credential{
			ID:        credentialID,
			CreatedBy: credential.CreatedBy,
			AccountId: accountID,
		}

		mockService.On("CreateNewCredential", mock.MatchedBy(func(c models.Credential) bool {
			return c.CreatedBy == credential.CreatedBy && c.AccountId == accountID
		})).Return(credentialID, "plaintext-secret", nil)

		mockService.On("FindOneCredentialByID", credentialID, accountID).Return(&createdCredential, nil)

		body, _ := json.Marshal(credential)
		req := httptest.NewRequest(http.MethodPost, "/credentials", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.CreateOneCredential(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("non-admin caller cannot grant admin scope", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		credential := models.Credential{
			CreatedBy: "test-user",
			Scopes:    []string{"admin"},
		}

		// Caller holds only read/write in context => must not be able to self-grant admin.
		callerCred := &models.Credential{AccountId: accountID, Scopes: []string{"read", "write"}}
		ctx := context.WithValue(createContextWithAccountID(accountID), utils.CredentialContextKey(), callerCred)

		body, _ := json.Marshal(credential)
		req := httptest.NewRequest(http.MethodPost, "/credentials", bytes.NewBuffer(body))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		controller.CreateOneCredential(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		mockService.AssertNotCalled(t, "CreateNewCredential")
	})

	t.Run("admin caller can grant admin scope", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)
		credentialID := uint64(101)

		credential := models.Credential{
			CreatedBy: "test-user",
			Scopes:    []string{"admin"},
		}
		created := models.Credential{ID: credentialID, CreatedBy: credential.CreatedBy, AccountId: accountID, Scopes: []string{"admin"}}

		mockService.On("CreateNewCredential", mock.Anything).Return(credentialID, "plaintext-secret", nil)
		mockService.On("FindOneCredentialByID", credentialID, accountID).Return(&created, nil)

		callerCred := &models.Credential{AccountId: accountID, Scopes: []string{"admin"}}
		ctx := context.WithValue(createContextWithAccountID(accountID), utils.CredentialContextKey(), callerCred)

		body, _ := json.Marshal(credential)
		req := httptest.NewRequest(http.MethodPost, "/credentials", bytes.NewBuffer(body))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		controller.CreateOneCredential(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("peer caller (no credential in context) can grant admin scope", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)
		credentialID := uint64(102)

		credential := models.Credential{
			CreatedBy: "operator",
			Scopes:    []string{"admin"},
		}
		created := models.Credential{ID: credentialID, CreatedBy: credential.CreatedBy, AccountId: accountID, Scopes: []string{"admin"}}

		mockService.On("CreateNewCredential", mock.Anything).Return(credentialID, "plaintext-secret", nil)
		mockService.On("FindOneCredentialByID", credentialID, accountID).Return(&created, nil)

		// No credential in context => peer/operator (basic auth) path.
		body, _ := json.Marshal(credential)
		req := httptest.NewRequest(http.MethodPost, "/credentials", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.CreateOneCredential(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing createdBy", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		credential := models.Credential{}

		body, _ := json.Marshal(credential)
		req := httptest.NewRequest(http.MethodPost, "/credentials", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.CreateOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "CreateNewCredential")
	})

	t.Run("empty body", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodPost, "/credentials", bytes.NewBuffer([]byte{}))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.CreateOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "CreateNewCredential")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodPost, "/credentials", bytes.NewBuffer([]byte("invalid json")))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.CreateOneCredential(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		mockService.AssertNotCalled(t, "CreateNewCredential")
	})

	t.Run("service create error", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		credential := models.Credential{
			CreatedBy: "test-user",
		}

		genericError := &utils.GenericError{
			Message: "failed to create credential",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("CreateNewCredential", mock.Anything).Return(uint64(0), "", genericError)

		body, _ := json.Marshal(credential)
		req := httptest.NewRequest(http.MethodPost, "/credentials", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.CreateOneCredential(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("service get error after create", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)
		credentialID := uint64(100)

		credential := models.Credential{
			CreatedBy: "test-user",
		}

		genericError := &utils.GenericError{
			Message: "failed to get credential",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("CreateNewCredential", mock.Anything).Return(credentialID, "plaintext-secret", nil)
		mockService.On("FindOneCredentialByID", credentialID, accountID).Return(nil, genericError)

		body, _ := json.Marshal(credential)
		req := httptest.NewRequest(http.MethodPost, "/credentials", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.CreateOneCredential(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)

		credential := models.Credential{
			CreatedBy: "test-user",
		}

		body, _ := json.Marshal(credential)
		req := httptest.NewRequest(http.MethodPost, "/credentials", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		controller.CreateOneCredential(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "CreateNewCredential")
	})
}

func TestCredentialController_GetOneCredential(t *testing.T) {
	t.Run("successful retrieval", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)
		credentialID := uint64(100)

		credential := &models.Credential{
			ID:        credentialID,
			AccountId: accountID,
		}

		mockService.On("FindOneCredentialByID", credentialID, accountID).Return(credential, nil)

		req := httptest.NewRequest(http.MethodGet, "/credentials/100", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.GetOneCredential(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid credential ID", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodGet, "/credentials/invalid", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		controller.GetOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "FindOneCredentialByID")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)
		credentialID := uint64(100)

		genericError := &utils.GenericError{
			Message: "credential not found",
			Type:    http.StatusNotFound,
		}

		mockService.On("FindOneCredentialByID", credentialID, accountID).Return(nil, genericError)

		req := httptest.NewRequest(http.MethodGet, "/credentials/100", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.GetOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)

		req := httptest.NewRequest(http.MethodGet, "/credentials/100", nil)
		req = req.WithContext(createContextWithRequestID())
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.GetOneCredential(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "FindOneCredentialByID")
	})
}

func TestCredentialController_UpdateOneCredential(t *testing.T) {
	t.Run("successful update", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)
		credentialID := uint64(100)
		modifiedBy := "test-user-updated"

		credential := models.Credential{
			ID:         credentialID,
			ModifiedBy: &modifiedBy,
		}

		updatedCredential := models.Credential{
			ID:         credentialID,
			ModifiedBy: &modifiedBy,
			AccountId:  accountID,
		}

		mockService.On("UpdateOneCredential", mock.MatchedBy(func(c models.Credential) bool {
			return c.ID == credentialID && c.AccountId == accountID && c.ModifiedBy != nil && *c.ModifiedBy == modifiedBy
		})).Return(&updatedCredential, nil)

		body, _ := json.Marshal(credential)
		req := httptest.NewRequest(http.MethodPut, "/credentials/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.UpdateOneCredential(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing modifiedBy", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		credential := models.Credential{
			ID: 100,
		}

		body, _ := json.Marshal(credential)
		req := httptest.NewRequest(http.MethodPut, "/credentials/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.UpdateOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "UpdateOneCredential")
	})

	t.Run("empty body", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodPut, "/credentials/100", bytes.NewBuffer([]byte{}))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.UpdateOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "UpdateOneCredential")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodPut, "/credentials/100", bytes.NewBuffer([]byte("invalid json")))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.UpdateOneCredential(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		mockService.AssertNotCalled(t, "UpdateOneCredential")
	})

	t.Run("invalid credential ID", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		credential := models.Credential{
			ModifiedBy: stringPtr("test-user"),
		}

		body, _ := json.Marshal(credential)
		req := httptest.NewRequest(http.MethodPut, "/credentials/invalid", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		controller.UpdateOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "UpdateOneCredential")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)
		credentialID := uint64(100)
		modifiedBy := "test-user"

		credential := models.Credential{
			ID:         credentialID,
			ModifiedBy: &modifiedBy,
		}

		genericError := &utils.GenericError{
			Message: "failed to update credential",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("UpdateOneCredential", mock.Anything).Return(nil, genericError)

		body, _ := json.Marshal(credential)
		req := httptest.NewRequest(http.MethodPut, "/credentials/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.UpdateOneCredential(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		credentialID := uint64(100)
		modifiedBy := "test-user"

		credential := models.Credential{
			ID:         credentialID,
			ModifiedBy: &modifiedBy,
		}

		body, _ := json.Marshal(credential)
		req := httptest.NewRequest(http.MethodPut, "/credentials/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithRequestID())
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.UpdateOneCredential(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "UpdateOneCredential")
	})
}

func TestCredentialController_DeleteOneCredential(t *testing.T) {
	t.Run("successful deletion", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)
		credentialID := uint64(100)
		deletedBy := "test-user"

		deleteRequest := map[string]string{
			"deletedBy": deletedBy,
		}

		deletedCredential := &models.Credential{
			ID:        credentialID,
			AccountId: accountID,
		}

		mockService.On("DeleteOneCredential", credentialID, accountID, deletedBy).Return(deletedCredential, nil)

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/credentials/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.DeleteOneCredential(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing deletedBy", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		deleteRequest := map[string]string{}

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/credentials/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.DeleteOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "DeleteOneCredential")
	})

	t.Run("invalid credential ID", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		deleteRequest := map[string]string{
			"deletedBy": "test-user",
		}

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/credentials/invalid", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		controller.DeleteOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "DeleteOneCredential")
	})

	t.Run("invalid request body", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodDelete, "/credentials/100", bytes.NewBuffer([]byte("invalid json")))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.DeleteOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "DeleteOneCredential")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)
		credentialID := uint64(100)
		deletedBy := "test-user"

		deleteRequest := map[string]string{
			"deletedBy": deletedBy,
		}

		genericError := &utils.GenericError{
			Message: "credential not found",
			Type:    http.StatusNotFound,
		}

		mockService.On("DeleteOneCredential", credentialID, accountID, deletedBy).Return(nil, genericError)

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/credentials/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.DeleteOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		deletedBy := "test-user"

		deleteRequest := map[string]string{
			"deletedBy": deletedBy,
		}

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/credentials/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithRequestID())
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.DeleteOneCredential(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "DeleteOneCredential")
	})
}

func TestCredentialController_ArchiveOneCredential(t *testing.T) {
	t.Run("successful archive", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)
		credentialID := uint64(100)
		archivedBy := "test-user"

		archiveRequest := map[string]string{
			"archivedBy": archivedBy,
		}

		archivedCredential := &models.Credential{
			ID:        credentialID,
			AccountId: accountID,
		}

		mockService.On("ArchiveOneCredential", credentialID, accountID, archivedBy).Return(archivedCredential, nil)

		body, _ := json.Marshal(archiveRequest)
		req := httptest.NewRequest(http.MethodPost, "/credentials/100/archive", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.ArchiveOneCredential(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing archivedBy", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		archiveRequest := map[string]string{}

		body, _ := json.Marshal(archiveRequest)
		req := httptest.NewRequest(http.MethodPost, "/credentials/100/archive", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.ArchiveOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "ArchiveOneCredential")
	})

	t.Run("invalid credential ID", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		archiveRequest := map[string]string{
			"archivedBy": "test-user",
		}

		body, _ := json.Marshal(archiveRequest)
		req := httptest.NewRequest(http.MethodPost, "/credentials/invalid/archive", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		controller.ArchiveOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "ArchiveOneCredential")
	})

	t.Run("invalid request body", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodPost, "/credentials/100/archive", bytes.NewBuffer([]byte("invalid json")))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.ArchiveOneCredential(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "ArchiveOneCredential")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)
		credentialID := uint64(100)
		archivedBy := "test-user"

		archiveRequest := map[string]string{
			"archivedBy": archivedBy,
		}

		genericError := &utils.GenericError{
			Message: "credential not found",
			Type:    http.StatusNotFound,
		}

		mockService.On("ArchiveOneCredential", credentialID, accountID, archivedBy).Return(nil, genericError)

		body, _ := json.Marshal(archiveRequest)
		req := httptest.NewRequest(http.MethodPost, "/credentials/100/archive", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.ArchiveOneCredential(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		archivedBy := "test-user"

		archiveRequest := map[string]string{
			"archivedBy": archivedBy,
		}

		body, _ := json.Marshal(archiveRequest)
		req := httptest.NewRequest(http.MethodPost, "/credentials/100/archive", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithRequestID())
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.ArchiveOneCredential(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "ArchiveOneCredential")
	})
}

func TestCredentialController_ListCredentials(t *testing.T) {
	t.Run("successful list", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		credentials := &models.PaginatedCredential{
			Total:  2,
			Offset: 0,
			Limit:  10,
			Data: []models.Credential{
				{ID: 1, AccountId: accountID},
				{ID: 2, AccountId: accountID},
			},
		}

		mockService.On("ListCredentials", uint64(0), uint64(10), "date_created", "DESC", accountID).Return(credentials, nil)

		req := httptest.NewRequest(http.MethodGet, "/credentials?limit=10&offset=0", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.ListCredentials(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("with custom orderBy", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		credentials := &models.PaginatedCredential{
			Total:  0,
			Offset: 0,
			Limit:  10,
			Data:   []models.Credential{},
		}

		mockService.On("ListCredentials", uint64(0), uint64(10), "date_modified", "ASC", accountID).Return(credentials, nil)

		req := httptest.NewRequest(http.MethodGet, "/credentials?limit=10&offset=0&orderBy=date_modified&orderByDirection=ASC", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.ListCredentials(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid offset", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodGet, "/credentials?limit=10&offset=invalid", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.ListCredentials(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "ListCredentials")
	})

	t.Run("invalid limit", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodGet, "/credentials?limit=invalid&offset=0", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.ListCredentials(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "ListCredentials")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)
		accountID := uint64(1)

		genericError := &utils.GenericError{
			Message: "failed to list credentials",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("ListCredentials", uint64(0), uint64(10), "date_created", "DESC", accountID).Return(nil, genericError)

		req := httptest.NewRequest(http.MethodGet, "/credentials?limit=10&offset=0", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.ListCredentials(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupCredentialController(t)

		req := httptest.NewRequest(http.MethodGet, "/credentials?limit=10&offset=0", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		controller.ListCredentials(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "ListCredentials")
	})
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
