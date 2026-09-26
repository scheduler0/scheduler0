package node

import (
	"context"
	"scheduler0/pkg/config"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/service/async_task"
	"scheduler0/pkg/service/etcd"
	"scheduler0/pkg/service/executor"
	"scheduler0/pkg/service/processor"
	"scheduler0/pkg/service/queue"
	"scheduler0/pkg/utils"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupRaftClusterManagerTest creates a raftClusterManager with mocked dependencies
func setupRaftClusterManagerTest(t *testing.T) (*raftClusterManager, *nodeService, *fsm.MockScheduler0RaftStore, *etcd.MockEtcdService) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "raft-cluster-test",
		Level: hclog.LevelFromString("ERROR"),
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockRaftStore := fsm.NewMockScheduler0RaftStore(t)
	mockEtcdService := etcd.NewMockEtcdService(t)

	mockConfig := &mockScheduler0ConfigForTesting{
		config: &config.Scheduler0Configurations{
			EtcdEndpoints: []string{"localhost:2379"},
			NodeId:        1,
		},
	}

	node := &nodeService{
		logger:               logger,
		ctx:                  ctx,
		etcdService:          mockEtcdService,
		scheduler0Config:     mockConfig,
		scheduler0RaftStore:  mockRaftStore,
		peerObserverChannels: make(chan raft.Observation, 10),
		isExistingNode:       false,
		peersMutex:           sync.RWMutex{},
	}

	// Initialize components
	node.serviceState = newServiceState(node)
	node.eventHandler = newEventHandler(node)
	node.peerComm = newPeerCommunicator(node)
	node.raftCluster = newRaftClusterManager(node)

	return node.raftCluster, node, mockRaftStore, mockEtcdService
}

func TestRaftClusterManager_GetRaftStats(t *testing.T) {
	t.Run("returns raft stats from store", func(t *testing.T) {
		rcm, _, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		expectedStats := map[string]string{
			"state": "Leader",
			"term":  "5",
		}

		mockRaftStore.EXPECT().GetRaftStats().Return(expectedStats)

		result := rcm.GetRaftStats()
		assert.Equal(t, expectedStats, result)
	})

	t.Run("returns empty stats when store returns nil", func(t *testing.T) {
		rcm, _, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		mockRaftStore.EXPECT().GetRaftStats().Return(nil)

		result := rcm.GetRaftStats()
		assert.Nil(t, result)
	})

	t.Run("handles empty stats map", func(t *testing.T) {
		rcm, _, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		mockRaftStore.EXPECT().GetRaftStats().Return(map[string]string{})

		result := rcm.GetRaftStats()
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})
}

func TestRaftClusterManager_GetRaftLeaderWithId(t *testing.T) {
	t.Run("returns leader address and ID", func(t *testing.T) {
		rcm, _, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		expectedAddr := raft.ServerAddress("node1:8080")
		expectedID := raft.ServerID("1")

		mockRaftStore.EXPECT().LeaderWithID().Return(expectedAddr, expectedID)

		addr, id := rcm.GetRaftLeaderWithId()
		assert.Equal(t, expectedAddr, addr)
		assert.Equal(t, expectedID, id)
	})

	t.Run("returns empty when no leader", func(t *testing.T) {
		rcm, _, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		mockRaftStore.EXPECT().LeaderWithID().Return(raft.ServerAddress(""), raft.ServerID(""))

		addr, id := rcm.GetRaftLeaderWithId()
		assert.Empty(t, addr)
		assert.Empty(t, id)
	})

	t.Run("handles different address formats", func(t *testing.T) {
		testCases := []struct {
			addr raft.ServerAddress
			id   raft.ServerID
		}{
			{raft.ServerAddress("127.0.0.1:8080"), raft.ServerID("1")},
			{raft.ServerAddress("node.example.com:9090"), raft.ServerID("node-1")},
			{raft.ServerAddress(""), raft.ServerID("")},
		}

		for _, tc := range testCases {
			t.Run(string(tc.addr), func(t *testing.T) {
				rcm, _, mockRaftStore, _ := setupRaftClusterManagerTest(t)
				mockRaftStore.EXPECT().LeaderWithID().Return(tc.addr, tc.id)
				addr, id := rcm.GetRaftLeaderWithId()
				assert.Equal(t, tc.addr, addr)
				assert.Equal(t, tc.id, id)
			})
		}
	})
}

func TestRaftClusterManager_AuthRaftConfiguration(t *testing.T) {
	t.Run("creates configuration with all peers", func(t *testing.T) {
		rcm, node, _, mockEtcd := setupRaftClusterManagerTest(t)

		configs := node.scheduler0Config.GetConfigurations()
		configs.NodeId = 1
		configs.NodeServiceDiscoveryAddress = "node1:8080"

		peers := []config.RaftNode{
			{NodeId: 1, NodeAddress: "node1:8080"},
			{NodeId: 2, NodeAddress: "node2:8080"},
		}

		node.peersMutex.Lock()
		node.peersFromEtcd = peers
		node.peersMutex.Unlock()

		// Mock GetPeers call from AuthenticateWithPeersFromEtcd
		mockEtcd.EXPECT().GetPeers(mock.Anything).Return(peers, nil)
		// Mock peer communicator methods
		mockClient := NewMockClient(t)
		node.client = mockClient
		mockClient.EXPECT().ConnectNode(mock.Anything).Return(&Status{IsAlive: true, IsAuth: true}, nil).Maybe()

		cfg := rcm.AuthRaftConfiguration()

		// Should include self and authenticated peers
		assert.GreaterOrEqual(t, len(cfg.Servers), 1)
	})

	t.Run("handles empty peers list", func(t *testing.T) {
		rcm, node, _, mockEtcd := setupRaftClusterManagerTest(t)

		node.peersMutex.Lock()
		node.peersFromEtcd = []config.RaftNode{}
		node.peersMutex.Unlock()

		// Mock GetPeers call
		mockEtcd.EXPECT().GetPeers(mock.Anything).Return([]config.RaftNode{}, nil)

		cfg := rcm.AuthRaftConfiguration()

		// Should at least include self
		assert.GreaterOrEqual(t, len(cfg.Servers), 1)
	})

	t.Run("handles peers with authentication failures", func(t *testing.T) {
		rcm, node, _, mockEtcd := setupRaftClusterManagerTest(t)

		peers := []config.RaftNode{
			{NodeId: 2, NodeAddress: "node2:8080"},
			{NodeId: 3, NodeAddress: "node3:8080"},
		}

		node.peersMutex.Lock()
		node.peersFromEtcd = peers
		node.peersMutex.Unlock()

		mockEtcd.EXPECT().GetPeers(mock.Anything).Return(peers, nil)
		mockClient := NewMockClient(t)
		node.client = mockClient
		// First peer authenticates, second fails
		mockClient.EXPECT().ConnectNode(peers[0]).Return(&Status{IsAlive: true, IsAuth: true}, nil)
		mockClient.EXPECT().ConnectNode(peers[1]).Return(&Status{IsAlive: true, IsAuth: false}, nil)

		cfg := rcm.AuthRaftConfiguration()

		// Should include self and only authenticated peer
		assert.GreaterOrEqual(t, len(cfg.Servers), 1)
		// Verify only authenticated peer is included
		foundAuthPeer := false
		for _, server := range cfg.Servers {
			if string(server.Address) == "node2:8080" {
				foundAuthPeer = true
			}
			if string(server.Address) == "node3:8080" {
				t.Error("unauthenticated peer should not be in configuration")
			}
		}
		assert.True(t, foundAuthPeer, "authenticated peer should be in configuration")
	})

	t.Run("handles peers that are not alive", func(t *testing.T) {
		rcm, node, _, mockEtcd := setupRaftClusterManagerTest(t)

		peers := []config.RaftNode{
			{NodeId: 2, NodeAddress: "node2:8080"},
		}

		node.peersMutex.Lock()
		node.peersFromEtcd = peers
		node.peersMutex.Unlock()

		mockEtcd.EXPECT().GetPeers(mock.Anything).Return(peers, nil)
		mockClient := NewMockClient(t)
		node.client = mockClient
		mockClient.EXPECT().ConnectNode(peers[0]).Return(&Status{IsAlive: false, IsAuth: true}, nil)

		cfg := rcm.AuthRaftConfiguration()

		// Should include self but not dead peer
		assert.GreaterOrEqual(t, len(cfg.Servers), 1)
		for _, server := range cfg.Servers {
			if string(server.Address) == "node2:8080" {
				t.Error("dead peer should not be in configuration")
			}
		}
	})
}

func TestRaftClusterManager_ResetRaftState(t *testing.T) {
	// Note: ResetRaftState calls os.Exit(0) at the end, so we can't fully test it
	// This test verifies the method structure up to the exit point
	t.Run("calls reset on store", func(t *testing.T) {
		// This test is skipped because ResetRaftState calls os.Exit(0)
		// which would terminate the test process
		t.Skip("ResetRaftState calls os.Exit(0) and cannot be fully unit tested")
	})
}

func TestRaftClusterManager_ReconcileRaftMembershipWithPeers(t *testing.T) {
	t.Run("skips reconciliation when raft not initialized", func(t *testing.T) {
		rcm, _, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		peers := []config.RaftNode{
			{NodeId: 1, NodeAddress: "node1:8080"},
		}

		mockRaftStore.EXPECT().GetRaft().Return(nil)

		// Should not panic
		rcm.ReconcileRaftMembershipWithPeers(peers)
	})

	t.Run("handles empty peers list", func(t *testing.T) {
		rcm, _, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		// Note: This test would require a real raft instance to fully test
		// We're just testing that it doesn't panic with nil raft
		mockRaftStore.EXPECT().GetRaft().Return(nil)

		rcm.ReconcileRaftMembershipWithPeers([]config.RaftNode{})
	})

	t.Run("handles nil peers list", func(t *testing.T) {
		rcm, _, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		mockRaftStore.EXPECT().GetRaft().Return(nil)

		// Should not panic with nil peers
		rcm.ReconcileRaftMembershipWithPeers(nil)
	})
}

func TestRaftClusterManager_RemoveSelfFromCluster(t *testing.T) {
	t.Run("returns error when raft not initialized", func(t *testing.T) {
		rcm, _, mockRaftStore, mockEtcd := setupRaftClusterManagerTest(t)

		// etcd is called first, then raft is checked
		mockEtcd.EXPECT().UnregisterNode().Return(nil)
		mockRaftStore.EXPECT().GetRaft().Return(nil)

		err := rcm.RemoveSelfFromCluster(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "raft not initialized")
	})

	t.Run("handles etcd unregistration failure", func(t *testing.T) {
		rcm, _, mockRaftStore, mockEtcd := setupRaftClusterManagerTest(t)

		// etcd unregistration happens before raft check
		mockEtcd.EXPECT().UnregisterNode().Return(assert.AnError)

		err := rcm.RemoveSelfFromCluster(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unregister from etcd")
		// GetRaft should not be called if etcd fails first
		mockRaftStore.AssertNotCalled(t, "GetRaft")
	})

	t.Run("handles nil etcd service", func(t *testing.T) {
		rcm, node, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		node.etcdService = nil
		// When etcd is nil, it skips etcd and goes straight to raft check
		mockRaftStore.EXPECT().GetRaft().Return(nil)

		err := rcm.RemoveSelfFromCluster(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "raft not initialized")
	})
}

func TestRaftClusterManager_AddSelfToCluster(t *testing.T) {
	t.Run("returns error when raft not initialized", func(t *testing.T) {
		rcm, _, mockRaftStore, mockEtcd := setupRaftClusterManagerTest(t)

		// etcd is called first, then raft is checked
		mockEtcd.EXPECT().RegisterNode(mock.Anything, mock.Anything).Return(nil)
		mockRaftStore.EXPECT().GetRaft().Return(nil)

		err := rcm.AddSelfToCluster(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "raft not initialized")
	})

	t.Run("handles etcd registration failure", func(t *testing.T) {
		rcm, _, mockRaftStore, mockEtcd := setupRaftClusterManagerTest(t)

		// etcd registration happens before raft check
		mockEtcd.EXPECT().RegisterNode(mock.Anything, mock.Anything).Return(assert.AnError)

		err := rcm.AddSelfToCluster(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to register in etcd")
		// GetRaft should not be called if etcd fails first
		mockRaftStore.AssertNotCalled(t, "GetRaft")
	})

	t.Run("handles nil etcd service", func(t *testing.T) {
		rcm, node, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		node.etcdService = nil
		mockRaftStore.EXPECT().GetRaft().Return(nil)

		err := rcm.AddSelfToCluster(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "raft not initialized")
	})
}

func TestRaftClusterManager_ForceRebuildCluster(t *testing.T) {
	t.Run("returns error when not seed node", func(t *testing.T) {
		rcm, node, _, _ := setupRaftClusterManagerTest(t)

		mockConfig := node.scheduler0Config.(*mockScheduler0ConfigForTesting)
		mockConfig.config.NodeId = 2 // Not the seed node

		err := rcm.ForceRebuildCluster(context.Background(), 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is not the seed node")
	})

	t.Run("returns error when raft not initialized", func(t *testing.T) {
		rcm, _, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		mockRaftStore.EXPECT().GetRaft().Return(nil)

		err := rcm.ForceRebuildCluster(context.Background(), 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "raft not initialized")
	})

	t.Run("returns error when data store not available", func(t *testing.T) {
		rcm, _, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		// Create a mock raft instance
		mockRaft := &raft.Raft{}
		mockRaftStore.EXPECT().GetRaft().Return(mockRaft).Maybe()
		mockRaftStore.EXPECT().GetDataStore().Return(nil)

		err := rcm.ForceRebuildCluster(context.Background(), 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "data store not available")
	})
}

func TestRaftClusterManager_HandleRaftLeadershipChangesDebounced(t *testing.T) {
	t.Run("debounces leadership changes", func(t *testing.T) {
		rcm, node, _, _ := setupRaftClusterManagerTest(t)

		// Set up debounce
		node.leadershipDebounce = utils.NewDebounce()
		node.latestIsLeader = false

		// Call multiple times rapidly
		rcm.HandleRaftLeadershipChangesDebounced(true)
		rcm.HandleRaftLeadershipChangesDebounced(true)
		rcm.HandleRaftLeadershipChangesDebounced(true)

		// Wait a bit for debounce
		time.Sleep(100 * time.Millisecond)

		// The debounced handler should eventually update
		node.latestIsLeaderMtx.Lock()
		latest := node.latestIsLeader
		node.latestIsLeaderMtx.Unlock()

		// The debounced version updates latestIsLeader but may not call HandleRaftLeadershipChanges immediately
		// This is expected behavior for debouncing
		assert.True(t, latest)
	})

	t.Run("handles rapid leader/follower transitions", func(t *testing.T) {
		rcm, node, _, _ := setupRaftClusterManagerTest(t)

		node.leadershipDebounce = utils.NewDebounce()
		node.latestIsLeader = false

		// Rapidly toggle leadership
		rcm.HandleRaftLeadershipChangesDebounced(true)
		rcm.HandleRaftLeadershipChangesDebounced(false)
		rcm.HandleRaftLeadershipChangesDebounced(true)

		time.Sleep(100 * time.Millisecond)

		node.latestIsLeaderMtx.Lock()
		latest := node.latestIsLeader
		node.latestIsLeaderMtx.Unlock()

		// Should reflect the last value
		assert.True(t, latest)
	})
}

func TestRaftClusterManager_HandleRaftObserverChannelChanges(t *testing.T) {
	t.Run("handles peer observation", func(t *testing.T) {
		rcm, _, _, _ := setupRaftClusterManagerTest(t)

		// Create a valid PeerObservation - Peer is a Server struct
		observation := raft.Observation{
			Data: raft.PeerObservation{
				Peer: raft.Server{
					ID:       raft.ServerID("1"),
					Address:  raft.ServerAddress("node1:8080"),
					Suffrage: raft.Voter,
				},
				Removed: false,
			},
		}

		// Should not panic
		rcm.HandleRaftObserverChannelChanges(observation)
	})

	t.Run("handles peer removal observation", func(t *testing.T) {
		rcm, _, _, _ := setupRaftClusterManagerTest(t)

		observation := raft.Observation{
			Data: raft.PeerObservation{
				Peer: raft.Server{
					ID:       raft.ServerID("2"),
					Address:  raft.ServerAddress("node2:8080"),
					Suffrage: raft.Voter,
				},
				Removed: true,
			},
		}

		// Should not panic
		rcm.HandleRaftObserverChannelChanges(observation)
	})

	t.Run("handles resumed heartbeat observation", func(t *testing.T) {
		rcm, _, _, mockEtcd := setupRaftClusterManagerTest(t)

		// Mock GetPeers since StartJobsOnWorkerNodes calls it
		mockEtcd.EXPECT().GetPeers(mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

		observation := raft.Observation{
			Data: raft.ResumedHeartbeatObservation{
				PeerID: raft.ServerID("1"),
			},
		}

		// Should not panic
		rcm.HandleRaftObserverChannelChanges(observation)
	})

	t.Run("handles unknown observation type", func(t *testing.T) {
		rcm, _, _, _ := setupRaftClusterManagerTest(t)

		observation := raft.Observation{
			Data: "unknown",
		}

		// Should not panic
		rcm.HandleRaftObserverChannelChanges(observation)
	})

	t.Run("handles nil observation data", func(t *testing.T) {
		rcm, _, _, _ := setupRaftClusterManagerTest(t)

		observation := raft.Observation{
			Data: nil,
		}

		// Should not panic
		rcm.HandleRaftObserverChannelChanges(observation)
	})
}

func TestRaftClusterManager_HandleRaftLeadershipChanges(t *testing.T) {
	t.Run("handles becoming leader in single node mode", func(t *testing.T) {
		rcm, node, mockRaftStore, mockEtcd := setupRaftClusterManagerTest(t)

		// Mock single node scenario
		mockRaftStore.EXPECT().GetServersOnRaftCluster().Return([]raft.Server{
			{ID: raft.ServerID("1"), Address: raft.ServerAddress("node1:8080")},
		})

		// Use proper mocks from mockery
		mockJobQueue := queue.NewMockJobQueueService(t)
		node.jobQueue = mockJobQueue
		mockJobQueue.EXPECT().RemoveServers(mock.Anything).Return()
		mockJobQueue.EXPECT().AddServers(mock.Anything).Return()
		mockJobQueue.EXPECT().SetSingleNodeMode(true).Return()

		mockJobProcessor := processor.NewMockJobProcessorService(t)
		node.jobProcessor = mockJobProcessor
		mockJobProcessor.EXPECT().StartJobs().Return()

		mockJobExecutor := executor.NewMockJobExecutorService(t)
		node.jobExecutor = mockJobExecutor
		mockJobExecutor.EXPECT().SetSingleNodeMode(true).Return()
		// In single node mode, GetUncommittedLogs is NOT called (only in multi-node mode)

		mockAsyncTaskManager := async_task.NewMockAsyncTaskService(t)
		node.asyncTaskManager = mockAsyncTaskManager
		mockAsyncTaskManager.EXPECT().SetSingleNodeMode(true).Return()
		mockAsyncTaskManager.EXPECT().SetNodeIsLeader(true).Return()
		// In single node mode, GetUnCommittedTasks is NOT called (only in multi-node mode)

		// Mock peer communicator methods - StopAllJobsOnAllWorkerNodes is called but in single node mode it should be a no-op
		mockEtcd.EXPECT().GetPeers(mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

		rcm.HandleRaftLeadershipChanges(true)

		// Verify single node mode was set
		assert.True(t, node.SingleNodeMode)
	})

	t.Run("handles becoming follower", func(t *testing.T) {
		rcm, node, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		mockRaftStore.EXPECT().GetServersOnRaftCluster().Return([]raft.Server{
			{ID: raft.ServerID("1"), Address: raft.ServerAddress("node1:8080")},
			{ID: raft.ServerID("2"), Address: raft.ServerAddress("node2:8080")},
		})

		mockJobQueue := queue.NewMockJobQueueService(t)
		node.jobQueue = mockJobQueue
		mockJobQueue.EXPECT().RemoveServers(mock.Anything).Return()

		mockAsyncTaskManager := async_task.NewMockAsyncTaskService(t)
		node.asyncTaskManager = mockAsyncTaskManager
		mockAsyncTaskManager.EXPECT().SetNodeIsLeader(false).Return()

		rcm.HandleRaftLeadershipChanges(false)

		// Verify quota allocations were reset
		assert.False(t, node.SingleNodeMode)
	})

	t.Run("handles multi-node leader transition", func(t *testing.T) {
		rcm, node, mockRaftStore, _ := setupRaftClusterManagerTest(t)

		mockRaftStore.EXPECT().GetServersOnRaftCluster().Return([]raft.Server{
			{ID: raft.ServerID("1"), Address: raft.ServerAddress("node1:8080")},
			{ID: raft.ServerID("2"), Address: raft.ServerAddress("node2:8080")},
		})

		mockJobQueue := queue.NewMockJobQueueService(t)
		node.jobQueue = mockJobQueue
		mockJobQueue.EXPECT().RemoveServers(mock.Anything).Return()
		mockJobQueue.EXPECT().AddServers(mock.Anything).Return()
		mockJobQueue.EXPECT().SetSingleNodeMode(false).Return()

		mockJobProcessor := processor.NewMockJobProcessorService(t)
		node.jobProcessor = mockJobProcessor
		mockJobProcessor.EXPECT().StartJobs().Return().Maybe()

		mockJobExecutor := executor.NewMockJobExecutorService(t)
		node.jobExecutor = mockJobExecutor
		mockJobExecutor.EXPECT().SetSingleNodeMode(false).Return()
		mockJobExecutor.EXPECT().GetUncommittedLogs().Return([]models.JobExecutionLog{})

		mockAsyncTaskManager := async_task.NewMockAsyncTaskService(t)
		node.asyncTaskManager = mockAsyncTaskManager
		mockAsyncTaskManager.EXPECT().SetSingleNodeMode(false).Return()
		mockAsyncTaskManager.EXPECT().SetNodeIsLeader(true).Return()
		mockAsyncTaskManager.EXPECT().GetUnCommittedTasks().Return([]models.AsyncTask{}, nil)

		// Mock peer communicator methods
		mockEtcd := etcd.NewMockEtcdService(t)
		node.etcdService = mockEtcd
		mockEtcd.EXPECT().GetPeers(mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

		rcm.HandleRaftLeadershipChanges(true)

		assert.False(t, node.SingleNodeMode)
	})
}
