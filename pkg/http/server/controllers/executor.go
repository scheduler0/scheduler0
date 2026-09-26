package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"scheduler0-private/pkg/constants"
	"scheduler0-private/pkg/models"
	"scheduler0-private/pkg/service/executor"
	"scheduler0-private/pkg/utils"
	"strconv"

	"github.com/gorilla/mux"
)

type JobExecutorHTTPController interface {
	CreateOneExecutor(w http.ResponseWriter, r *http.Request)
	GetOneExecutor(w http.ResponseWriter, r *http.Request)
	UpdateOneExecutor(w http.ResponseWriter, r *http.Request)
	DeleteOneExecutor(w http.ResponseWriter, r *http.Request)
	ListExecutors(w http.ResponseWriter, r *http.Request)
	TestInvokeExecutor(w http.ResponseWriter, r *http.Request)
}

type jobExecutorController struct {
	jobExecutorService executor.JobExecutorService
	logger             *log.Logger
}

func NewJobExecutorController(logger *log.Logger, jobExecutorService executor.JobExecutorService) JobExecutorHTTPController {
	return &jobExecutorController{
		jobExecutorService,
		logger,
	}
}

func (c *jobExecutorController) CreateOneExecutor(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneExecutor entry", r.URL.Path))

	body := utils.ExtractBody(w, r)

	if body == nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneExecutor error: empty request body", r.URL.Path))
		utils.SendJSON(w, "body is nil", false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		requestID := utils.GetRequestID(r.Context())
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneExecutor error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	executor := models.JobExecutor{}
	if err := json.Unmarshal(body, &executor); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneExecutor error: failed to unmarshal request body, accountId=%d, error=%v", r.URL.Path, accountId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	if executor.CreatedBy == "" {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneExecutor error: createdBy is required, accountId=%d", r.URL.Path, accountId))
		utils.SendJSON(w, "createdBy is required", false, http.StatusBadRequest, nil)
		return
	}

	executor.AccountId = accountId

	createdExecutorId, createErr := c.jobExecutorService.CreateExecutor(executor)
	if createErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneExecutor error: failed to create executor, accountId=%d, error=%s", r.URL.Path, accountId, createErr.Message))
		utils.SendJSON(w, createErr.Message, false, createErr.Type, nil)
		return
	}

	createdExecutor, getErr := c.jobExecutorService.GetOneByID(createdExecutorId, accountId)
	if getErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneExecutor error: failed to get created executor, executorId=%d, accountId=%d, error=%s", r.URL.Path, createdExecutorId, accountId, getErr.Message))
		utils.SendJSON(w, getErr.Message, false, getErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneExecutor success, status=201, executorId=%d, accountId=%d", r.URL.Path, createdExecutorId, accountId))
	// The executor secrets are surfaced exactly once, here, on create. They are stored
	// encrypted at rest and the base JobExecutor model never serializes them (json:"-"),
	// so read endpoints (GET/LIST/UPDATE) don't leak them.
	response := struct {
		*models.JobExecutor
		CloudApiKey    string `json:"cloudApiKey,omitempty"`
		CloudApiSecret string `json:"cloudApiSecret,omitempty"`
		WebhookSecret  string `json:"webhookSecret,omitempty"`
	}{
		JobExecutor:    createdExecutor,
		CloudApiKey:    createdExecutor.CloudApiKey,
		CloudApiSecret: createdExecutor.CloudApiSecret,
		WebhookSecret:  createdExecutor.WebhookSecret,
	}
	utils.SendJSON(w, response, true, http.StatusCreated, nil)
}

// TestInvokeExecutor handles POST /api/v1/executors/{id}/test-invoke. It fires a
// synthetic ("test") job through the executor immediately and synchronously so
// developers can exercise a scheduled job without waiting for its spec/start date
// to elapse. Nothing is persisted or rescheduled. The endpoint returns 200 once
// the invocation completes; whether the target accepted it is reported in the
// response body's `success` field.
func (c *jobExecutorController) TestInvokeExecutor(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - TestInvokeExecutor entry", r.URL.Path))

	params := mux.Vars(r)
	executorID, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - TestInvokeExecutor error: invalid executor ID parameter, id=%s, error=%v", r.URL.Path, params["id"], convertErr))
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - TestInvokeExecutor error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	// The request body is optional: an empty body test-invokes with defaults.
	// ExtractBody is intentionally NOT used here because it rejects empty bodies.
	testRequest := models.TestInvocationRequest{}
	body, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - TestInvokeExecutor error: failed to read request body, executorId=%d, accountId=%d, error=%v", r.URL.Path, executorID, accountId, readErr))
		utils.SendJSON(w, "failed to read request body", false, http.StatusUnprocessableEntity, nil)
		return
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &testRequest); err != nil {
			utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - TestInvokeExecutor error: failed to unmarshal request body, executorId=%d, accountId=%d, error=%v", r.URL.Path, executorID, accountId, err))
			utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
			return
		}
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - TestInvokeExecutor invoking, executorId=%d, accountId=%d", r.URL.Path, executorID, accountId))

	result, invokeErr := c.jobExecutorService.TestInvokeExecutor(accountId, uint64(executorID), testRequest)
	if invokeErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - TestInvokeExecutor error: %s, executorId=%d, accountId=%d", r.URL.Path, invokeErr.Message, executorID, accountId))
		utils.SendJSON(w, invokeErr.Message, false, invokeErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - TestInvokeExecutor success, status=200, executorId=%d, accountId=%d, invocationSuccess=%t, durationMs=%d", r.URL.Path, executorID, accountId, result.Success, result.DurationMs))
	utils.SendJSON(w, result, true, http.StatusOK, nil)
}

func (c *jobExecutorController) GetOneExecutor(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	executorID, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetOneExecutor error: invalid executor ID parameter, id=%s, error=%v", r.URL.Path, params["id"], convertErr))
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		requestID := utils.GetRequestID(r.Context())
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneExecutor error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	executorIDUint64 := uint64(executorID)

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetOneExecutor entry, executorId=%d, accountId=%d", r.URL.Path, executorID, accountId))

	executorT, getOneExecutorError := c.jobExecutorService.GetOneByID(executorIDUint64, accountId)
	if getOneExecutorError != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetOneExecutor error: failed to get executor, executorId=%d, accountId=%d, error=%s", r.URL.Path, executorID, accountId, getOneExecutorError.Message))
		utils.SendJSON(w, getOneExecutorError.Message, false, getOneExecutorError.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetOneExecutor success, status=200, executorId=%d, accountId=%d", r.URL.Path, executorID, accountId))
	utils.SendJSON(w, executorT, true, http.StatusOK, nil)
}

func (c *jobExecutorController) UpdateOneExecutor(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	body := utils.ExtractBody(w, r)

	if body == nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneExecutor error: empty request body", r.URL.Path))
		utils.SendJSON(w, "body is nil", false, http.StatusBadRequest, nil)
		return
	}

	params := mux.Vars(r)

	executorID, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneExecutor error: invalid executor ID parameter, id=%s, error=%v", r.URL.Path, params["id"], convertErr))
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		requestID := utils.GetRequestID(r.Context())
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneExecutor error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	executor := models.JobExecutor{
		ID:        uint64(executorID),
		AccountId: accountId,
	}

	if err := json.Unmarshal(body, &executor); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneExecutor error: failed to unmarshal request body, executorId=%d, accountId=%d, error=%v", r.URL.Path, executorID, accountId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	if executor.ModifiedBy == nil || *executor.ModifiedBy == "" {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneExecutor error: modifiedBy is required, executorId=%d", r.URL.Path, executorID))
		utils.SendJSON(w, "modifiedBy is required", false, http.StatusBadRequest, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneExecutor entry, executorId=%d, accountId=%d", r.URL.Path, executorID, accountId))

	executor.AccountId = accountId

	updatedExecutorID, updateErr := c.jobExecutorService.UpdateOneBy(executor)
	if updateErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneExecutor error: failed to update executor, executorId=%d, accountId=%d, error=%s", r.URL.Path, executorID, accountId, updateErr.Message))
		utils.SendJSON(w, updateErr.Message, false, updateErr.Type, nil)
		return
	}

	// Fetch the updated executor to return the full object
	updatedExecutor, getErr := c.jobExecutorService.GetOneByID(updatedExecutorID, accountId)
	if getErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneExecutor error: failed to get updated executor, executorId=%d, accountId=%d, error=%s", r.URL.Path, updatedExecutorID, accountId, getErr.Message))
		utils.SendJSON(w, getErr.Message, false, getErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneExecutor success, status=200, executorId=%d, accountId=%d", r.URL.Path, executorID, accountId))
	utils.SendJSON(w, updatedExecutor, true, http.StatusOK, nil)
}

func (c *jobExecutorController) DeleteOneExecutor(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	executorID, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneExecutor error: invalid executor ID parameter, id=%s, error=%v", r.URL.Path, params["id"], convertErr))
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}

	// Parse request body to get deletedBy
	body := utils.ExtractBody(w, r)
	if body == nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneExecutor error: empty request body", r.URL.Path))
		return
	}

	var deleteRequest struct {
		DeletedBy string `json:"deletedBy"`
	}

	if err := json.Unmarshal(body, &deleteRequest); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneExecutor error: invalid request body, executorId=%d, error=%v", r.URL.Path, executorID, err))
		utils.SendJSON(w, "Invalid request body", false, http.StatusBadRequest, nil)
		return
	}

	if deleteRequest.DeletedBy == "" {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneExecutor error: deletedBy is required, executorId=%d", r.URL.Path, executorID))
		utils.SendJSON(w, "deletedBy is required", false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		requestID := utils.GetRequestID(r.Context())
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneExecutor error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	executorIDUint64 := uint64(executorID)

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneExecutor entry, executorId=%d, accountId=%d, deletedBy=%s", r.URL.Path, executorID, accountId, deleteRequest.DeletedBy))

	_, deleteErr := c.jobExecutorService.DeleteOneByID(executorIDUint64, accountId, deleteRequest.DeletedBy)
	if deleteErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneExecutor error: failed to delete executor, executorId=%d, accountId=%d, error=%s", r.URL.Path, executorID, accountId, deleteErr.Message))
		utils.SendJSON(w, deleteErr.Message, false, deleteErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneExecutor success, status=204, executorId=%d, accountId=%d", r.URL.Path, executorID, accountId))
	utils.SendJSON(w, nil, true, http.StatusNoContent, nil)
}

func (c *jobExecutorController) ListExecutors(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - ListExecutors entry, query=%s", r.URL.Path, r.URL.RawQuery))

	defaultLimit := strconv.Itoa(constants.DefaultListLimit)
	defaultOffset := "0"

	limitParam, err := utils.ValidateQueryStringWithDefault("limit", r, &defaultLimit)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - ListExecutors error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offsetParam, err := utils.ValidateQueryStringWithDefault("offset", r, &defaultOffset)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - ListExecutors error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offset, err := strconv.Atoi(offsetParam)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - ListExecutors error: invalid offset parameter, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	limit, err := strconv.Atoi(limitParam)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - ListExecutors error: invalid limit parameter, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		requestID := utils.GetRequestID(r.Context())
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneExecutor error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	defaultOrderByColumn := constants.JobExecutorDateCreatedColumn
	defaultOrderByDirection := constants.OrderDirectionDesc

	orderByColumn, err := utils.ValidateQueryStringWithDefault("orderBy", r, &defaultOrderByColumn)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - ListExecutors error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	orderByDirection, err := utils.ValidateQueryStringWithDefault("orderByDirection", r, &defaultOrderByDirection)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - ListExecutors error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	executors, listErr := c.jobExecutorService.ListJobExecutors(uint64(offset), uint64(limit), orderByColumn, orderByDirection, accountId)
	if listErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - ListExecutors error: failed to list executors, accountId=%d, offset=%d, limit=%d, error=%s", r.URL.Path, accountId, offset, limit, listErr.Message))
		utils.SendJSON(w, listErr.Message, false, listErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - ListExecutors success, status=200, accountId=%d, count=%d, offset=%d, limit=%d", r.URL.Path, accountId, executors.Total, offset, limit))
	utils.SendJSON(w, executors, true, http.StatusOK, nil)
}
