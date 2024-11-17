package node

import (
	"context"
	"scheduler0/pkg/config"
	"scheduler0/pkg/models"
)

//go:generate mockery --name NodeClient --output ./ --inpackage
type NodeClient interface {
	FetchUncommittedLogsFromPeersPhase1(ctx context.Context, node *nodeService, peerFanIns []models.PeerFanIn)
	FetchUncommittedLogsFromPeersPhase2(ctx context.Context, node *nodeService, peerFanIns []models.PeerFanIn)
	ConnectNode(replica config.RaftNode) (*Status, error)
	StopJobs(ctx context.Context, node *nodeService, peer config.RaftNode) error
	StartJobs(ctx context.Context, node *nodeService, peer config.RaftNode) error
}
