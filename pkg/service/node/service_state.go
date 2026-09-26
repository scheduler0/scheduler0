package node

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"scheduler0/pkg/config"
	"scheduler0/pkg/models"
)

type ServiceState interface {
	CanAcceptClientWriteRequest() bool
	CanAcceptRequest() bool
	StopJobs()
	StartJobs()
	BeginAcceptingClientWriteRequest()
	StopAcceptingClientWriteRequest()
	BeginAcceptingClientRequest()
	UpdateLocalQuotaAllocations(accountAllocations map[uint64]uint64) error
	ResetLocalQuotaAllocations()
	GetLocalQuotaAllocations() map[uint64]uint64
	NotifyLeaderAccountExhaustion(accountId uint64) error
	UpdateJobsStatusByAccountId(accountId uint64, status string) error
	UpdateJobOnLeader(job models.Job) error
}

type serviceState struct {
	node *nodeService
}

func newServiceState(node *nodeService) *serviceState {
	return &serviceState{
		node: node,
	}
}

func (s *serviceState) CanAcceptClientWriteRequest() bool {
	return s.node.acceptClientWrites
}

func (s *serviceState) CanAcceptRequest() bool {
	return s.node.acceptRequest
}

func (s *serviceState) StopJobs() {
	s.node.jobExecutor.StopAll()
}

func (s *serviceState) StartJobs() {
	s.node.jobProcessor.RecoverJobs()
}

func (s *serviceState) BeginAcceptingClientWriteRequest() {
	s.node.acceptClientWrites = true
	s.node.logger.Info("ready to accept write requests")
}

func (s *serviceState) StopAcceptingClientWriteRequest() {
	s.node.acceptClientWrites = false
	s.node.logger.Info("stopped accepting httpClient write requests")
}

func (s *serviceState) BeginAcceptingClientRequest() {
	s.node.acceptRequest = true
	s.node.logger.Info("being accepting httpClient requests")
}

func (s *serviceState) UpdateLocalQuotaAllocations(accountAllocations map[uint64]uint64) error {
	s.node.jobExecutor.UpdateLocalQuotaAllocations(accountAllocations)
	s.node.logger.Debug("forwarded quota allocations to executor", "accountCount", len(accountAllocations))
	return nil
}

func (s *serviceState) ResetLocalQuotaAllocations() {
	s.node.jobExecutor.ResetLocalQuotaAllocations()
	s.node.logger.Info("reset local quota allocations in executor")
}

func (s *serviceState) GetLocalQuotaAllocations() map[uint64]uint64 {
	return s.node.jobExecutor.GetAllLocalQuotaAllocations()
}

func (s *serviceState) NotifyLeaderAccountExhaustion(accountId uint64) error {
	// Get leader node info from raft
	_, leaderServerID := s.node.GetRaftLeaderWithId()
	if leaderServerID == "" {
		s.node.logger.Warn("no leader found, cannot notify account exhaustion", "accountId", accountId)
		return fmt.Errorf("no leader found")
	}

	// Convert leader server ID to node ID
	leaderNodeId, err := strconv.ParseUint(string(leaderServerID), 10, 64)
	if err != nil {
		s.node.logger.Error("failed to parse leader node ID", "error", err, "leaderServerID", string(leaderServerID))
		return fmt.Errorf("failed to parse leader node ID: %w", err)
	}

	// Get leader peer info from etcd
	var leaderPeer config.RaftNode
	if s.node.etcdService != nil {
		peers, err := s.node.etcdService.GetPeers(s.node.ctx)
		if err != nil {
			s.node.logger.Error("failed to get peers from etcd", "error", err)
			return fmt.Errorf("failed to get peers from etcd: %w", err)
		}

		// Find the leader peer
		found := false
		for _, peer := range peers {
			if peer.NodeId == leaderNodeId {
				leaderPeer = peer
				found = true
				break
			}
		}

		if !found {
			s.node.logger.Error("leader peer not found in etcd", "leaderNodeId", leaderNodeId)
			return fmt.Errorf("leader peer not found in etcd")
		}
	} else {
		s.node.logger.Error("etcd service not available, cannot find leader peer")
		return fmt.Errorf("etcd service not available")
	}

	// Call client's NotifyAccountExhaustion method
	ctx, cancel := context.WithTimeout(s.node.ctx, 30*time.Second)
	defer cancel()

	err = s.node.client.NotifyAccountExhaustion(ctx, leaderPeer, accountId)
	if err != nil {
		s.node.logger.Error("failed to notify leader of account exhaustion", "error", err, "accountId", accountId, "leaderAddress", leaderPeer.NodeAddress)
		return fmt.Errorf("failed to notify leader: %w", err)
	}

	s.node.logger.Info("successfully notified leader of account exhaustion", "accountId", accountId, "leaderAddress", leaderPeer.NodeAddress)
	return nil
}

func (s *serviceState) UpdateJobsStatusByAccountId(accountId uint64, status string) error {
	err := s.node.jobRepo.UpdateJobsStatusByAccountId(accountId, status)
	if err != nil {
		s.node.logger.Error("failed to update jobs status", "error", err, "accountId", accountId, "status", status)
		return err
	}

	s.node.logger.Info("successfully updated jobs status", "accountId", accountId, "status", status)
	return nil
}

func (s *serviceState) UpdateJobOnLeader(job models.Job) error {
	// Get leader node info from raft
	_, leaderServerID := s.node.GetRaftLeaderWithId()
	if leaderServerID == "" {
		s.node.logger.Warn("no leader found, cannot update job", "jobId", job.ID)
		return fmt.Errorf("no leader found")
	}

	// Convert leader server ID to node ID
	leaderNodeId, err := strconv.ParseUint(string(leaderServerID), 10, 64)
	if err != nil {
		s.node.logger.Error("failed to parse leader node ID", "error", err, "leaderServerID", string(leaderServerID))
		return fmt.Errorf("failed to parse leader node ID: %w", err)
	}

	// Get leader peer info from etcd
	var leaderPeer config.RaftNode
	if s.node.etcdService != nil {
		peers, err := s.node.etcdService.GetPeers(s.node.ctx)
		if err != nil {
			s.node.logger.Error("failed to get peers from etcd", "error", err)
			return fmt.Errorf("failed to get peers from etcd: %w", err)
		}

		// Find the leader peer
		found := false
		for _, peer := range peers {
			if peer.NodeId == leaderNodeId {
				leaderPeer = peer
				found = true
				break
			}
		}

		if !found {
			s.node.logger.Error("leader peer not found in etcd", "leaderNodeId", leaderNodeId)
			return fmt.Errorf("leader peer not found in etcd")
		}
	} else {
		s.node.logger.Error("etcd service not available, cannot find leader peer")
		return fmt.Errorf("etcd service not available")
	}

	// Call client's UpdateJobOnLeader method
	ctx, cancel := context.WithTimeout(s.node.ctx, 30*time.Second)
	defer cancel()

	err = s.node.client.UpdateJobOnLeader(ctx, leaderPeer, job)
	if err != nil {
		s.node.logger.Error("failed to update job on leader", "error", err, "jobId", job.ID, "leaderAddress", leaderPeer.NodeAddress)
		return fmt.Errorf("failed to update job on leader: %w", err)
	}

	s.node.logger.Info("successfully updated job on leader", "jobId", job.ID, "leaderAddress", leaderPeer.NodeAddress)
	return nil
}
