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
	fsmStore *fsm.Store
	logger   *log.Logger
}

type healthCheckRes struct {
	LeaderAddress string            `json:"leaderAddress"`
	LeaderId      string            `json:"leaderId"`
	RaftStats     map[string]string `json:"raftStats"`
}

func NewHealthCheckController(logger *log.Logger, fsmStore *fsm.Store) HealthCheckController {
	return &healthCheckController{
		fsmStore: fsmStore,
		logger:   logger,
	}
}

func (controller *healthCheckController) HealthCheck(w http.ResponseWriter, r *http.Request) {
	leaderAddress, leaderId := controller.fsmStore.Raft.LeaderWithID()
	res := healthCheckRes{
		LeaderAddress: string(leaderAddress),
		LeaderId:      string(leaderId),
		RaftStats:     controller.fsmStore.Raft.Stats(),
	}
	utils.SendJSON(w, res, true, http.StatusOK, nil)
}
