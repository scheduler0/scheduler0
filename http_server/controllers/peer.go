package controllers

import (
	"fmt"
	"log"
	"net/http"
	"scheduler0/config"
	"scheduler0/fsm"
	"scheduler0/service/node"
	"scheduler0/utils"
)

type PeerController interface {
	Handshake(w http.ResponseWriter, r *http.Request)
	ExecutionLogs(w http.ResponseWriter, r *http.Request)
}

type peerController struct {
	fsmStore *fsm.Store
	logger   *log.Logger
	peer     *node.Node
}

func NewPeerController(logger *log.Logger, fsmStore *fsm.Store, peer *node.Node) PeerController {
	controller := peerController{
		fsmStore: fsmStore,
		logger:   logger,
		peer:     peer,
	}
	return &controller
}

func (controller *peerController) Handshake(w http.ResponseWriter, r *http.Request) {
	configs := config.GetConfigurations()

	res := node.Res{
		IsLeader: configs.Bootstrap,
	}

	utils.SendJSON(w, res, true, http.StatusOK, nil)
}

func (controller *peerController) ExecutionLogs(w http.ResponseWriter, r *http.Request) {
	requestId := r.Context().Value("RequestID")

	controller.peer.ReturnUncommittedLogs(requestId.(string))

	w.Header().Set("Location", fmt.Sprintf("/async-tasks/%s", requestId))

	utils.SendJSON(w, nil, true, http.StatusAccepted, nil)
	return
}
