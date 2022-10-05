package peers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"scheduler0/http_server/middlewares/auth"
	"scheduler0/utils"
	"strings"
	"sync"
	"time"
)

type Peer struct {
	Address           string `json:"address" yaml:"Address"`
	ApiKey            string `json:"api_key" yaml:"ApiKey"`
	ApiSecret         string `json:"api_secret" yaml:"ApiSecret"`
	RaftAddress       string `json:"raft_address" yaml:"RaftAddress"`
	Connected         bool
	Version           int64
	LastGossipVersion int64
}

type SlimPeer struct {
	Address   string `json:"address" yaml:"Address"`
	Connected bool
}

type PeerRequestBody struct {
	Version int64      `json:"version"`
	Peers   []SlimPeer `json:"peers"`
}

type peerManager struct {
	mtx       sync.Mutex
	canGossip bool
	version   int64
	peers     map[string]*Peer
	updates   chan *Peer
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
	ReceivePeer(peers []SlimPeer, version int64, peerAddress string)
	UpdateCanGossip(gossip bool)
}

const GossipInterval = time.Second * time.Duration(1)
const GossipFan0ut = 2

func NewPeersManager() *peerManager {
	return &peerManager{
		peers:     map[string]*Peer{},
		updates:   make(chan *Peer, 1),
		canGossip: false,
		version:   0,
	}
}

func (pM *peerManager) UpdateCanGossip(gossip bool) {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()

	pM.canGossip = gossip
}

func (pM *peerManager) UpdateVersion() {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()

	pM.version = pM.version + 1
}

func (pM *peerManager) UpdateLastSentVersion(address string, version int64) {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()

	if cp, ok := pM.peers[address]; ok {
		cp.LastGossipVersion = version
	}
}

func (pM *peerManager) AddPeer(peer *Peer) {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()
	pM.peers[peer.Address] = peer
	peer.Version = -1
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
			req.Header.Add(auth.APIKeyHeader, p.ApiKey)
			req.Header.Add(auth.SecretKeyHeader, p.ApiSecret)

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

func (pM *peerManager) getRandomPeersExcept(nodeAddress string) []Peer {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()

	peers := []Peer{}

	for _, p := range pM.peers {
		if p.Address != nodeAddress {
			peers = append(peers, *p)
		}
	}

	lastIndex := len(peers) - 1

	for lastIndex > 0 {
		randIndex := rand.Intn(lastIndex)
		tempP := peers[lastIndex]
		peers[lastIndex] = peers[randIndex]
		peers[randIndex] = tempP
		lastIndex -= 1
	}

	return peers[:GossipFan0ut]
}

func (pM *peerManager) MonitorPeers() {
	configs := utils.GetScheduler0Configurations()

	ticker := time.NewTicker(GossipInterval)
	for {
		select {
		case <-ticker.C:
			if pM.canGossip {
				randomPeers := pM.getRandomPeersExcept(fmt.Sprintf("%v://%v:%v", configs.Protocol, configs.Host, configs.Port))

				for _, peer := range randomPeers {
					go func(p Peer, pM *peerManager) {
						address := fmt.Sprintf("%s/peer", p.Address)

						peersM := []SlimPeer{}

						for _, pe := range pM.peers {
							peersM = append(peersM, SlimPeer{
								Address:   pe.Address,
								Connected: pe.Connected,
							})
						}

						peerRequestBody := PeerRequestBody{
							Version: pM.version,
							Peers:   peersM,
						}

						reqBody, err := json.Marshal(peerRequestBody)

						req, err := http.NewRequest(http.MethodPost, address, strings.NewReader(string(reqBody)))
						if err != nil {
							utils.Error("peer connection error: request creation: ", err.Error())
							return
						}

						req.Header.Add("peer-address", fmt.Sprintf("%v://%v:%v", configs.Protocol, configs.Host, configs.Port))
						req.Header.Add(auth.APIKeyHeader, p.ApiKey)
						req.Header.Add(auth.SecretKeyHeader, p.ApiSecret)

						var client = &http.Client{
							Transport: &http.Transport{},
						}

						res, err := client.Do(req)
						if err != nil {
							utils.Error("peer connection error: default client: ", err.Error())
							pM.MarkNotConnected(p.Address)
							return
						}

						req.Close = true

						isHealthy := res.StatusCode == http.StatusOK

						if !isHealthy {
							pM.MarkNotConnected(p.Address)
						}

						pM.UpdateLastSentVersion(p.Address, pM.version)
					}(peer, pM)
				}
			}

		}
	}
}

func (pM *peerManager) MarkConnected(nodeAddress string) {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()
	if node, ok := pM.peers[nodeAddress]; ok {

		if node.Connected {
			return
		}

		node.Connected = true
		pM.updates <- node
		utils.Info("successfully connect to peer ", nodeAddress)
	}
	pM.version += 1
}

func (pM *peerManager) MarkNotConnected(nodeAddress string) {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()
	if node, ok := pM.peers[nodeAddress]; ok {
		if !node.Connected {
			return
		}

		node.Connected = false
		pM.updates <- node
		utils.Error("failed to connect to peer ", nodeAddress)
	}
	pM.version += 1
}

func (pM *peerManager) IsConnected(nodeAddress string, isRaftAddr bool) bool {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()

	//fmt.Println("Number of peers", len(pM.peers))

	for _, peer := range pM.peers {
		//fmt.Println("isRaftAddr", isRaftAddr, "peer.RaftAddress", peer.RaftAddress, "nodeAddress", nodeAddress, "peer.Connected", peer.Connected)
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

func (pM *peerManager) ReceivePeer(peers []SlimPeer, version int64, peerAddress string) {
	pM.mtx.Lock()
	defer pM.mtx.Unlock()

	configs := utils.GetScheduler0Configurations()

	diff := 0

	if version >= pM.version {
		for _, p := range peers {
			if p.Address == fmt.Sprintf("%v://%v:%v", configs.Protocol, configs.Host, configs.Port) {
				continue
			}

			if p.Address == peerAddress && p.Connected == false {
				continue
			}

			if p.Address == peerAddress && p.Connected == true && pM.peers[peerAddress].Connected {
				continue
			}

			if p.Address == peerAddress && p.Connected == true && !pM.peers[peerAddress].Connected {
				diff += 1
				pM.peers[peerAddress].Connected = true
				pM.updates <- pM.peers[p.Address]
				continue
			}

			if cp, ok := pM.peers[p.Address]; ok && p.Connected != cp.Connected {
				cp.Connected = p.Connected
				pM.updates <- pM.peers[p.Address]
				diff += 1
			}
		}
	}

	if diff > 0 {
		pM.version = version + 1
	}

	if diff == 0 && version > pM.version {
		pM.version = version
	}

	if pM.canGossip == false {
		pM.canGossip = true
	}

	if cp, ok := pM.peers[peerAddress]; ok {
		cp.Version = version
	}
}
