package controllers

import (
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"scheduler0/pkg/models"
	"scheduler0/pkg/service/async_task"
	"scheduler0/pkg/utils"
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
	params := mux.Vars(r)
	requestID := params["id"]

	task, err := controller.asyncTaskService.GetTaskWithRequestIdNonBlocking(requestID)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, err.Type, nil)
		return
	}

	if task.State == models.AsyncTaskSuccess {
		utils.SendJSON(w, task, true, http.StatusOK, nil)
		return
	}

	taskCh, subId, err := controller.asyncTaskService.GetTaskWithRequestIdBlocking(requestID)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, err.Type, nil)
		return
	}

	for {
		select {
		case task := <-taskCh:
			controller.logger.Println("returning task from channel")
			utils.SendJSON(w, task, true, http.StatusOK, nil)
			return
		case <-r.Context().Done():
			taskIdInt, err := controller.asyncTaskService.GetTaskIdWithRequestId(requestID)
			if err != nil {
				controller.logger.Println(fmt.Sprintf("failed to delete subscriber for task %d: could not convert taskid str to int", taskIdInt))
				return
			}
			delErr := controller.asyncTaskService.DeleteSubscriber(taskIdInt, subId)
			if taskCh != nil {
				close(taskCh)
			}
			if delErr != nil {
				controller.logger.Println(fmt.Sprintf("failed to delete subscriber with id %d for task %s: ", taskIdInt, delErr.Error()))
				return
			}
			controller.logger.Println("returning success")
			return
		}
	}

}
