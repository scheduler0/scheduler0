package peers

import (
	"fmt"
	"net/http"
	"scheduler0/server/http_server/middlewares/auth"
	"scheduler0/utils"
	"sync"
	"time"
)

type Peer struct {
	Address     string `json:"address" yaml:"Address"`
	ApiKey      string `json:"api_key" yaml:"ApiKey"`
	ApiSecret   string `json:"api_secret" yaml:"ApiSecret"`
	RaftAddress string `json:"raft_address" yaml:"RaftAddress"`
	Connected   bool
}

type peerManager struct {
	mtx     sync.Mutex
	peers   map[string]*Peer
	updates chan *Peer
}

type PeerManager interface {
	AddPeer(peer *Peer)
	RemovePeer(peer *Peer)
	ConnectPeers()
	ConnectPeer(p *Peer)
	MonitorPeers()
	MarkConnected(nodeAddress string)
	MarkNotConnected(nodeAddress string)
	IsConnected(nodeAddress string, isRaftAddr bool) bool
	UpdatesCh() chan *Peer
}

func NewPeersManager() *peerManager {
	return &peerManager{
		peers:   map[string]*Peer{},
		updates: make(chan *Peer, 1),
	}
}

func (pM *peerManager) AddPeer(peer *Peer) {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()
	pM.peers[peer.Address] = peer
}

func (pM *peerManager) RemovePeer(peer *Peer) {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()
	delete(pM.peers, peer.Address)
}

func (pM *peerManager) ConnectPeers() {
	configs := utils.GetScheduler0Configurations()

	for _, peer := range pM.peers {
		go func(p *Peer, pM *peerManager) {
			address := fmt.Sprintf("%s/peer", p.Address)
			req, err := http.NewRequest("POST", address, nil)
			if err != nil {
				utils.Error("peer connection error: request creation: %v", err.Error())
				return
			}

			req.Header.Add("peer-address", fmt.Sprintf("%v://%v:%v", configs.Protocol, configs.Host, configs.Port))
			req.Header.Add("api-key", p.ApiKey)
			req.Header.Add("api-secret", p.ApiSecret)

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				utils.Error("peer connection error: default client: %v", err.Error())
				return
			}

			if res.StatusCode == http.StatusOK {
				pM.MarkConnected(p.Address)
				utils.Info("successfully connect to peer ", p.Address)
				return
			} else {
				pM.MarkNotConnected(p.Address)
			}
		}(peer, pM)
	}
}

func (pM *peerManager) ConnectPeer(p *Peer) {
	configs := utils.GetScheduler0Configurations()

	address := fmt.Sprintf("%s/peer", p.Address)
	req, err := http.NewRequest("POST", address, nil)
	if err != nil {
		utils.Error("peer connection error: request creation: %v", err.Error())
		return
	}

	req.Header.Add("peer-address", fmt.Sprintf("%v://%v:%v", configs.Protocol, configs.Host, configs.Port))
	req.Header.Add(auth.APIKeyHeader, p.ApiKey)
	req.Header.Add(auth.SecretKeyHeader, p.ApiSecret)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		utils.Error("peer connection error: default client: ", err.Error())
		return
	}

	if res.StatusCode == http.StatusOK {
		pM.MarkConnected(p.Address)
		return
	} else {
		pM.MarkNotConnected(p.Address)
	}
}

func (pM *peerManager) MonitorPeers() {
	ticker := time.NewTicker(time.Millisecond * 500)
	for {
		select {
		case <-ticker.C:
			for _, peer := range pM.peers {
				go func(p *Peer, pM *peerManager) {
					address := fmt.Sprintf("%s/healthcheck", p.Address)
					req, err := http.NewRequest("GET", address, nil)
					if err != nil {
						utils.Error("peer connection error: request creation: ", err.Error())
						pM.MarkNotConnected(p.Address)
						return
					}

					res, err := http.DefaultClient.Do(req)
					if err != nil {
						utils.Error("peer connection error: default client: ", err.Error())
						pM.MarkNotConnected(p.Address)
						return
					}

					isHealthy := res.StatusCode == http.StatusOK

					if !pM.IsConnected(p.Address, false) && isHealthy {
						pM.ConnectPeer(p)
					}

					if !isHealthy {
						pM.MarkNotConnected(p.Address)
					}
				}(peer, pM)
			}
		}
	}
}

func (pM *peerManager) MarkConnected(nodeAddress string) {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()
	if node, ok := pM.peers[nodeAddress]; ok {
		node.Connected = true
		pM.updates <- node
		utils.Info("successfully connect to peer ", nodeAddress)
	}
}

func (pM *peerManager) MarkNotConnected(nodeAddress string) {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()
	if node, ok := pM.peers[nodeAddress]; ok {
		node.Connected = false
		pM.updates <- node
		utils.Error("failed to connect to peer ", nodeAddress)
	}
}

func (pM *peerManager) IsConnected(nodeAddress string, isRaftAddr bool) bool {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()

	for _, peer := range pM.peers {
		fmt.Println("isRaftAddr", isRaftAddr, "peer.RaftAddress", peer.RaftAddress, "nodeAddress", nodeAddress, "peer.Connected", peer.Connected)
		if isRaftAddr && peer.RaftAddress == nodeAddress && peer.Connected ||
			!isRaftAddr && peer.Address == nodeAddress && peer.Connected {
			return true
		}

		if !isRaftAddr && peer.Address == nodeAddress && !peer.Connected ||
			isRaftAddr && peer.RaftAddress == nodeAddress && !peer.Connected {
			return false
		}
	}

	return false
}

func (pM *peerManager) UpdatesCh() chan *Peer {
	return pM.updates
}
