package controllers

import (
	"cron-server/server/models"
	"cron-server/server/migrations"
	"net/http"
)

var basicExecutionController = BasicController{model: models.Execution{}}

type ExecutionController struct {
	Pool migrations.Pool
}

func (cc *ExecutionController) GetOne(w http.ResponseWriter, r *http.Request) {
	basicExecutionController.GetOne(w, r, cc.Pool)
}

func (cc *ExecutionController) GetAll(w http.ResponseWriter, r *http.Request) {
	basicExecutionController.GetAll(w, r, cc.Pool)
}
