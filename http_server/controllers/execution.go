package controllers

import (
	"net/http"
	"scheduler0/service"
	"scheduler0/utils"
	"strconv"
)

type Execution interface {
	ListExecutions(w http.ResponseWriter, r *http.Request)
}

// ExecutionsHTTPController handlers all incoming http requests for /executions
type executionsHTTPController struct {
	executionsService service.ExecutionService
}

func NewExecutionsController(executionsService service.ExecutionService) Execution {
	return &executionsHTTPController{
		executionsService: executionsService,
	}
}

// ListExecutions request handler that returns paginated executions result set
func (executionController *executionsHTTPController) ListExecutions(w http.ResponseWriter, r *http.Request) {
	offset := 0
	limit := 50

	jobIDQueryParam, err := utils.ValidateQueryString("jobID", r)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	limitParam, err := utils.ValidateQueryString("limit", r)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offsetParam, err := utils.ValidateQueryString("offset", r)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offset, err = strconv.Atoi(offsetParam)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	limit, err = strconv.Atoi(limitParam)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	jobID, convertErr := strconv.Atoi(jobIDQueryParam)
	if convertErr != nil {
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
	}

	executions, getError := executionController.executionsService.GetAllExecutionsByJobUUID(int64(jobID), int64(offset), int64(limit))

	if getError != nil {
		utils.SendJSON(w, getError.Message, false, http.StatusBadRequest, nil)
	} else {
		utils.SendJSON(w, executions, true, http.StatusOK, nil)
	}
}
