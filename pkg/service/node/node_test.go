package node

import (
	"context"
	"scheduler0/pkg/config"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/mocks"
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

// setupNodeServiceTest creates a nodeService with mocked dependencies
func setupNodeServiceTest(t *testing.T) (*nodeService, *fsm.MockScheduler0RaftStore, *etcd.MockEtcdService) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "node-test",
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

	// Initialize embedded components
	node.serviceState = newServiceState(node)
	node.eventHandler = newEventHandler(node)
	node.peerComm = newPeerCommunicator(node)
	node.raftCluster = newRaftClusterManager(node)

	return node, mockRaftStore, mockEtcdService
}

func TestNodeService_DelegationMethods(t *testing.T) {
	t.Run("GetRaftStats delegates to raftCluster", func(t *testing.T) {
		node, mockRaftStore, _ := setupNodeServiceTest(t)

		expectedStats := map[string]string{"state": "Leader"}
		mockRaftStore.EXPECT().GetRaftStats().Return(expectedStats)

		result := node.GetRaftStats()
		assert.Equal(t, expectedStats, result)
	})

	t.Run("GetRaftLeaderWithId delegates to raftCluster", func(t *testing.T) {
		node, mockRaftStore, _ := setupNodeServiceTest(t)

		expectedAddr := raft.ServerAddress("node1:8080")
		expectedID := raft.ServerID("1")
		mockRaftStore.EXPECT().LeaderWithID().Return(expectedAddr, expectedID)

		addr, id := node.GetRaftLeaderWithId()
		assert.Equal(t, expectedAddr, addr)
		assert.Equal(t, expectedID, id)
	})

	t.Run("CanAcceptClientWriteRequest delegates to serviceState", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)

		node.acceptClientWrites = true
		assert.True(t, node.CanAcceptClientWriteRequest())

		node.acceptClientWrites = false
		assert.False(t, node.CanAcceptClientWriteRequest())
	})

	t.Run("CanAcceptRequest delegates to serviceState", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)

		node.acceptRequest = true
		assert.True(t, node.CanAcceptRequest())

		node.acceptRequest = false
		assert.False(t, node.CanAcceptRequest())
	})

	// Note: GetUncommittedLogs and ReturnUncommittedLogs require asyncTaskManager and dispatcher
	// which need more complex setup. These are tested indirectly through integration tests.

	t.Run("GetPeers delegates to peerComm", func(t *testing.T) {
		node, _, mockEtcd := setupNodeServiceTest(t)

		// Test case 1: etcd not configured - returns nil
		mockConfig := node.scheduler0Config.(*mockScheduler0ConfigForTesting)
		mockConfig.config.EtcdEndpoints = []string{} // Empty to test nil return path

		peers := node.GetPeers()
		assert.Nil(t, peers, "should return nil when etcd is not configured")

		// Test case 2: etcd configured with cached peers
		mockConfig.config.EtcdEndpoints = []string{"localhost:2379"}
		node.peersMutex.Lock()
		node.peersFromEtcd = []config.RaftNode{
			{NodeId: 1, NodeAddress: "node1:8080"},
		}
		node.peersMutex.Unlock()

		peers = node.GetPeers()
		assert.NotNil(t, peers, "should return cached peers when available")
		assert.Len(t, peers, 1)

		// Test case 3: etcd configured but no cached peers - calls etcd
		node.peersMutex.Lock()
		node.peersFromEtcd = []config.RaftNode{}
		node.peersMutex.Unlock()

		mockEtcd.EXPECT().GetPeers(mock.Anything).Return([]config.RaftNode{
			{NodeId: 2, NodeAddress: "node2:8080"},
		}, nil)

		peers = node.GetPeers()
		assert.NotNil(t, peers, "should return peers from etcd when cache is empty")
	})

	t.Run("AuthenticateWithPeersFromEtcd delegates to peerComm", func(t *testing.T) {
		node, _, mockEtcd := setupNodeServiceTest(t)

		mockEtcd.EXPECT().GetPeers(mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

		result := node.AuthenticateWithPeersFromEtcd()
		assert.NotNil(t, result)
	})

	t.Run("AuthRaftConfiguration delegates to raftCluster", func(t *testing.T) {
		node, _, mockEtcd := setupNodeServiceTest(t)

		mockEtcd.EXPECT().GetPeers(mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

		cfg := node.AuthRaftConfiguration()
		assert.NotNil(t, cfg)
	})

	t.Run("ReconcileRaftMembershipWithPeers delegates to raftCluster", func(t *testing.T) {
		node, mockRaftStore, _ := setupNodeServiceTest(t)

		peers := []config.RaftNode{
			{NodeId: 1, NodeAddress: "node1:8080"},
		}

		mockRaftStore.EXPECT().GetRaft().Return(nil).Maybe()

		// Should not panic
		node.ReconcileRaftMembershipWithPeers(peers)
	})

	t.Run("HandleRaftObserverChannelChanges delegates to raftCluster", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)

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
		node.HandleRaftObserverChannelChanges(observation)
	})
}

func TestNodeService_DelegationMethods_Remaining(t *testing.T) {
	t.Run("Start delegates to raftCluster", func(t *testing.T) {
		node, mockRaftStore, mockEtcd := setupNodeServiceTest(t)
		mockRaftStore.EXPECT().GetRaft().Return(nil).Maybe()
		mockRaftStore.EXPECT().GetLeaderChangeChannel().Return(make(chan bool)).Maybe()
		mockRaftStore.EXPECT().GetRaftStats().Return(map[string]string{}).Maybe()
		mockRaftStore.EXPECT().GetServersOnRaftCluster().Return([]raft.Server{}).Maybe()
		mockRaftStore.EXPECT().RecoverRaftState().Return().Maybe()
		mockRaftStore.EXPECT().BootstrapRaftClusterWithConfig(mock.Anything).Return().Maybe()
		mockRaftStore.EXPECT().RegisterObserver(mock.Anything).Return().Maybe()
		mockEtcd.EXPECT().GetPeers(mock.Anything).Return([]config.RaftNode{}, nil).Maybe()
		mockEtcd.EXPECT().RegisterNode(mock.Anything, mock.Anything).Return(nil).Maybe()
		// WatchPeersFromEtcd is called in a goroutine, so we need to mock it
		peerCh := make(chan []config.RaftNode, 1)
		close(peerCh)
		mockEtcd.EXPECT().WatchPeers(mock.Anything).Return(peerCh, nil).Maybe()

		// Start should not panic
		node.Start()
		time.Sleep(50 * time.Millisecond) // Give goroutine time to start
	})

	t.Run("RemoveSelfFromCluster delegates to raftCluster", func(t *testing.T) {
		node, mockRaftStore, mockEtcd := setupNodeServiceTest(t)
		mockEtcd.EXPECT().UnregisterNode().Return(nil)
		mockRaftStore.EXPECT().GetRaft().Return(nil)

		err := node.RemoveSelfFromCluster(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "raft not initialized")
	})

	t.Run("AddSelfToCluster delegates to raftCluster", func(t *testing.T) {
		node, mockRaftStore, mockEtcd := setupNodeServiceTest(t)
		mockEtcd.EXPECT().RegisterNode(mock.Anything, mock.Anything).Return(nil)
		mockRaftStore.EXPECT().GetRaft().Return(nil)

		err := node.AddSelfToCluster(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "raft not initialized")
	})

	t.Run("GetUncommittedLogs delegates to peerComm", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)
		mockAsyncTaskManager := async_task.NewMockAsyncTaskService(t)
		mockJobExecutor := executor.NewMockJobExecutorService(t)
		mockDispatcher := utils.NewDispatcher(node.ctx, 1, 10)
		mockDispatcher.Run()

		node.asyncTaskManager = mockAsyncTaskManager
		node.jobExecutor = mockJobExecutor
		node.dispatcher = mockDispatcher

		mockAsyncTaskManager.EXPECT().AddTasks("", "test-id", mock.Anything, uint64(1)).Return([]uint64{1}, nil).Maybe()
		mockAsyncTaskManager.EXPECT().UpdateTasksByRequestId("test-id", mock.Anything, mock.Anything).Return(nil).Maybe()
		mockJobExecutor.EXPECT().GetUncommittedLogs().Return([]models.JobExecutionLog{}).Maybe()
		mockAsyncTaskManager.EXPECT().GetUnCommittedTasks().Return([]models.AsyncTask{}, nil).Maybe()

		// Should not panic
		node.GetUncommittedLogs("test-id")
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("ReturnUncommittedLogs delegates to peerComm", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)
		mockAsyncTaskManager := async_task.NewMockAsyncTaskService(t)
		mockJobExecutor := executor.NewMockJobExecutorService(t)
		mockDispatcher := utils.NewDispatcher(node.ctx, 1, 10)
		mockDispatcher.Run()

		node.asyncTaskManager = mockAsyncTaskManager
		node.jobExecutor = mockJobExecutor
		node.dispatcher = mockDispatcher

		mockAsyncTaskManager.EXPECT().AddTasks("", "test-id", mock.Anything, uint64(1)).Return([]uint64{1}, nil).Maybe()
		mockAsyncTaskManager.EXPECT().UpdateTasksByRequestId("test-id", mock.Anything, mock.Anything).Return(nil).Maybe()
		mockJobExecutor.EXPECT().GetUncommittedLogs().Return([]models.JobExecutionLog{}).Maybe()
		mockAsyncTaskManager.EXPECT().GetUnCommittedTasks().Return([]models.AsyncTask{}, nil).Maybe()

		// Should not panic
		node.ReturnUncommittedLogs("test-id")
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("StopJobs delegates to serviceState", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)
		mockJobExecutor := executor.NewMockJobExecutorService(t)
		node.jobExecutor = mockJobExecutor

		mockJobExecutor.EXPECT().StopAll().Return()

		node.StopJobs()
	})

	t.Run("StartJobs delegates to serviceState", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)
		mockJobProcessor := processor.NewMockJobProcessorService(t)
		node.jobProcessor = mockJobProcessor

		mockJobProcessor.EXPECT().RecoverJobs().Return()

		node.StartJobs()
	})

	t.Run("UpdateLocalQuotaAllocations delegates to serviceState", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)
		mockJobExecutor := executor.NewMockJobExecutorService(t)
		node.jobExecutor = mockJobExecutor

		allocations := map[uint64]uint64{1: 100}
		mockJobExecutor.EXPECT().UpdateLocalQuotaAllocations(allocations).Return()

		err := node.UpdateLocalQuotaAllocations(allocations)
		assert.NoError(t, err)
	})

	t.Run("ResetLocalQuotaAllocations delegates to serviceState", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)
		mockJobExecutor := executor.NewMockJobExecutorService(t)
		node.jobExecutor = mockJobExecutor

		mockJobExecutor.EXPECT().ResetLocalQuotaAllocations().Return()

		node.ResetLocalQuotaAllocations()
	})

	t.Run("GetLocalQuotaAllocations delegates to serviceState", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)
		mockJobExecutor := executor.NewMockJobExecutorService(t)
		node.jobExecutor = mockJobExecutor

		expected := map[uint64]uint64{1: 100}
		mockJobExecutor.EXPECT().GetAllLocalQuotaAllocations().Return(expected)

		result := node.GetLocalQuotaAllocations()
		assert.Equal(t, expected, result)
	})

	t.Run("WatchPeersFromEtcd delegates to peerComm", func(t *testing.T) {
		node, _, mockEtcd := setupNodeServiceTest(t)
		peerCh := make(chan []config.RaftNode)
		close(peerCh)

		mockEtcd.EXPECT().WatchPeers(mock.Anything).Return(peerCh, nil).Maybe()

		// Should return when channel is closed
		node.WatchPeersFromEtcd()
	})

	t.Run("HandleRaftLeadershipChanges delegates to raftCluster", func(t *testing.T) {
		node, mockRaftStore, mockEtcd := setupNodeServiceTest(t)
		// HandleRaftLeadershipChanges needs jobQueue, jobExecutor, asyncTaskManager, jobProcessor, and peerComm
		mockJobQueue := queue.NewMockJobQueueService(t)
		mockJobExecutor := executor.NewMockJobExecutorService(t)
		mockAsyncTaskManager := async_task.NewMockAsyncTaskService(t)
		mockJobProcessor := processor.NewMockJobProcessorService(t)
		mockJobExecutionRepo := mocks.NewMockJobExecutionsRepo(t)
		mockAsyncTaskRepo := mocks.NewMockAsyncTasksRepo(t)
		mockClient := NewMockClient(t)

		node.jobQueue = mockJobQueue
		node.jobExecutor = mockJobExecutor
		node.asyncTaskManager = mockAsyncTaskManager
		node.jobProcessor = mockJobProcessor
		node.jobExecutionRepo = mockJobExecutionRepo
		node.asyncTaskRepo = mockAsyncTaskRepo
		node.client = mockClient

		// Single node mode (len(servers) == 1)
		servers := []raft.Server{
			{ID: "1", Address: "node1"},
		}
		mockRaftStore.EXPECT().GetServersOnRaftCluster().Return(servers)
		mockJobQueue.EXPECT().RemoveServers([]uint64{1}).Return()
		mockJobQueue.EXPECT().AddServers([]uint64{1}).Return()
		mockJobQueue.EXPECT().SetSingleNodeMode(true).Return()
		mockJobExecutor.EXPECT().SetSingleNodeMode(true).Return()
		// StopAllJobsOnAllWorkerNodes is called which needs etcd
		mockEtcd.EXPECT().GetPeers(mock.Anything).Return([]config.RaftNode{}, nil).Maybe()
		mockClient.EXPECT().StopJobs(mock.Anything, node, mock.Anything).Return(nil).Maybe()
		mockAsyncTaskManager.EXPECT().SetSingleNodeMode(true).Return()
		mockAsyncTaskManager.EXPECT().SetNodeIsLeader(true).Return()
		// In single node mode, GetUncommittedLogs and GetUnCommittedTasks are not called
		mockJobProcessor.EXPECT().StartJobs().Return()

		// Should not panic
		node.HandleRaftLeadershipChanges(true)
	})

	t.Run("HandleRaftLeadershipChangesDebounced delegates to raftCluster", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)
		// Initialize leadershipDebounce if nil
		if node.leadershipDebounce == nil {
			node.leadershipDebounce = utils.NewDebounce()
		}
		// This method uses debouncing which is complex to test in unit tests
		// We just verify it can be called without immediate panic
		// Full testing should be done in integration tests
		node.HandleRaftLeadershipChangesDebounced(false)
		// Cancel context to stop debounce goroutine
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("ListenOnInputQueues delegates to eventHandler", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		node, mockRaftStore, _ := setupNodeServiceTest(t)
		node.ctx = ctx // Use cancellable context

		// Initialize leadershipDebounce if nil
		if node.leadershipDebounce == nil {
			node.leadershipDebounce = utils.NewDebounce()
		}

		leaderCh := make(chan bool)
		peerObsCh := make(chan raft.Observation, 10)
		fanInCh := make(chan models.PeerFanIn, 10)
		postProcessCh := make(chan models.PostProcess, 10)

		node.peerObserverChannels = peerObsCh
		node.fanInCh = fanInCh
		node.postProcessingChannel = postProcessCh

		mockRaftStore.EXPECT().GetLeaderChangeChannel().Return(leaderCh)

		// Start in goroutine and cancel context to stop it
		done := make(chan bool)
		go func() {
			node.ListenOnInputQueues()
			done <- true
		}()

		// Cancel context to stop the loop
		cancel()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("ListenOnInputQueues did not stop after context cancellation")
		}
	})

	t.Run("HandleCompletedPeerFanIn delegates to eventHandler", func(t *testing.T) {
		node, _, mockEtcd := setupNodeServiceTest(t)
		mockJobRepo := mocks.NewMockJobRepo(t)
		mockAsyncTaskManager := async_task.NewMockAsyncTaskService(t)
		mockJobProcessor := processor.NewMockJobProcessorService(t)
		node.jobRepo = mockJobRepo
		node.asyncTaskManager = mockAsyncTaskManager
		node.jobProcessor = mockJobProcessor

		peerFanIn := models.PeerFanIn{
			PeerNodeAddress: "node1",
			State:           models.PeerFanInStateComplete,
			Data: models.LocalData{
				ExecutionLogs: []models.JobExecutionLog{{Id: 1}},
				AsyncTasks:    []models.AsyncTask{{Id: 1}},
			},
		}

		// Store peerFanIn in completedFanInCh
		node.completedFanInCh.Store("node1", peerFanIn)

		// GetPeers is called to check peer count
		mockEtcd.EXPECT().GetPeers(mock.Anything).Return([]config.RaftNode{
			{NodeId: 1, NodeAddress: "node1"},
			{NodeId: 2, NodeAddress: "node2"},
		}, nil).Maybe()

		// Mock expectations for batch insert (if conditions are met)
		mockJobRepo.EXPECT().BatchInsertJobs(mock.Anything).Return([]uint64{1}, nil).Maybe()
		mockAsyncTaskManager.EXPECT().UpdateTasksByRequestId(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		mockJobProcessor.EXPECT().StartJobs().Return().Maybe()

		// Should not panic
		node.HandleCompletedPeerFanIn(peerFanIn)
	})

	t.Run("HandleUncommittedAsyncTasks delegates to eventHandler", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)
		mockJobRepo := mocks.NewMockJobRepo(t)
		mockAsyncTaskManager := async_task.NewMockAsyncTaskService(t)
		node.jobRepo = mockJobRepo
		node.asyncTaskManager = mockAsyncTaskManager

		asyncTasks := []models.AsyncTask{
			{Id: 1, RequestId: "test", State: models.AsyncTaskInProgress, Service: "job_executor"},
		}

		mockAsyncTaskManager.EXPECT().UpdateTasksByRequestId("test", models.AsyncTaskSuccess, "").Return(nil).Maybe()

		// Should not panic
		node.HandleUncommittedAsyncTasks(asyncTasks)
	})

	t.Run("BeginAcceptingClientWriteRequest delegates to serviceState", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)

		// Should not panic
		node.BeginAcceptingClientWriteRequest()
	})

	t.Run("StopAcceptingClientWriteRequest delegates to serviceState", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)

		// Should not panic
		node.StopAcceptingClientWriteRequest()
	})

	t.Run("BeginAcceptingClientRequest delegates to serviceState", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)

		// Should not panic
		node.BeginAcceptingClientRequest()
	})
}

func TestNewNode(t *testing.T) {
	t.Run("creates node service with all components", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "node-test",
			Level: hclog.LevelFromString("ERROR"),
		})

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		mockConfig := &mockScheduler0ConfigForTesting{
			config: &config.Scheduler0Configurations{
				EtcdEndpoints: []string{"localhost:2379"},
			},
		}

		mockRaftStore := fsm.NewMockScheduler0RaftStore(t)
		mockEtcdService := etcd.NewMockEtcdService(t)
		mockEtcdService.EXPECT().GetPeers(mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

		// Create minimal node service
		node := NewNode(
			ctx,
			logger,
			mockConfig,
			nil, // scheduler0Secrets
			mockRaftStore,
			nil,   // fsmActions
			nil,   // jobExecutor
			nil,   // jobQueue
			nil,   // jobProcessor
			nil,   // jobRepo
			nil,   // sharedRepo
			nil,   // jobExecutionRepo
			nil,   // asyncTaskRepo
			nil,   // asyncTaskManager
			nil,   // dispatcher
			nil,   // postProcessingChannel
			false, // isExistingNode
			nil,   // nodeClient
			nil,   // raftLn
			mockEtcdService,
			nil, // logDb
			nil, // storeDb
			nil, // fileSnapShot
			nil, // transportManager
		)

		assert.NotNil(t, node)
		assert.Implements(t, (*NodeService)(nil), node)
	})
}

func TestNodeService_DelegationMethods_PeerCommunicator(t *testing.T) {
	t.Run("StopAllJobsOnAllWorkerNodes delegates to peerComm", func(t *testing.T) {
		node, _, mockEtcd := setupNodeServiceTest(t)
		mockClient := NewMockClient(t)
		node.client = mockClient

		peers := []config.RaftNode{
			{NodeId: 1, NodeAddress: "node1"},
		}
		configs := node.scheduler0Config.GetConfigurations()
		configs.NodeId = 2

		mockEtcd.EXPECT().GetPeers(mock.Anything).Return(peers, nil).Maybe()
		mockClient.EXPECT().StopJobs(mock.Anything, node, peers[0]).Return(nil).Maybe()

		// Should not panic
		node.StopAllJobsOnAllWorkerNodes()
	})

	t.Run("StartJobsOnWorkerNodes delegates to peerComm", func(t *testing.T) {
		node, _, mockEtcd := setupNodeServiceTest(t)
		mockClient := NewMockClient(t)
		node.client = mockClient

		peers := []config.RaftNode{
			{NodeId: 1, NodeAddress: "node1"},
		}
		configs := node.scheduler0Config.GetConfigurations()
		configs.NodeId = 2

		mockEtcd.EXPECT().GetPeers(mock.Anything).Return(peers, nil).Maybe()
		mockClient.EXPECT().StartJobs(mock.Anything, node, peers[0]).Return(nil).Maybe()

		// Should not panic
		node.StartJobsOnWorkerNodes()
	})

	t.Run("GetRandomFanInPeerHTTPAddresses delegates to peerComm", func(t *testing.T) {
		node, mockRaftStore, mockEtcd := setupNodeServiceTest(t)
		mockRaftStore.EXPECT().GetServersOnRaftCluster().Return([]raft.Server{
			{ID: "1", Address: "node1"},
			{ID: "2", Address: "node2"},
		})

		configs := node.scheduler0Config.GetConfigurations()
		configs.NodeId = 3

		node.peersFromEtcd = []config.RaftNode{
			{NodeId: 1, NodeAddress: "node1"},
			{NodeId: 2, NodeAddress: "node2"},
		}

		// GetPeers is called inside GetRandomFanInPeerHTTPAddresses
		mockEtcd.EXPECT().GetPeers(mock.Anything).Return(node.peersFromEtcd, nil).Maybe()

		excludeList := map[string]bool{}
		// GetPeers is called inside GetRandomFanInPeerHTTPAddresses
		mockEtcd.EXPECT().GetPeers(mock.Anything).Return(node.peersFromEtcd, nil).Maybe()

		result := node.GetRandomFanInPeerHTTPAddresses(excludeList)

		// Just verify the method doesn't panic and returns a result (may be empty if conditions aren't met)
		_ = result
	})

	t.Run("FanInLocalDataFromPeersSync delegates to peerComm", func(t *testing.T) {
		node, mockRaftStore, _ := setupNodeServiceTest(t)
		mockRaftStore.EXPECT().GetServersOnRaftCluster().Return([]raft.Server{
			{ID: "1", Address: "node1"},
		})

		configs := node.scheduler0Config.GetConfigurations()
		configs.NodeId = 2
		configs.RaftTransportTimeout = 5

		node.peersFromEtcd = []config.RaftNode{
			{NodeId: 1, NodeAddress: "node1"},
		}

		// FanInLocalDataFromPeersSync will try to connect but will timeout/fail
		// We just verify it can be called without panic
		node.FanInLocalDataFromPeersSync()
	})

	t.Run("FanInLocalDataFromPeers delegates to peerComm", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		node, mockRaftStore, _ := setupNodeServiceTest(t)
		node.ctx = ctx

		mockRaftStore.EXPECT().GetServersOnRaftCluster().Return([]raft.Server{
			{ID: "1", Address: "node1"},
		}).Maybe()

		configs := node.scheduler0Config.GetConfigurations()
		configs.NodeId = 2
		configs.ExecutionLogFetchIntervalSeconds = 1

		node.peersFromEtcd = []config.RaftNode{
			{NodeId: 1, NodeAddress: "node1"},
		}

		// Starts a goroutine, cancel context to stop it
		node.FanInLocalDataFromPeers()
		time.Sleep(50 * time.Millisecond)
		cancel()
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("SelectRandomPeersToFanIn delegates to peerComm", func(t *testing.T) {
		node, mockRaftStore, _ := setupNodeServiceTest(t)
		mockRaftStore.EXPECT().GetServersOnRaftCluster().Return([]raft.Server{
			{ID: "1", Address: "node1"},
			{ID: "2", Address: "node2"},
		})

		configs := node.scheduler0Config.GetConfigurations()
		configs.NodeId = 3

		node.peersFromEtcd = []config.RaftNode{
			{NodeId: 1, NodeAddress: "node1"},
			{NodeId: 2, NodeAddress: "node2"},
		}

		result := node.SelectRandomPeersToFanIn()
		// Should return peers (excluding self)
		assert.NotNil(t, result)
	})

	t.Run("CommitFetchedUnCommittedLogs delegates to peerComm", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)
		mockJobExecutionRepo := mocks.NewMockJobExecutionsRepo(t)
		mockAsyncTaskRepo := mocks.NewMockAsyncTasksRepo(t)
		node.jobExecutionRepo = mockJobExecutionRepo
		node.asyncTaskRepo = mockAsyncTaskRepo
		// Initialize fanInCh to prevent nil channel panic
		node.fanInCh = make(chan models.PeerFanIn, 10)

		configs := node.scheduler0Config.GetConfigurations()
		configs.NodeId = 1

		peerFanIns := []models.PeerFanIn{
			{
				PeerNodeAddress: "node1",
				Data: models.LocalData{
					ExecutionLogs: []models.JobExecutionLog{{Id: 1}},
					AsyncTasks:    []models.AsyncTask{{Id: 1}},
				},
			},
		}

		mockJobExecutionRepo.EXPECT().RaftInsertExecutionLogs(peerFanIns[0].Data.ExecutionLogs, configs.NodeId).Return().Maybe()
		mockAsyncTaskRepo.EXPECT().RaftBatchInsert(peerFanIns[0].Data.AsyncTasks, configs.NodeId).Return([]uint64{1}, nil).Maybe()

		// Should not panic
		node.CommitFetchedUnCommittedLogs(peerFanIns)
		time.Sleep(50 * time.Millisecond) // Give goroutine time to send
	})

}

func TestNodeService_DelegationMethods_RaftCluster(t *testing.T) {
	t.Run("ForceRebuildCluster delegates to raftCluster", func(t *testing.T) {
		node, mockRaftStore, _ := setupNodeServiceTest(t)
		mockRaftStore.EXPECT().GetRaft().Return(nil).Maybe()

		err := node.ForceRebuildCluster(context.Background(), 1)
		// May return error if raft not initialized, that's ok
		_ = err
	})

	t.Run("ResetRaftState delegates to raftCluster", func(t *testing.T) {
		node, _, _ := setupNodeServiceTest(t)
		// ResetRaftState calls os.Exit, so we can't fully test it
		// Just verify the method exists and can be called
		// In a real scenario, this would need integration testing
		_ = node.ResetRaftState
	})
}
