package controllers

import (
	"cron-server/server/models"
	"cron-server/server/repository"
	"net/http"
)

var basicExecutionController = BasicController{model: models.Execution{}}

type ExecutionController struct {
	Pool repository.Pool
}

func (cc *ExecutionController) GetOne(w http.ResponseWriter, r *http.Request) {
	basicExecutionController.GetOne(w, r, cc.Pool)
}

func (cc *ExecutionController) GetAll(w http.ResponseWriter, r *http.Request) {
	basicExecutionController.GetAll(w, r, cc.Pool)
}
