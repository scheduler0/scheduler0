package controllers

import (
	"encoding/json"
	"github.com/hashicorp/raft"
	"io"
	"log"
	"net/http"
	"scheduler0/config"
	"scheduler0/headers"
	"scheduler0/models"
	"scheduler0/peers"
	"scheduler0/utils"
)

type PeerController interface {
	Handshake(w http.ResponseWriter, r *http.Request)
	ExecutionLogs(w http.ResponseWriter, r *http.Request)
}

type peerController struct {
	raft   *raft.Raft
	logger *log.Logger
	peer   *peers.Peer
}

func NewPeerController(logger *log.Logger, rft *raft.Raft, peer *peers.Peer) PeerController {
	return &peerController{
		raft:   rft,
		logger: logger,
		peer:   peer,
	}
}

func (controller *peerController) Handshake(w http.ResponseWriter, r *http.Request) {
	configs := config.GetScheduler0Configurations(controller.logger)

	res := peers.PeerRes{
		IsLeader: configs.Bootstrap == "true",
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

	jobsState := models.JobStateReqPayload{}
	err = json.Unmarshal(data, &jobsState)
	if err != nil {
		controller.logger.Println("failed to read data from execution logs request", err)
		utils.SendJSON(w, nil, true, http.StatusUnprocessableEntity, nil)
		return
	}

	go controller.peer.LogJobsStatePeers(r.Header.Get(headers.PeerAddressHeader), jobsState)

	utils.SendJSON(w, nil, true, http.StatusOK, nil)
	return
}
