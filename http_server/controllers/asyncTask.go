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
	requestID := params["id"]

	taskCh, subId, err := controller.asyncTaskService.GetTaskWithRequestId(requestID)
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
				controller.logger.Println("failed to delete subscriber for task %s: could not convert taskid str to int", taskIdInt)
				return
			}
			delErr := controller.asyncTaskService.DeleteSubscriber(taskIdInt, subId)
			close(taskCh)
			if delErr != nil {
				controller.logger.Println("failed to delete subscriber for task %s: ", taskIdInt, delErr)
				return
			}
			controller.logger.Println("returning success")
			return
		}
	}

}
