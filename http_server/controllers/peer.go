package controllers

import (
	"github.com/hashicorp/raft"
	"log"
	"net/http"
	"scheduler0/peers"
	"scheduler0/utils"
)

type PeerController interface {
	Handshake(w http.ResponseWriter, r *http.Request)
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
	configs := utils.GetScheduler0Configurations(controller.logger)
	res := peers.PeerRes{
		IsLeader: configs.Bootstrap == "true",
	}
	peerAddress := r.Header.Get(peers.PeerAddressHeader)

	controller.peer.ReceiveHandshakePeer(peerAddress)

	utils.SendJSON(w, res, true, http.StatusOK, nil)
}
