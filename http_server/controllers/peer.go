package controllers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"scheduler0/config"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/node"
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
	configs := config.GetConfigurations(controller.logger)

	res := node.Res{
		IsLeader: configs.Bootstrap,
	}

	utils.SendJSON(w, res, true, http.StatusOK, nil)
}

func (controller *peerController) ExecutionLogs(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		controller.logger.Println("failed to read data from execution logs request", err)
		utils.SendJSON(w, nil, true, http.StatusBadRequest, nil)
		return
	}

	jobsState := models.JobStateLog{}
	err = json.Unmarshal(data, &jobsState)
	if err != nil {
		controller.logger.Println("failed to read data from execution logs request", err)
		utils.SendJSON(w, nil, true, http.StatusUnprocessableEntity, nil)
		return
	}

	utils.SendJSON(w, nil, true, http.StatusOK, nil)
	return
}
