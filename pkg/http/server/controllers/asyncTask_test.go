package controllers_test

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"scheduler0-private/pkg/http/server/controllers"
	"scheduler0-private/pkg/models"
	async_task "scheduler0-private/pkg/service/async_task"
	"scheduler0-private/pkg/utils"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func setupAsyncTaskController(t *testing.T) (*controllers.AsyncTaskController, *async_task.MockAsyncTaskService) {
	logger := log.New(&bytes.Buffer{}, "", 0)
	mockService := async_task.NewMockAsyncTaskService(t)
	controller := controllers.NewAsyncTaskController(logger, mockService)
	return &controller, mockService
}

func TestAsyncTaskController_GetTask(t *testing.T) {
	t.Run("task already completed - success", func(t *testing.T) {
		controller, mockService := setupAsyncTaskController(t)
		accountID := uint64(1)
		taskRequestID := "test-request-id-123"

		task := &models.AsyncTask{
			Id:        100,
			RequestId: taskRequestID,
			State:     models.AsyncTaskSuccess,
			AccountId: accountID,
		}

		mockService.On("GetTaskWithRequestIdNonBlocking", taskRequestID, accountID).Return(task, nil)

		req := httptest.NewRequest(http.MethodGet, "/async-tasks/"+taskRequestID, nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": taskRequestID})
		w := httptest.NewRecorder()

		(*controller).GetTask(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
		mockService.AssertNotCalled(t, "GetTaskWithRequestIdBlocking")
	})

	t.Run("task already failed", func(t *testing.T) {
		controller, mockService := setupAsyncTaskController(t)
		accountID := uint64(1)
		taskRequestID := "test-request-id-456"

		task := &models.AsyncTask{
			Id:        200,
			RequestId: taskRequestID,
			State:     models.AsyncTaskFail,
			AccountId: accountID,
		}

		mockService.On("GetTaskWithRequestIdNonBlocking", taskRequestID, accountID).Return(task, nil)

		req := httptest.NewRequest(http.MethodGet, "/async-tasks/"+taskRequestID, nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": taskRequestID})
		w := httptest.NewRecorder()

		(*controller).GetTask(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
		mockService.AssertNotCalled(t, "GetTaskWithRequestIdBlocking")
	})

	t.Run("task in progress - switches to blocking", func(t *testing.T) {
		controller, mockService := setupAsyncTaskController(t)
		accountID := uint64(1)
		taskRequestID := "test-request-id-789"
		subscriberID := uint64(1)

		taskInProgress := &models.AsyncTask{
			Id:        300,
			RequestId: taskRequestID,
			State:     models.AsyncTaskInProgress,
			AccountId: accountID,
		}

		taskCompleted := &models.AsyncTask{
			Id:        300,
			RequestId: taskRequestID,
			State:     models.AsyncTaskSuccess,
			AccountId: accountID,
		}

		taskCh := make(chan models.AsyncTask, 1)
		taskCh <- *taskCompleted

		mockService.On("GetTaskWithRequestIdNonBlocking", taskRequestID, accountID).Return(taskInProgress, nil)
		mockService.On("GetTaskWithRequestIdBlocking", taskRequestID, accountID).Return(taskCh, subscriberID, nil)
		mockService.On("GetTaskIdWithRequestId", taskRequestID, accountID).Return(uint64(300), nil).Maybe()
		mockService.On("DeleteSubscriber", uint64(300), subscriberID).Return(nil).Maybe()

		req := httptest.NewRequest(http.MethodGet, "/async-tasks/"+taskRequestID, nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": taskRequestID})
		w := httptest.NewRecorder()

		// Use a context with timeout to avoid hanging
		ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
		defer cancel()
		req = req.WithContext(ctx)

		(*controller).GetTask(w, req)

		// The test should complete quickly due to the buffered channel
		close(taskCh)
		mockService.AssertExpectations(t)
	})

	t.Run("task not found", func(t *testing.T) {
		controller, mockService := setupAsyncTaskController(t)
		accountID := uint64(1)
		taskRequestID := "non-existent-id"

		genericError := &utils.GenericError{
			Message: "task not found",
			Type:    http.StatusNotFound,
		}

		mockService.On("GetTaskWithRequestIdNonBlocking", taskRequestID, accountID).Return(nil, genericError)

		req := httptest.NewRequest(http.MethodGet, "/async-tasks/"+taskRequestID, nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": taskRequestID})
		w := httptest.NewRecorder()

		(*controller).GetTask(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupAsyncTaskController(t)
		taskRequestID := "test-request-id"

		req := httptest.NewRequest(http.MethodGet, "/async-tasks/"+taskRequestID, nil)
		// Create context without account ID
		ctx := context.Background()
		ctx = context.WithValue(ctx, utils.RequestIDContextKey(), "test-request-id")
		req = req.WithContext(ctx)
		req = mux.SetURLVars(req, map[string]string{"id": taskRequestID})
		w := httptest.NewRecorder()

		(*controller).GetTask(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "GetTaskWithRequestIdNonBlocking")
	})

	t.Run("blocking call error", func(t *testing.T) {
		controller, mockService := setupAsyncTaskController(t)
		accountID := uint64(1)
		taskRequestID := "test-request-id-789"

		taskInProgress := &models.AsyncTask{
			Id:        300,
			RequestId: taskRequestID,
			State:     models.AsyncTaskInProgress,
			AccountId: accountID,
		}

		genericError := &utils.GenericError{
			Message: "failed to get task",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("GetTaskWithRequestIdNonBlocking", taskRequestID, accountID).Return(taskInProgress, nil)
		mockService.On("GetTaskWithRequestIdBlocking", taskRequestID, accountID).Return(nil, uint64(0), genericError)

		req := httptest.NewRequest(http.MethodGet, "/async-tasks/"+taskRequestID, nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": taskRequestID})
		w := httptest.NewRecorder()

		(*controller).GetTask(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("uses account ID from context not hardcoded system account", func(t *testing.T) {
		controller, mockService := setupAsyncTaskController(t)
		accountID := uint64(42)
		taskRequestID := "tenant-request-id"

		task := &models.AsyncTask{
			Id:        500,
			RequestId: taskRequestID,
			State:     models.AsyncTaskSuccess,
			AccountId: accountID,
		}

		mockService.On("GetTaskWithRequestIdNonBlocking", taskRequestID, accountID).Return(task, nil)

		req := httptest.NewRequest(http.MethodGet, "/async-tasks/"+taskRequestID, nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": taskRequestID})
		w := httptest.NewRecorder()

		(*controller).GetTask(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
		mockService.AssertNotCalled(t, "GetTaskWithRequestIdNonBlocking", taskRequestID, uint64(1))
	})
}
