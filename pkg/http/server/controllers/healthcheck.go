package controllers

import (
	"fmt"
	"log"
	"net/http"
	"scheduler0/pkg/service/node"
	"scheduler0/pkg/utils"
)

type HealthCheckController interface {
	HealthCheck(w http.ResponseWriter, r *http.Request)
}

type healthCheckController struct {
	service node.NodeService
	logger  *log.Logger
}

type healthCheckRes struct {
	LeaderAddress string            `json:"leaderAddress"`
	LeaderId      string            `json:"leaderId"`
	RaftStats     map[string]string `json:"raftStats"`
}

func NewHealthCheckController(logger *log.Logger, service node.NodeService) HealthCheckController {
	return &healthCheckController{
		service: service,
		logger:  logger,
	}
}

func (controller *healthCheckController) HealthCheck(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - HealthCheck entry", r.URL.Path))

	leaderAddress, leaderId := controller.service.GetRaftLeaderWithId()
	res := healthCheckRes{
		LeaderAddress: string(leaderAddress),
		LeaderId:      string(leaderId),
		RaftStats:     controller.service.GetRaftStats(),
	}
	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - HealthCheck success, status=200, leaderId=%s, leaderAddress=%s", r.URL.Path, string(leaderId), string(leaderAddress)))
	utils.SendJSON(w, res, true, http.StatusOK, nil)
}
