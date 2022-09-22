package controllers

import (
	"net/http"
	"scheduler0/utils"
)

type HealthCheckController interface {
	HealthCheck(w http.ResponseWriter, r *http.Request)
}

type healthCheckController struct{}

func NewHealthCheckController() HealthCheckController {
	return &healthCheckController{}
}

func (controller *healthCheckController) HealthCheck(w http.ResponseWriter, r *http.Request) {
	utils.SendJSON(w, nil, true, http.StatusOK, nil)
}
