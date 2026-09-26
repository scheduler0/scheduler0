package controllers_test

import (
	"bytes"
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
	"github.com/stretchr/testify/require"
)

func setupLocalExecutorController(t *testing.T) (controllers.LocalExecutorHTTPController, *executor.MockJobExecutorService) {
	logger := log.New(&bytes.Buffer{}, "", 0)
	mockService := executor.NewMockJobExecutorService(t)
	controller := controllers.NewLocalExecutorController(logger, mockService)
	return controller, mockService
}

func TestLocalExecutorController_RegisterLocalExecutor(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		controller, mockService := setupLocalExecutorController(t)
		accountID := uint64(1)
		executorID := uint64(42)

		mockService.On(
			"RegisterLocalExecutor",
			"local-multi",
			"echo hello",
			"/tmp",
			"tester",
			accountID,
		).Return(executorID, nil)

		body, _ := json.Marshal(map[string]string{
			"name":       "local-multi",
			"command":    "echo hello",
			"workingDir": "/tmp",
			"createdBy":  "tester",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/local-executors", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.RegisterLocalExecutor(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		var resp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
			Success bool `json:"success"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.True(t, resp.Success)
		assert.Equal(t, int64(executorID), resp.Data.ID)
		mockService.AssertExpectations(t)
	})

	t.Run("missing command", func(t *testing.T) {
		controller, mockService := setupLocalExecutorController(t)
		accountID := uint64(1)

		body, _ := json.Marshal(map[string]string{
			"name":      "local-multi",
			"createdBy": "tester",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/local-executors", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.RegisterLocalExecutor(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "RegisterLocalExecutor")
	})
}

func TestLocalExecutorController_PullJobs_MultipleJobsOneExecutor(t *testing.T) {
	controller, mockService := setupLocalExecutorController(t)
	accountID := uint64(1)
	executorID := uint64(7)
	execID := executorID

	jobs := []models.Job{
		{ID: 1, ProjectID: 1, Spec: "@every 1m", Data: `{"job":"alpha"}`, ExecutorId: &execID, AccountId: accountID, Status: models.JobStatusActive},
		{ID: 2, ProjectID: 1, Spec: "@every 2m", Data: `{"job":"beta"}`, ExecutorId: &execID, AccountId: accountID, Status: models.JobStatusActive},
		{ID: 3, ProjectID: 1, Spec: "@every 5m", Data: `{"job":"gamma"}`, ExecutorId: &execID, AccountId: accountID, Status: models.JobStatusActive},
	}

	mockService.On("PullExecutorJobs", executorID, accountID).Return(jobs, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/local-executors/7/jobs", nil)
	req = req.WithContext(createContextWithAccountID(accountID))
	req = mux.SetURLVars(req, map[string]string{"id": "7"})
	w := httptest.NewRecorder()

	controller.PullJobs(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data    []models.Job `json:"data"`
		Success bool         `json:"success"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	require.Len(t, resp.Data, 3)

	seen := map[uint64]string{}
	for _, job := range resp.Data {
		require.NotNil(t, job.ExecutorId)
		assert.Equal(t, executorID, *job.ExecutorId)
		seen[job.ID] = job.Spec
	}
	assert.Equal(t, map[uint64]string{
		1: "@every 1m",
		2: "@every 2m",
		3: "@every 5m",
	}, seen)
	mockService.AssertExpectations(t)
}

func TestLocalExecutorController_ReportExecutions_MultipleJobs(t *testing.T) {
	controller, mockService := setupLocalExecutorController(t)
	accountID := uint64(1)
	executorID := uint64(7)

	reports := []models.LocalExecutionReport{
		{JobID: 1, UniqueID: "u1", State: 1, LastExecutionTime: "2026-07-28T21:00:00Z", NextExecutionTime: "2026-07-28T21:01:00Z", ExecutionVersion: 1, JobQueueVersion: 1},
		{JobID: 2, UniqueID: "u2", State: 1, LastExecutionTime: "2026-07-28T21:00:00Z", NextExecutionTime: "2026-07-28T21:02:00Z", ExecutionVersion: 1, JobQueueVersion: 1},
		{JobID: 3, UniqueID: "u3", State: 1, LastExecutionTime: "2026-07-28T21:00:00Z", NextExecutionTime: "2026-07-28T21:05:00Z", ExecutionVersion: 1, JobQueueVersion: 1},
	}

	mockService.On("ReportExecutions", executorID, accountID, reports).Return(3, nil)

	body, _ := json.Marshal(reports)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/local-executors/7/executions", bytes.NewBuffer(body))
	req = req.WithContext(createContextWithAccountID(accountID))
	req = mux.SetURLVars(req, map[string]string{"id": "7"})
	w := httptest.NewRecorder()

	controller.ReportExecutions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data struct {
			Committed int `json:"committed"`
		} `json:"data"`
		Success bool `json:"success"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, 3, resp.Data.Committed)
	mockService.AssertExpectations(t)
}

func TestLocalExecutorController_PullJobs_ServiceError(t *testing.T) {
	controller, mockService := setupLocalExecutorController(t)
	accountID := uint64(1)
	executorID := uint64(7)

	mockService.On("PullExecutorJobs", executorID, accountID).
		Return(nil, utils.HTTPGenericError(http.StatusNotFound, "executor not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/local-executors/7/jobs", nil)
	req = req.WithContext(createContextWithAccountID(accountID))
	req = mux.SetURLVars(req, map[string]string{"id": "7"})
	w := httptest.NewRecorder()

	controller.PullJobs(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockService.AssertExpectations(t)
}
