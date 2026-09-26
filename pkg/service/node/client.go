package node

import (
	"context"
	"scheduler0/pkg/config"
	"scheduler0/pkg/models"
)

type Client interface {
	FetchUncommittedLogsFromPeersPhase1(ctx context.Context, node *nodeService, peerFanIns []models.PeerFanIn)
	FetchUncommittedLogsFromPeersPhase2(ctx context.Context, node *nodeService, peerFanIns []models.PeerFanIn)
	ConnectNode(replica config.RaftNode) (*Status, error)
	StopJobs(ctx context.Context, node *nodeService, peer config.RaftNode) error
	StartJobs(ctx context.Context, node *nodeService, peer config.RaftNode) error
	SendQuotaAllocation(ctx context.Context, peer config.RaftNode, accountAllocations map[uint64]uint64) error
	RequestLocalQuotaAllocations(ctx context.Context, peer config.RaftNode) (map[uint64]uint64, error)
	NotifyAccountExhaustion(ctx context.Context, leader config.RaftNode, accountId uint64) error
	UpdateJobOnLeader(ctx context.Context, leader config.RaftNode, job models.Job) error
}
