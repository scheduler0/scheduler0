package controllers

import (
	"fmt"
	"log"
	"net/http"
	"scheduler0/config"
	"scheduler0/fsm"
	"scheduler0/service"
	"scheduler0/utils"
)

type PeerController interface {
	Handshake(w http.ResponseWriter, r *http.Request)
	ExecutionLogs(w http.ResponseWriter, r *http.Request)
	StopJobs(w http.ResponseWriter, r *http.Request)
	StartJobs(w http.ResponseWriter, r *http.Request)
}

type peerController struct {
	scheduler0Config config.Scheduler0Config
	fsmStore         fsm.Scheduler0RaftStore
	logger           *log.Logger
	peer             *service.Node
}

func NewPeerController(logger *log.Logger, scheduler0Config config.Scheduler0Config, fsmStore fsm.Scheduler0RaftStore, peer *service.Node) PeerController {
	controller := peerController{
		scheduler0Config: scheduler0Config,
		fsmStore:         fsmStore,
		logger:           logger,
		peer:             peer,
	}
	return &controller
}

func (controller *peerController) Handshake(w http.ResponseWriter, r *http.Request) {
	configs := controller.scheduler0Config.GetConfigurations()

	res := service.Res{
		IsLeader: configs.Bootstrap,
	}

	utils.SendJSON(w, res, true, http.StatusOK, nil)
}

func (controller *peerController) ExecutionLogs(w http.ResponseWriter, r *http.Request) {
	requestId := r.Context().Value("RequestID")

	controller.peer.GetUncommittedLogs(requestId.(string))

	w.Header().Set("Location", fmt.Sprintf("/v1/async-tasks/%s", requestId))

	utils.SendJSON(w, nil, true, http.StatusAccepted, nil)
	return
}

func (controller *peerController) StopJobs(w http.ResponseWriter, r *http.Request) {
	controller.peer.StopJobs()
	utils.SendJSON(w, nil, true, http.StatusAccepted, nil)
	return
}

func (controller *peerController) StartJobs(w http.ResponseWriter, r *http.Request) {
	controller.peer.StartJobs()
	utils.SendJSON(w, nil, true, http.StatusAccepted, nil)
	return
}
