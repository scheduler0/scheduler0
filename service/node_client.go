package service

import (
	"context"
	"scheduler0/config"
	"scheduler0/models"
)

//go:generate mockery --name NodeClient --output ../mocks --inpackage
type NodeClient interface {
	FetchUncommittedLogsFromPeersPhase1(ctx context.Context, node *Node, peerFanIns []models.PeerFanIn)
	FetchUncommittedLogsFromPeersPhase2(ctx context.Context, node *Node, peerFanIns []models.PeerFanIn)
	ConnectNode(replica config.RaftNode) (*Status, error)
}
