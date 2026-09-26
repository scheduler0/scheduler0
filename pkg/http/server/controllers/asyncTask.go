package controllers

import (
	"fmt"
	"log"
	"net/http"
	"scheduler0/pkg/models"
	"scheduler0/pkg/service/async_task"
	"scheduler0/pkg/utils"

	"github.com/gorilla/mux"
)

type AsyncTaskController interface {
	GetTask(w http.ResponseWriter, r *http.Request)
}

type asyncTaskController struct {
	logger           *log.Logger
	asyncTaskService async_task.AsyncTaskService
}

func NewAsyncTaskController(logger *log.Logger, asyncTaskService async_task.AsyncTaskService) AsyncTaskController {
	controller := asyncTaskController{
		logger:           logger,
		asyncTaskService: asyncTaskService,
	}
	return &controller
}

func (controller *asyncTaskController) GetTask(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)
	taskRequestID := params["id"]

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask error: account ID not found in context, taskRequestId=%s", r.URL.Path, taskRequestID))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask entry, taskRequestId=%s, accountId=%d", r.URL.Path, taskRequestID, accountId))

	task, err := controller.asyncTaskService.GetTaskWithRequestIdNonBlocking(taskRequestID, accountId)
	if err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask error: failed to get task (non-blocking), taskRequestId=%s, accountId=%d, error=%s", r.URL.Path, taskRequestID, accountId, err.Message))
		utils.SendJSON(w, err.Error(), false, err.Type, nil)
		return
	}
	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask retrieved task from non-blocking request, taskId=%d, state=%d", r.URL.Path, task.Id, task.State))

	if task.State == models.AsyncTaskSuccess || task.State == models.AsyncTaskFail {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask success, status=200, taskId=%d, state=%d", r.URL.Path, task.Id, task.State))
		utils.SendJSON(w, task, true, http.StatusOK, nil)
		return
	}

	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask task not complete, switching to blocking mode, taskRequestId=%s, accountId=%d", r.URL.Path, taskRequestID, accountId))
	taskCh, subId, err := controller.asyncTaskService.GetTaskWithRequestIdBlocking(taskRequestID, accountId)
	if err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask error: failed to get task (blocking), taskRequestId=%s, accountId=%d, error=%s", r.URL.Path, taskRequestID, accountId, err.Message))
		utils.SendJSON(w, err.Error(), false, err.Type, nil)
		return
	}
	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask retrieved task from blocking request, subscriberId=%d", r.URL.Path, subId))
	for {
		select {
		case task := <-taskCh:
			utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask success, status=200, returning task from channel, taskId=%d, state=%d", r.URL.Path, task.Id, task.State))
			utils.SendJSON(w, task, true, http.StatusOK, nil)
			return
		case <-r.Context().Done():
			utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask context done, cleaning up subscriber, taskRequestId=%s", r.URL.Path, taskRequestID))
			taskIdInt, err := controller.asyncTaskService.GetTaskIdWithRequestId(taskRequestID, accountId)
			if err != nil {
				utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask error: failed to get task ID for subscriber cleanup, taskRequestId=%s, error=%v", r.URL.Path, taskRequestID, err))
				return
			}
			delErr := controller.asyncTaskService.DeleteSubscriber(taskIdInt, subId)
			if taskCh != nil {
				close(taskCh)
			} else {
				utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask warning: attempted to close nil task channel, taskId=%d, taskRequestId=%s", r.URL.Path, taskIdInt, taskRequestID))
			}
			if delErr != nil {
				utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask error: failed to delete subscriber, taskId=%d, subscriberId=%d, error=%v", r.URL.Path, taskIdInt, subId, delErr))
				return
			}
			utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetTask subscriber cleanup completed, taskId=%d", r.URL.Path, taskIdInt))
			return
		}
	}

}
