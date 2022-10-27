package controllers

import (
	"github.com/hashicorp/raft"
	"log"
	"net/http"
	"scheduler0/utils"
)

type HealthCheckController interface {
	HealthCheck(w http.ResponseWriter, r *http.Request)
}

type healthCheckController struct {
	raft   *raft.Raft
	logger *log.Logger
}

type healthCheckRes struct {
	LeaderAddress string            `json:"leaderAddress"`
	LeaderId      string            `json:"leaderId"`
	RaftStats     map[string]string `json:"raftStats"`
}

func NewHealthCheckController(logger *log.Logger, rft *raft.Raft) HealthCheckController {
	return &healthCheckController{
		raft:   rft,
		logger: logger,
	}
}

func (controller *healthCheckController) HealthCheck(w http.ResponseWriter, r *http.Request) {
	leaderAddress, leaderId := controller.raft.LeaderWithID()
	res := healthCheckRes{
		LeaderAddress: string(leaderAddress),
		LeaderId:      string(leaderId),
		RaftStats:     controller.raft.Stats(),
	}
	utils.SendJSON(w, res, true, http.StatusOK, nil)
}
