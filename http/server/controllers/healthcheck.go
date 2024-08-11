package controllers

import (
	"log"
	"net/http"
	"scheduler0/fsm"
	"scheduler0/utils"
)

type HealthCheckController interface {
	HealthCheck(w http.ResponseWriter, r *http.Request)
}

type healthCheckController struct {
	fsmStore fsm.Scheduler0RaftStore
	logger   *log.Logger
}

type healthCheckRes struct {
	LeaderAddress string            `json:"leaderAddress"`
	LeaderId      string            `json:"leaderId"`
	RaftStats     map[string]string `json:"raftStats"`
}

func NewHealthCheckController(logger *log.Logger, fsmStore fsm.Scheduler0RaftStore) HealthCheckController {
	return &healthCheckController{
		fsmStore: fsmStore,
		logger:   logger,
	}
}

func (controller *healthCheckController) HealthCheck(w http.ResponseWriter, r *http.Request) {
	leaderAddress, leaderId := controller.fsmStore.GetRaft().LeaderWithID()
	res := healthCheckRes{
		LeaderAddress: string(leaderAddress),
		LeaderId:      string(leaderId),
		RaftStats:     controller.fsmStore.GetRaft().Stats(),
	}
	utils.SendJSON(w, res, true, http.StatusOK, nil)
}
