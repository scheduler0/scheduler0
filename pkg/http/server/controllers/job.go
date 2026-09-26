package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	"scheduler0/pkg/service/job"
	"scheduler0/pkg/service/job_execution_service"
	"scheduler0/pkg/service/project"
	"scheduler0/pkg/utils"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// HTTPController http request handler for /job requests
type jobHTTPController struct {
	jobService             job.JobService
	projectService         project.ProjectService
	logger                 *log.Logger
	jobExecutionLogService job_execution_service.JobExecutionLogService
}

type JobHTTPController interface {
	ListJobs(w http.ResponseWriter, r *http.Request)
	BatchCreateJobs(w http.ResponseWriter, r *http.Request)
	GetOneJob(w http.ResponseWriter, r *http.Request)
	UpdateOneJob(w http.ResponseWriter, r *http.Request)
	DeleteOneJob(w http.ResponseWriter, r *http.Request)
	GetJobExecutionLogs(w http.ResponseWriter, r *http.Request)
	GetDateRangeAnalytics(w http.ResponseWriter, r *http.Request)
	GetExecutionTotals(w http.ResponseWriter, r *http.Request)
	CleanupOldExecutionLogs(w http.ResponseWriter, r *http.Request)
}

func NewJoBHTTPController(logger *log.Logger, jobService job.JobService, projectService project.ProjectService, jobExecutionLogService job_execution_service.JobExecutionLogService) JobHTTPController {
	controller := &jobHTTPController{
		jobService:             jobService,
		projectService:         projectService,
		logger:                 logger,
		jobExecutionLogService: jobExecutionLogService,
	}
	return controller
}

// ListJobs returns a paginated list of jobs
func (jobController *jobHTTPController) ListJobs(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - ListJobs entry, query=%s", r.URL.Path, r.URL.RawQuery))

	// Make projectId optional - if not provided, list all jobs for the account
	var projectID *uint64
	projectIDQueryParam, err := utils.ValidateQueryString("projectId", r)
	if err == nil {
		// projectId was provided, validate it
		projectIDInt, convertErr := strconv.Atoi(projectIDQueryParam)
		if convertErr != nil {
			utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - ListJobs error: invalid projectId parameter", r.URL.Path))
			utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
			return
		}
		projectIDUint := uint64(projectIDInt)
		projectID = &projectIDUint
	}
	// If projectId is not provided, projectID remains nil

	defaultLimit := strconv.Itoa(constants.DefaultListLimit)
	defaultOffset := "0"

	limitParam, err := utils.ValidateQueryStringWithDefault("limit", r, &defaultLimit)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - ListJobs error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offsetParam, err := utils.ValidateQueryStringWithDefault("offset", r, &defaultOffset)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - ListJobs error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offset, err := strconv.Atoi(offsetParam)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - ListJobs error: invalid offset parameter", r.URL.Path))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	limit, err := strconv.Atoi(limitParam)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - ListJobs error: invalid limit parameter", r.URL.Path))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - ListJobs error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	// Only validate project if projectID is provided
	if projectID != nil {
		project := models.Project{
			ID:        *projectID,
			AccountId: accountId,
		}
		getErr := jobController.projectService.GetOneByID(&project)
		if getErr != nil {
			utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - ListJobs error: project validation failed, projectId=%d, accountId=%d, error=%v", r.URL.Path, *projectID, accountId, getErr))
			utils.SendJSON(w, getErr.Error(), false, http.StatusBadRequest, nil)
			return
		}

		if project.AccountId != accountId {
			utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - ListJobs error: project not found, projectId=%d, accountId=%d", r.URL.Path, *projectID, accountId))
			utils.SendJSON(w, "project not found", false, http.StatusNotFound, nil)
			return
		}
	}

	defaultOrderByColumn := constants.JobsDateCreatedColumn
	defaultOrderByDirection := constants.OrderDirectionDesc

	orderByColumn, err := utils.ValidateQueryStringWithDefault("orderBy", r, &defaultOrderByColumn)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - ListJobs error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	orderByDirection, err := utils.ValidateQueryStringWithDefault("orderByDirection", r, &defaultOrderByDirection)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - ListJobs error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	jobs, getJobsError := jobController.jobService.GetJobsByAccountID(accountId, projectID, uint64(offset), uint64(limit), orderByColumn, orderByDirection)
	if getJobsError != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - ListJobs error: failed to get jobs, accountId=%d, projectId=%v, error=%s", r.URL.Path, accountId, projectID, getJobsError.Message))
		utils.SendJSON(w, getJobsError.Message, false, getJobsError.Type, nil)
		return
	}

	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - ListJobs success, status=200, accountId=%d, projectId=%v, count=%d, offset=%d, limit=%d", r.URL.Path, accountId, projectID, jobs.Total, offset, limit))
	utils.SendJSON(w, jobs, true, http.StatusOK, nil)
}

// BatchCreateJobs handles request to job in batches
func (jobController *jobHTTPController) BatchCreateJobs(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - BatchCreateJobs entry", r.URL.Path))

	body := utils.ExtractBody(w, r)

	if body == nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - BatchCreateJobs error: empty request body", r.URL.Path))
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - BatchCreateJobs error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	jobs := []models.Job{}

	if err := json.Unmarshal(body, &jobs); err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - BatchCreateJobs error: failed to unmarshal request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - BatchCreateJobs processing, accountId=%d, jobCount=%d", r.URL.Path, accountId, len(jobs)))

	for i := range jobs {
		jobs[i].AccountId = accountId
		if jobs[i].CreatedBy == "" {
			utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - BatchCreateJobs error: createdBy is required for all jobs", r.URL.Path))
			utils.SendJSON(w, "createdBy is required for all jobs", false, http.StatusBadRequest, nil)
			return
		}
	}

	if requestID == "" {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - BatchCreateJobs error: request ID not found in context", r.URL.Path))
		utils.SendJSON(w, "request ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	_, createErr := jobController.jobService.BatchInsertJobs(requestID, jobs)
	if createErr != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - BatchCreateJobs error: failed to create jobs, accountId=%d, jobCount=%d, error=%s", r.URL.Path, accountId, len(jobs), createErr.Message))
		utils.SendJSON(w, createErr.Message, false, http.StatusBadRequest, nil)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/async-tasks/%s", requestID))

	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - BatchCreateJobs success, status=202, accountId=%d, jobCount=%d, asyncTaskId=%s", r.URL.Path, accountId, len(jobs), requestID))
	// Return request ID for tracking async operation
	utils.SendJSON(w, requestID, true, http.StatusAccepted, nil)
}

// GetOneJob handles request to return a single job
func (jobController *jobHTTPController) GetOneJob(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	jobID, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetOneJob error: invalid job ID parameter, id=%s, error=%v", r.URL.Path, params["id"], convertErr))
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetOneJob error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetOneJob entry, jobId=%d, accountId=%d", r.URL.Path, jobID, accountId))

	job := models.Job{
		ID:        uint64(jobID),
		AccountId: accountId,
	}

	jobT, getOneJobError := jobController.jobService.GetJob(job)
	if getOneJobError != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetOneJob error: failed to get job, jobId=%d, accountId=%d, error=%s", r.URL.Path, jobID, accountId, getOneJobError.Message))
		utils.SendJSON(w, getOneJobError.Message, false, getOneJobError.Type, nil)
		return
	}

	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetOneJob success, status=200, jobId=%d, accountId=%d", r.URL.Path, jobID, accountId))
	utils.SendJSON(w, jobT, true, http.StatusOK, nil)
}

// UpdateOneJob handles request to update a single job
func (jobController *jobHTTPController) UpdateOneJob(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	body := utils.ExtractBody(w, r)
	jobBody := models.Job{}
	err := jobBody.FromJSON(body)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneJob error: failed to unmarshal request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	jobID, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneJob error: invalid job ID parameter, id=%s, error=%v", r.URL.Path, params["id"], convertErr))
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}

	jobBody.ID = uint64(jobID)

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - UpdateOneJob error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	jobBody.AccountId = accountId

	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneJob entry, jobId=%d, accountId=%d", r.URL.Path, jobID, accountId))

	if jobBody.ModifiedBy == nil || *jobBody.ModifiedBy == "" {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneJob error: modifiedBy is required, jobId=%d", r.URL.Path, jobID))
		utils.SendJSON(w, "modifiedBy is required", false, http.StatusBadRequest, nil)
		return
	}

	jobT, updateOneJobError := jobController.jobService.UpdateJob(jobBody)
	if updateOneJobError != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneJob error: failed to update job, jobId=%d, accountId=%d, error=%s", r.URL.Path, jobID, accountId, updateOneJobError.Message))
		utils.SendJSON(w, updateOneJobError.Message, false, updateOneJobError.Type, nil)
		return
	}

	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneJob success, status=200, jobId=%d, accountId=%d", r.URL.Path, jobID, accountId))
	utils.SendJSON(w, jobT, true, http.StatusOK, nil)
}

// DeleteOneJob handles request to delete a single job
func (jobController *jobHTTPController) DeleteOneJob(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	jobID, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneJob error: invalid job ID parameter, id=%s, error=%v", r.URL.Path, params["id"], convertErr))
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}

	// Parse request body to get deletedBy
	body := utils.ExtractBody(w, r)
	if body == nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneJob error: empty request body", r.URL.Path))
		return
	}

	var deleteRequest struct {
		DeletedBy string `json:"deletedBy"`
	}

	if err := json.Unmarshal(body, &deleteRequest); err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneJob error: invalid request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, "Invalid request body", false, http.StatusBadRequest, nil)
		return
	}

	if deleteRequest.DeletedBy == "" {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneJob error: deletedBy is required, jobId=%d", r.URL.Path, jobID))
		utils.SendJSON(w, "deletedBy is required", false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneJob entry, jobId=%d, accountId=%d, deletedBy=%s", r.URL.Path, jobID, accountId, deleteRequest.DeletedBy))

	job := models.Job{
		ID:        uint64(jobID),
		AccountId: accountId,
		DeletedBy: &deleteRequest.DeletedBy,
	}

	deleteOneJobError := jobController.jobService.DeleteJob(job)
	if deleteOneJobError != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneJob error: failed to delete job, jobId=%d, accountId=%d, error=%s", r.URL.Path, jobID, accountId, deleteOneJobError.Message))
		utils.SendJSON(w, deleteOneJobError.Message, false, deleteOneJobError.Type, nil)
		return
	}

	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneJob success, status=204, jobId=%d, accountId=%d", r.URL.Path, jobID, accountId))
	utils.SendJSON(w, nil, true, http.StatusNoContent, nil)
}

// GetJobExecutionLogs handles GET /job-execution-logs
func (jobController *jobHTTPController) GetJobExecutionLogs(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs entry, query=%s", r.URL.Path, r.URL.RawQuery))

	query := r.URL.Query()
	startDateStr := query.Get("startDate")
	endDateStr := query.Get("endDate")
	projectIdStr := query.Get("projectId")
	jobIdStr := query.Get("jobId")
	stateStr := query.Get("state")
	orderByStr := query.Get("orderBy")
	orderDirectionStr := query.Get("orderDirection")
	limitStr := query.Get("limit")
	offsetStr := query.Get("offset")

	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs query parameters: startDate=%s, endDate=%s, projectId=%s, jobId=%s, state=%s, orderBy=%s, orderDirection=%s, limit=%s, offset=%s",
		r.URL.Path, startDateStr, endDateStr, projectIdStr, jobIdStr, stateStr, orderByStr, orderDirectionStr, limitStr, offsetStr))

	// pagination defaults
	defaultLimit := strconv.Itoa(constants.DefaultExecutionLogsListLimit)
	defaultOffset := "0"

	limitParam, err := utils.ValidateQueryStringWithDefault("limit", r, &defaultLimit)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offsetParam, err := utils.ValidateQueryStringWithDefault("offset", r, &defaultOffset)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	// parse pagination numbers
	offset, err := strconv.Atoi(offsetParam)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs error: invalid offset parameter", r.URL.Path))
		utils.SendJSON(w, "invalid offset", false, http.StatusBadRequest, nil)
		return
	}
	limit, err := strconv.Atoi(limitParam)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs error: invalid limit parameter", r.URL.Path))
		utils.SendJSON(w, "invalid limit", false, http.StatusBadRequest, nil)
		return
	}

	// Parse dates if provided (optional)
	var startDate, endDate *time.Time
	if startDateStr != "" {
		parsedStartDate, err := time.Parse(time.RFC3339, startDateStr)
		if err != nil {
			utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs error: invalid startDate format, expected RFC3339", r.URL.Path))
			utils.SendJSON(w, "invalid startDate (expected RFC3339)", false, http.StatusBadRequest, nil)
			return
		}
		startDate = &parsedStartDate
	}
	if endDateStr != "" {
		parsedEndDate, err := time.Parse(time.RFC3339, endDateStr)
		if err != nil {
			utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs error: invalid endDate format, expected RFC3339", r.URL.Path))
			utils.SendJSON(w, "invalid endDate (expected RFC3339)", false, http.StatusBadRequest, nil)
			return
		}
		endDate = &parsedEndDate
	}

	var projectId *uint64
	if projectIdStr != "" {
		pid, err := strconv.ParseUint(projectIdStr, 10, 64)
		if err != nil {
			utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs error: invalid projectId parameter", r.URL.Path))
			utils.SendJSON(w, "invalid projectId", false, http.StatusBadRequest, nil)
			return
		}
		if pid > 0 {
			projectId = &pid
		}
	}
	var jobId *uint64
	if jobIdStr != "" {
		jid, err := strconv.ParseUint(jobIdStr, 10, 64)
		if err != nil {
			utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs error: invalid jobId parameter", r.URL.Path))
			utils.SendJSON(w, "invalid jobId", false, http.StatusBadRequest, nil)
			return
		}
		if jid > 0 {
			jobId = &jid
		}
	}

	// Parse state filter if provided
	var state *models.JobExecutionLogState
	if stateStr != "" {
		var stateValue models.JobExecutionLogState
		switch stateStr {
		case models.ExecutionStateScheduled:
			stateValue = models.ExecutionLogScheduleState
		case models.ExecutionStateSuccess:
			stateValue = models.ExecutionLogSuccessState
		case models.ExecutionStateFailed:
			stateValue = models.ExecutionLogFailedState
		default:
			utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs error: invalid state parameter, expected '%s', '%s', or '%s'", r.URL.Path, models.ExecutionStateScheduled, models.ExecutionStateSuccess, models.ExecutionStateFailed))
			utils.SendJSON(w, fmt.Sprintf("invalid state parameter, expected '%s', '%s', or '%s'", models.ExecutionStateScheduled, models.ExecutionStateSuccess, models.ExecutionStateFailed), false, http.StatusBadRequest, nil)
			return
		}
		state = &stateValue
	}

	// Parse sort parameters
	orderBy := query.Get("orderBy")
	orderDirection := query.Get("orderDirection")

	// Validate orderBy
	validOrderByFields := map[string]bool{
		"dateCreated":           true,
		"lastExecutionDateTime": true,
		"nextExecutionDateTime": true,
	}
	if orderBy != "" && !validOrderByFields[orderBy] {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs error: invalid orderBy parameter, expected 'dateCreated', 'lastExecutionDateTime', or 'nextExecutionDateTime'", r.URL.Path))
		utils.SendJSON(w, "invalid orderBy parameter, expected 'dateCreated', 'lastExecutionDateTime', or 'nextExecutionDateTime'", false, http.StatusBadRequest, nil)
		return
	}

	// Validate orderDirection (default to DESC if invalid)
	if orderDirection != constants.OrderDirectionAsc && orderDirection != constants.OrderDirectionDesc {
		if orderBy != "" {
			// If orderBy is provided but orderDirection is invalid, default to DESC
			orderDirection = constants.OrderDirectionDesc
		} else {
			// If no orderBy, default to dateCreated DESC
			orderBy = "dateCreated"
			orderDirection = constants.OrderDirectionDesc
		}
	}

	// Default to dateCreated DESC if no sort parameters provided
	if orderBy == "" {
		orderBy = "dateCreated"
		orderDirection = constants.OrderDirectionDesc
	}

	logs, err := jobController.jobExecutionLogService.GetExecutionLogsFiltered(accountId, startDate, endDate, projectId, jobId, state, orderBy, orderDirection)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs error: failed to get execution logs, accountId=%d, projectId=%v, jobId=%v, error=%v", r.URL.Path, accountId, projectId, jobId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}

	// apply pagination in-memory for now
	total := len(logs)
	start := offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	paged := logs[start:end]

	resp := models.PaginatedJobExecutionLog{
		Total:  uint64(total),
		Offset: uint64(offset),
		Limit:  uint64(limit),
		Data:   paged,
	}

	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("GET %s - GetJobExecutionLogs success, status=200, accountId=%d, total=%d, returned=%d, offset=%d, limit=%d", r.URL.Path, accountId, total, len(paged), offset, limit))
	utils.SendJSON(w, resp, true, http.StatusOK, nil)
}

// GetDateRangeAnalytics returns execution counts grouped by minute buckets for a date range
func (jobController *jobHTTPController) GetDateRangeAnalytics(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetDateRangeAnalytics entry, query=%s", r.Method, r.URL.Path, r.URL.RawQuery))

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetDateRangeAnalytics error: account ID not found in context", r.Method, r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	// Extract query string values (timezone conversion is done on frontend)
	startDateStr, err := utils.ValidateQueryString("startDate", r)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetDateRangeAnalytics error: %v", r.Method, r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	startTimeStr, err := utils.ValidateQueryString("startTime", r)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetDateRangeAnalytics error: %v", r.Method, r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	// Parse startDate (YYYY-MM-DD format)
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetDateRangeAnalytics error: invalid startDate format: %v", r.Method, r.URL.Path, err))
		utils.SendJSON(w, "invalid startDate format (expected YYYY-MM-DD)", false, http.StatusBadRequest, nil)
		return
	}

	// Parse startTime (HH:MM:SS or HH:MM format)
	var startTime time.Time
	if len(startTimeStr) == 8 { // HH:MM:SS
		startTime, err = time.Parse("15:04:05", startTimeStr)
	} else if len(startTimeStr) == 5 { // HH:MM
		startTime, err = time.Parse("15:04", startTimeStr)
		// Add seconds
		startTime = time.Date(2000, 1, 1, startTime.Hour(), startTime.Minute(), 0, 0, time.UTC)
	} else {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetDateRangeAnalytics error: invalid startTime format", r.Method, r.URL.Path))
		utils.SendJSON(w, "invalid startTime format (expected HH:MM:SS or HH:MM)", false, http.StatusBadRequest, nil)
		return
	}

	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetDateRangeAnalytics error: invalid startTime format: %v", r.Method, r.URL.Path, err))
		utils.SendJSON(w, "invalid startTime format", false, http.StatusBadRequest, nil)
		return
	}

	// Extract time components (dates/times should already be in UTC from frontend)
	startTimeOnly := time.Date(2000, 1, 1, startTime.Hour(), startTime.Minute(), startTime.Second(), 0, time.UTC)

	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetDateRangeAnalytics: parsed params, accountId=%d, startDate=%s, startTime=%s", r.Method, r.URL.Path, accountId, startDateStr, startTimeStr))

	// Call service method (all times in UTC)
	response, err := jobController.jobExecutionLogService.GetDateRangeAnalytics(accountId, startDate, startTimeOnly)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetDateRangeAnalytics error: failed to get analytics, accountId=%d, error=%v", r.Method, r.URL.Path, accountId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}

	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetDateRangeAnalytics success, status=200, accountId=%d, pointsCount=%d", r.Method, r.URL.Path, accountId, len(response.Points)))
	utils.SendJSON(w, response, true, http.StatusOK, nil)
}

// GetExecutionTotals returns total counts of scheduled, success, and failed executions for an account
func (jobController *jobHTTPController) GetExecutionTotals(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetExecutionTotals entry", r.Method, r.URL.Path))

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetExecutionTotals error: account ID not found in context", r.Method, r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	// Call service method
	response, err := jobController.jobExecutionLogService.GetExecutionTotals(accountId)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetExecutionTotals error: failed to get execution totals, accountId=%d, error=%v", r.Method, r.URL.Path, accountId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}

	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("%s %s - GetExecutionTotals success, status=200, accountId=%d, scheduled=%d, success=%d, failed=%d", r.Method, r.URL.Path, accountId, response.Scheduled, response.Success, response.Failed))
	utils.SendJSON(w, response, true, http.StatusOK, nil)
}

// CleanupOldExecutionLogs handles POST /executions/cleanup-old-logs
// This is a peer-authenticated endpoint that cleans up old execution logs for a specific account
func (jobController *jobHTTPController) CleanupOldExecutionLogs(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - CleanupOldExecutionLogs entry", r.URL.Path))

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - CleanupOldExecutionLogs error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	body := utils.ExtractBody(w, r)
	if body == nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - CleanupOldExecutionLogs error: request body is required", r.URL.Path))
		utils.SendJSON(w, "request body is required", false, http.StatusBadRequest, nil)
		return
	}

	var requestBody struct {
		AccountID       string `json:"accountId"`
		RetentionMonths int    `json:"retentionMonths"`
	}

	if err := json.Unmarshal(body, &requestBody); err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - CleanupOldExecutionLogs error: failed to parse request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, "invalid request body", false, http.StatusBadRequest, nil)
		return
	}

	if requestBody.AccountID == "" {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - CleanupOldExecutionLogs error: accountId is required", r.URL.Path))
		utils.SendJSON(w, "accountId is required", false, http.StatusBadRequest, nil)
		return
	}

	requestAccountId, err := strconv.ParseUint(requestBody.AccountID, 10, 64)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - CleanupOldExecutionLogs error: invalid account ID, error=%v", r.URL.Path, err))
		utils.SendJSON(w, "invalid account ID", false, http.StatusBadRequest, nil)
		return
	}

	if accountId != requestAccountId {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - CleanupOldExecutionLogs error: account ID mismatch, request accountId=%d, expected accountId=%d", r.URL.Path, requestAccountId, accountId))
		utils.SendJSON(w, "account ID mismatch", false, http.StatusBadRequest, nil)
		return
	}

	if requestBody.RetentionMonths <= 0 {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - CleanupOldExecutionLogs error: retentionMonths must be greater than 0", r.URL.Path))
		utils.SendJSON(w, "retentionMonths must be greater than 0", false, http.StatusBadRequest, nil)
		return
	}

	// Per-account cleanup
	accountID, err := strconv.ParseUint(requestBody.AccountID, 10, 64)
	if err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - CleanupOldExecutionLogs error: invalid account ID, error=%v", r.URL.Path, err))
		utils.SendJSON(w, "invalid account ID", false, http.StatusBadRequest, nil)
		return
	}

	retentionDays := requestBody.RetentionMonths * 30 // Convert months to days (approximate)
	if err := jobController.jobExecutionLogService.CleanupOldExecutionLogsForAccount(accountID, retentionDays); err != nil {
		utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - CleanupOldExecutionLogs error: failed to cleanup old execution logs for account, accountId=%d, error=%v", r.URL.Path, accountID, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}

	utils.LogWithRequestID(jobController.logger, requestID, "", fmt.Sprintf("POST %s - CleanupOldExecutionLogs success for account, accountId=%d, retentionMonths=%d", r.URL.Path, accountID, requestBody.RetentionMonths))
	utils.SendJSON(w, map[string]string{"message": fmt.Sprintf("Old execution logs cleaned up successfully for account %d", accountID)}, true, http.StatusOK, nil)
}
