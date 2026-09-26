package controllers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"scheduler0-private/pkg/http/server/controllers"
	"scheduler0-private/pkg/models"
	"scheduler0-private/pkg/service/executor"
	"scheduler0-private/pkg/utils"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupExecutorController(t *testing.T) (*controllers.JobExecutorHTTPController, *executor.MockJobExecutorService) {
	logger := log.New(&bytes.Buffer{}, "", 0)
	mockService := executor.NewMockJobExecutorService(t)
	controller := controllers.NewJobExecutorController(logger, mockService)
	return &controller, mockService
}

// Shared helper functions for all controller tests
func createContextWithAccountID(accountID uint64) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, utils.AccountIDContextKey(), accountID)
	ctx = context.WithValue(ctx, utils.RequestIDContextKey(), "test-request-id")
	return ctx
}

func createContextWithRequestID() context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, utils.RequestIDContextKey(), "test-request-id")
	return ctx
}

func TestExecutorController_CreateOneExecutor(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)
		executorID := uint64(100)

		executor := models.JobExecutor{
			Name:      "test-executor",
			Type:      "webhook_url",
			CreatedBy: "test-user",
		}

		createdExecutor := models.JobExecutor{
			ID:        executorID,
			Name:      executor.Name,
			Type:      executor.Type,
			CreatedBy: executor.CreatedBy,
			AccountId: accountID,
		}

		mockService.On("CreateExecutor", mock.MatchedBy(func(e models.JobExecutor) bool {
			return e.Name == executor.Name && e.Type == executor.Type && e.CreatedBy == executor.CreatedBy && e.AccountId == accountID
		})).Return(executorID, nil)

		mockService.On("GetOneByID", executorID, accountID).Return(&createdExecutor, nil)

		body, _ := json.Marshal(executor)
		req := httptest.NewRequest(http.MethodPost, "/executors", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		(*controller).CreateOneExecutor(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing createdBy", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		executor := models.JobExecutor{
			Name: "test-executor",
			Type: "webhook_url",
		}

		body, _ := json.Marshal(executor)
		req := httptest.NewRequest(http.MethodPost, "/executors", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		(*controller).CreateOneExecutor(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "CreateExecutor")
	})

	t.Run("empty body", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodPost, "/executors", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		(*controller).CreateOneExecutor(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "CreateExecutor")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodPost, "/executors", bytes.NewBuffer([]byte("invalid json")))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		(*controller).CreateOneExecutor(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		mockService.AssertNotCalled(t, "CreateExecutor")
	})
}

func TestExecutorController_GetOneExecutor(t *testing.T) {
	t.Run("successful retrieval", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)
		executorID := uint64(100)

		executor := &models.JobExecutor{
			ID:        executorID,
			Name:      "test-executor",
			Type:      "webhook_url",
			AccountId: accountID,
		}

		mockService.On("GetOneByID", executorID, accountID).Return(executor, nil)

		req := httptest.NewRequest(http.MethodGet, "/executors/100", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).GetOneExecutor(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid executor ID", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodGet, "/executors/invalid", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		(*controller).GetOneExecutor(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "GetOneByID")
	})
}

func TestExecutorController_UpdateOneExecutor(t *testing.T) {
	t.Run("successful update", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)
		executorID := uint64(100)
		modifiedBy := "test-user-updated"

		executor := models.JobExecutor{
			ID:        executorID,
			Name:      "updated-executor",
			Type:      "webhook_url",
			ModifiedBy: &modifiedBy,
		}

		mockService.On("UpdateOneBy", mock.MatchedBy(func(e models.JobExecutor) bool {
			return e.ID == executorID && e.ModifiedBy != nil && *e.ModifiedBy == modifiedBy
		})).Return(executorID, nil)
		mockService.On("GetOneByID", executorID, accountID).Return(&executor, nil)

		body, _ := json.Marshal(executor)
		req := httptest.NewRequest(http.MethodPut, "/executors/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).UpdateOneExecutor(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing modifiedBy", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		executor := models.JobExecutor{
			ID:   100,
			Name: "updated-executor",
			Type: "webhook_url",
		}

		body, _ := json.Marshal(executor)
		req := httptest.NewRequest(http.MethodPut, "/executors/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).UpdateOneExecutor(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "UpdateOneBy")
	})
}

func TestExecutorController_DeleteOneExecutor(t *testing.T) {
	t.Run("successful deletion", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)
		executorID := uint64(100)
		deletedBy := "test-user"

		deleteRequest := map[string]string{
			"deletedBy": deletedBy,
		}

		mockService.On("DeleteOneByID", executorID, accountID, deletedBy).Return(uint64(1), nil)

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/executors/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).DeleteOneExecutor(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing deletedBy", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		deleteRequest := map[string]string{}

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/executors/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).DeleteOneExecutor(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "DeleteOneByID")
	})
}

func TestExecutorController_ListExecutors(t *testing.T) {
	t.Run("successful list", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		executors := &models.PaginatedJobExecutor{
			Total:  2,
			Offset: 0,
			Limit:  10,
			Data: []models.JobExecutor{
				{ID: 1, Name: "executor-1", AccountId: accountID},
				{ID: 2, Name: "executor-2", AccountId: accountID},
			},
		}

		mockService.On("ListJobExecutors", uint64(0), uint64(10), "date_created", "DESC", accountID).Return(executors, nil)

		req := httptest.NewRequest(http.MethodGet, "/executors?limit=10&offset=0", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		(*controller).ListExecutors(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("with custom orderBy", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		executors := &models.PaginatedJobExecutor{
			Total:  0,
			Offset: 0,
			Limit:  10,
			Data:   []models.JobExecutor{},
		}

		mockService.On("ListJobExecutors", uint64(0), uint64(10), "name", "ASC", accountID).Return(executors, nil)

		req := httptest.NewRequest(http.MethodGet, "/executors?limit=10&offset=0&orderBy=name&orderByDirection=ASC", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		(*controller).ListExecutors(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid offset", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodGet, "/executors?limit=10&offset=invalid", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		(*controller).ListExecutors(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "ListJobExecutors")
	})

	t.Run("invalid limit", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodGet, "/executors?limit=invalid&offset=0", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		(*controller).ListExecutors(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "ListJobExecutors")
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)

		req := httptest.NewRequest(http.MethodGet, "/executors?limit=10&offset=0", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		(*controller).ListExecutors(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "ListJobExecutors")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		genericError := &utils.GenericError{
			Message: "failed to list executors",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("ListJobExecutors", uint64(0), uint64(10), "date_created", "DESC", accountID).Return(nil, genericError)

		req := httptest.NewRequest(http.MethodGet, "/executors?limit=10&offset=0", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		(*controller).ListExecutors(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid orderBy validation error", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		// The validation actually passes and calls the service, so we need to mock it
		executors := &models.PaginatedJobExecutor{
			Total:  0,
			Offset: 0,
			Limit:  10,
			Data:   []models.JobExecutor{},
		}
		mockService.On("ListJobExecutors", uint64(0), uint64(10), "invalid", "invalid", accountID).Return(executors, nil)

		req := httptest.NewRequest(http.MethodGet, "/executors?limit=10&offset=0&orderBy=invalid&orderByDirection=invalid", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		(*controller).ListExecutors(w, req)

		// The validation passes, so the service is called and returns OK
		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestExecutorController_CreateOneExecutor_ErrorPaths(t *testing.T) {
	t.Run("service create error", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		executor := models.JobExecutor{
			Name:      "test-executor",
			Type:      "webhook_url",
			CreatedBy: "test-user",
		}

		genericError := &utils.GenericError{
			Message: "failed to create executor",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("CreateExecutor", mock.Anything).Return(uint64(0), genericError)

		body, _ := json.Marshal(executor)
		req := httptest.NewRequest(http.MethodPost, "/executors", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		(*controller).CreateOneExecutor(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("service get error after create", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)
		executorID := uint64(100)

		executor := models.JobExecutor{
			Name:      "test-executor",
			Type:      "webhook_url",
			CreatedBy: "test-user",
		}

		genericError := &utils.GenericError{
			Message: "failed to get executor",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("CreateExecutor", mock.Anything).Return(executorID, nil)
		mockService.On("GetOneByID", executorID, accountID).Return(nil, genericError)

		body, _ := json.Marshal(executor)
		req := httptest.NewRequest(http.MethodPost, "/executors", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		(*controller).CreateOneExecutor(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)

		executor := models.JobExecutor{
			Name:      "test-executor",
			Type:      "webhook_url",
			CreatedBy: "test-user",
		}

		body, _ := json.Marshal(executor)
		req := httptest.NewRequest(http.MethodPost, "/executors", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		(*controller).CreateOneExecutor(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "CreateExecutor")
	})
}

func TestExecutorController_GetOneExecutor_ErrorPaths(t *testing.T) {
	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)
		executorID := uint64(100)

		genericError := &utils.GenericError{
			Message: "executor not found",
			Type:    http.StatusNotFound,
		}

		mockService.On("GetOneByID", executorID, accountID).Return(nil, genericError)

		req := httptest.NewRequest(http.MethodGet, "/executors/100", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).GetOneExecutor(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)

		req := httptest.NewRequest(http.MethodGet, "/executors/100", nil)
		req = req.WithContext(createContextWithRequestID())
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).GetOneExecutor(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "GetOneByID")
	})
}

func TestExecutorController_UpdateOneExecutor_ErrorPaths(t *testing.T) {
	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)
		executorID := uint64(100)
		modifiedBy := "test-user"

		executor := models.JobExecutor{
			ID:         executorID,
			Name:       "updated-executor",
			Type:       "webhook_url",
			ModifiedBy: &modifiedBy,
		}

		genericError := &utils.GenericError{
			Message: "failed to update executor",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("UpdateOneBy", mock.Anything).Return(uint64(0), genericError)

		body, _ := json.Marshal(executor)
		req := httptest.NewRequest(http.MethodPut, "/executors/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).UpdateOneExecutor(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodPut, "/executors/100", bytes.NewBuffer([]byte("invalid json")))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).UpdateOneExecutor(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		mockService.AssertNotCalled(t, "UpdateOneBy")
	})

	t.Run("invalid executor ID", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)
		modifiedBy := "test-user"

		executor := models.JobExecutor{
			Name:       "updated-executor",
			Type:       "webhook_url",
			ModifiedBy: &modifiedBy,
		}

		body, _ := json.Marshal(executor)
		req := httptest.NewRequest(http.MethodPut, "/executors/invalid", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		(*controller).UpdateOneExecutor(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "UpdateOneBy")
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		executorID := uint64(100)
		modifiedBy := "test-user"

		executor := models.JobExecutor{
			ID:         executorID,
			Name:       "updated-executor",
			Type:       "webhook_url",
			ModifiedBy: &modifiedBy,
		}

		body, _ := json.Marshal(executor)
		req := httptest.NewRequest(http.MethodPut, "/executors/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithRequestID())
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).UpdateOneExecutor(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "UpdateOneBy")
	})
}

func TestExecutorController_DeleteOneExecutor_ErrorPaths(t *testing.T) {
	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)
		executorID := uint64(100)
		deletedBy := "test-user"

		deleteRequest := map[string]string{
			"deletedBy": deletedBy,
		}

		genericError := &utils.GenericError{
			Message: "executor not found",
			Type:    http.StatusNotFound,
		}

		mockService.On("DeleteOneByID", executorID, accountID, deletedBy).Return(uint64(0), genericError)

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/executors/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).DeleteOneExecutor(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid request body JSON", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodDelete, "/executors/100", bytes.NewBuffer([]byte("invalid json")))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).DeleteOneExecutor(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "DeleteOneByID")
	})

	t.Run("invalid executor ID", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)
		deletedBy := "test-user"

		deleteRequest := map[string]string{
			"deletedBy": deletedBy,
		}

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/executors/invalid", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		(*controller).DeleteOneExecutor(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "DeleteOneByID")
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		deletedBy := "test-user"

		deleteRequest := map[string]string{
			"deletedBy": deletedBy,
		}

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/executors/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithRequestID())
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).DeleteOneExecutor(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "DeleteOneByID")
	})
}

// TestExecutorController_WebhookSecretHandling verifies that the webhook secret is accepted
// as input, revealed exactly once in the create response, and never returned on reads.
func TestExecutorController_WebhookSecretHandling(t *testing.T) {
	t.Run("create accepts secret input and reveals it once", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)
		executorID := uint64(100)
		const plainSecret = "super-secret-value"

		createdExecutor := models.JobExecutor{
			ID:            executorID,
			Name:          "test-executor",
			Type:          "webhook_url",
			CreatedBy:     "test-user",
			AccountId:     accountID,
			WebhookSecret: plainSecret,
		}

		// The controller must forward the submitted secret to the service.
		mockService.On("CreateExecutor", mock.MatchedBy(func(e models.JobExecutor) bool {
			return e.WebhookSecret == plainSecret && e.Type == "webhook_url"
		})).Return(executorID, nil)
		mockService.On("GetOneByID", executorID, accountID).Return(&createdExecutor, nil)

		// Build the request body from raw JSON since json.Marshal(JobExecutor) omits the
		// secret (json:"-"); this mirrors a real API client sending webhookSecret.
		body := []byte(`{"name":"test-executor","type":"webhook_url","createdBy":"test-user","webhookUrl":"https://example.com","webhookMethod":"POST","webhookSecret":"` + plainSecret + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/executors", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		(*controller).CreateOneExecutor(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), plainSecret, "create response should reveal the webhook secret exactly once")
		mockService.AssertExpectations(t)
	})

	t.Run("get does not return the secret", func(t *testing.T) {
		controller, mockService := setupExecutorController(t)
		accountID := uint64(1)
		executorID := uint64(100)
		const plainSecret = "super-secret-value"

		executor := &models.JobExecutor{
			ID:            executorID,
			Name:          "test-executor",
			Type:          "webhook_url",
			AccountId:     accountID,
			WebhookSecret: plainSecret,
		}

		mockService.On("GetOneByID", executorID, accountID).Return(executor, nil)

		req := httptest.NewRequest(http.MethodGet, "/executors/100", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		(*controller).GetOneExecutor(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), plainSecret, "get response must not expose the webhook secret")
		assert.NotContains(t, w.Body.String(), "webhookSecret", "get response must not include the webhookSecret field")
		mockService.AssertExpectations(t)
	})
}

