package controllers

import (
	"net/http"
	"scheduler0/server/peers"
	"scheduler0/utils"
)

type PeerController interface {
	PeerConnect(w http.ResponseWriter, r *http.Request)
}

type peerController struct {
	peersManager peers.PeerManager
}

func NewPeerControllerController(peersManager peers.PeerManager) PeerController {
	return &peerController{
		peersManager: peersManager,
	}
}

func (controller *peerController) PeerConnect(w http.ResponseWriter, r *http.Request) {
	peerAddress := r.Header.Get("peer-address")
	controller.peersManager.MarkConnected(peerAddress)
	utils.Info("peer connection request from ", peerAddress)
	utils.SendJSON(w, nil, true, http.StatusOK, nil)
	return
}
