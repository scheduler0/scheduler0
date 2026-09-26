package models

type PeerFanInState uint64

const (
	PeerFanInStateNotStated         PeerFanInState = 0
	PeerFanInStateGetRequestId                     = 1
	PeerFanInStateGetExecutionsLogs                = 2
	PeerFanInStateComplete                         = 3
)

type PeerFanIn struct {
	PeerNodeAddress string         `json:"peerNodeAddress"`
	RequestId       string         `json:"requestId"`
	State           PeerFanInState `json:"state"`
	Data            LocalData      `json:"data"`
}
