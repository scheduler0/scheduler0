package controllers

import (
	"fmt"
	"log"
	"net/http"
	"scheduler0/pkg/config"
	"scheduler0/pkg/service/node"
	"scheduler0/pkg/utils"
)

type PeerController interface {
	Handshake(w http.ResponseWriter, r *http.Request)
	ExecutionLogs(w http.ResponseWriter, r *http.Request)
	StopJobs(w http.ResponseWriter, r *http.Request)
	StartJobs(w http.ResponseWriter, r *http.Request)
}

type peerController struct {
	scheduler0Config config.Scheduler0Config
	logger           *log.Logger
	peer             node.NodeService
}

func NewPeerController(logger *log.Logger, scheduler0Config config.Scheduler0Config, peer node.NodeService) PeerController {
	controller := peerController{
		scheduler0Config: scheduler0Config,
		logger:           logger,
		peer:             peer,
	}
	return &controller
}

func (controller *peerController) Handshake(w http.ResponseWriter, r *http.Request) {
	configs := controller.scheduler0Config.GetConfigurations()

	res := node.Res{
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
