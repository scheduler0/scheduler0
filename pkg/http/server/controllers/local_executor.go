package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"scheduler0-private/pkg/models"
	"scheduler0-private/pkg/service/executor"
	"scheduler0-private/pkg/utils"
	"strconv"

	"github.com/gorilla/mux"
)

// LocalExecutorHTTPController exposes the endpoints used by the scheduler0-cli local
// executor: register the machine, pull its assigned jobs, and report execution results.
type LocalExecutorHTTPController interface {
	RegisterLocalExecutor(w http.ResponseWriter, r *http.Request)
	PullJobs(w http.ResponseWriter, r *http.Request)
	ReportExecutions(w http.ResponseWriter, r *http.Request)
}

type localExecutorController struct {
	jobExecutorService executor.JobExecutorService
	logger             *log.Logger
}

func NewLocalExecutorController(logger *log.Logger, jobExecutorService executor.JobExecutorService) LocalExecutorHTTPController {
	return &localExecutorController{
		jobExecutorService: jobExecutorService,
		logger:             logger,
	}
}

func (c *localExecutorController) RegisterLocalExecutor(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RegisterLocalExecutor entry", r.URL.Path))

	body := utils.ExtractBody(w, r)
	if body == nil {
		utils.SendJSON(w, "body is nil", false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	var registerRequest struct {
		Name       string `json:"name"`
		Command    string `json:"command"`
		WorkingDir string `json:"workingDir"`
		CreatedBy  string `json:"createdBy"`
	}
	if err := json.Unmarshal(body, &registerRequest); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RegisterLocalExecutor error: failed to unmarshal request body, accountId=%d, error=%v", r.URL.Path, accountId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	if registerRequest.Name == "" {
		utils.SendJSON(w, "name is required", false, http.StatusBadRequest, nil)
		return
	}
	if registerRequest.Command == "" {
		utils.SendJSON(w, "command is required", false, http.StatusBadRequest, nil)
		return
	}

	createdBy := registerRequest.CreatedBy
	if createdBy == "" {
		utils.SendJSON(w, "createdBy is required", false, http.StatusBadRequest, nil)
		return
	}

	createdID, createErr := c.jobExecutorService.RegisterLocalExecutor(registerRequest.Name, registerRequest.Command, registerRequest.WorkingDir, createdBy, accountId)
	if createErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RegisterLocalExecutor error: failed to register executor, accountId=%d, error=%s", r.URL.Path, accountId, createErr.Message))
		utils.SendJSON(w, createErr.Message, false, createErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RegisterLocalExecutor success, status=201, executorId=%d, accountId=%d", r.URL.Path, createdID, accountId))
	utils.SendJSON(w, struct {
		ID int64 `json:"id"`
	}{ID: int64(createdID)}, true, http.StatusCreated, nil)
}

func (c *localExecutorController) PullJobs(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	executorID, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - PullJobs entry, executorId=%d, accountId=%d", r.URL.Path, executorID, accountId))

	jobs, pullErr := c.jobExecutorService.PullExecutorJobs(uint64(executorID), accountId)
	if pullErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - PullJobs error: failed to pull jobs, executorId=%d, accountId=%d, error=%s", r.URL.Path, executorID, accountId, pullErr.Message))
		utils.SendJSON(w, pullErr.Message, false, pullErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - PullJobs success, status=200, executorId=%d, accountId=%d, count=%d", r.URL.Path, executorID, accountId, len(jobs)))
	utils.SendJSON(w, jobs, true, http.StatusOK, nil)
}

func (c *localExecutorController) ReportExecutions(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	executorID, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}

	body := utils.ExtractBody(w, r)
	if body == nil {
		utils.SendJSON(w, "body is nil", false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	var reports []models.LocalExecutionReport
	if err := json.Unmarshal(body, &reports); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ReportExecutions error: failed to unmarshal request body, executorId=%d, accountId=%d, error=%v", r.URL.Path, executorID, accountId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ReportExecutions entry, executorId=%d, accountId=%d, reportCount=%d", r.URL.Path, executorID, accountId, len(reports)))

	committed, reportErr := c.jobExecutorService.ReportExecutions(uint64(executorID), accountId, reports)
	if reportErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ReportExecutions error: failed to report executions, executorId=%d, accountId=%d, error=%s", r.URL.Path, executorID, accountId, reportErr.Message))
		utils.SendJSON(w, reportErr.Message, false, reportErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ReportExecutions success, status=200, executorId=%d, accountId=%d, committed=%d", r.URL.Path, executorID, accountId, committed))
	utils.SendJSON(w, struct {
		Committed int `json:"committed"`
	}{Committed: committed}, true, http.StatusOK, nil)
}
