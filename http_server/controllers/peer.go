package controllers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"scheduler0/peers"
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
	body, err := ioutil.ReadAll(r.Body)
	err = r.Body.Close()
	if err != nil {
		panic(err)
	}
	if err != nil {
		utils.SendJSON(w, "request body required", false, http.StatusUnprocessableEntity, nil)
		return
	}

	if len(body) < 1 {
		utils.SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}

	data := peers.PeerRequestBody{}

	err = json.Unmarshal(body, &data)
	if err != nil {
		utils.SendJSON(w, "request body required", false, http.StatusInternalServerError, nil)
		return
	}

	controller.peersManager.ReceivePeer(data.Peers, data.Version, peerAddress)

	utils.SendJSON(w, nil, true, http.StatusOK, nil)

	return
}
