package execution

import (
	"github.com/victorlenerd/scheduler0/server/src/controllers"
	"github.com/victorlenerd/scheduler0/server/src/service"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"net/http"
	"strconv"
)

type ExecutionController controllers.Controller


func (executionController *ExecutionController) List(w http.ResponseWriter, r *http.Request) {
	executionService := service.ExecutionService{Pool: executionController.Pool, Ctx: r.Context()}

	offset := 0
	limit := 50

	jobID, err := utils.ValidateQueryString("jobUUID", r)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	limitParam, err := utils.ValidateQueryString("limit", r)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offsetParam, err := utils.ValidateQueryString("offset", r)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offset, err = strconv.Atoi(offsetParam)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	limit, err = strconv.Atoi(limitParam)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	executions, err := executionService.GetAllExecutionsByJobUUID(jobID, offset, limit)

	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
	} else {
		utils.SendJson(w, executions, true, http.StatusOK, nil)
	}
}
