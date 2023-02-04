package controllers

import (
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"scheduler0/fsm"
	"scheduler0/service/async_task_manager"
	"scheduler0/utils"
)

type AsyncTaskController interface {
	GetTask(w http.ResponseWriter, r *http.Request)
}

type asyncTaskController struct {
	fsmStore         *fsm.Store
	logger           *log.Logger
	asyncTaskService *async_task_manager.AsyncTaskManager
}

func NewAsyncTaskController(logger *log.Logger, fsmStore *fsm.Store, asyncTaskService *async_task_manager.AsyncTaskManager) AsyncTaskController {
	controller := asyncTaskController{
		fsmStore:         fsmStore,
		logger:           logger,
		asyncTaskService: asyncTaskService,
	}
	return &controller
}

func (controller *asyncTaskController) GetTask(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	taskID := params["id"]

	task, err := controller.asyncTaskService.GetTaskWithRequestId(taskID)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, err.Type, nil)
		return
	}

	utils.SendJSON(w, task, true, http.StatusOK, nil)
	return
}
