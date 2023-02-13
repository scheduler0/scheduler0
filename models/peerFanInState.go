package models

type PeerFanInState uint64

const (
	PeerFanInStateNotStated         PeerFanInState = 0
	PeerFanInStateGetRequestId                     = 1
	PeerFanInStateGetExecutionsLogs                = 2
	PeerFanInStateComplete                         = 3
)

type PeerFanIn struct {
	PeerHTTPAddress string            `json:"peerHTTPAddress"`
	RequestId       string            `json:"requestId"`
	State           PeerFanInState    `json:"state"`
	ExecutionLogs   []JobExecutionLog `json:"executionLogs"`
}
