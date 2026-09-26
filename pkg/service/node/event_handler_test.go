package node

import (
	"context"
	"encoding/json"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/mocks"
	async_task_service "scheduler0/pkg/service/async_task"
	"scheduler0/pkg/service/executor"
	"scheduler0/pkg/service/processor"
	"scheduler0/pkg/utils"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestEventHandler_HandleCompletedPeerFanIn(t *testing.T) {
	t.Run("stores peer fan in and checks completion", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		mockJobProcessor := processor.NewMockJobProcessorService(t)

		node := &nodeService{
			logger:          logger,
			jobProcessor:    mockJobProcessor,
			completedFanInCh: sync.Map{},
			scheduler0Config: config.NewScheduler0Config(),
			acceptClientWrites: false,
		}

		// Create real components
		peerComm := newPeerCommunicator(node)
		serviceState := newServiceState(node)
		node.peerComm = peerComm
		node.serviceState = serviceState

		eh := newEventHandler(node)

		peerFanIn := models.PeerFanIn{
			PeerNodeAddress: "node1",
			State:           models.PeerFanInStateGetExecutionsLogs,
		}

		// Mock GetPeers to return peers (excluding self)
		// Since we can't easily mock the peerComm component, we'll test with a real one
		// but we need to set up the node properly
		node.peersFromEtcd = []config.RaftNode{
			{NodeId: 1, NodeAddress: "node1"},
			{NodeId: 2, NodeAddress: "node2"},
		}

		mockJobProcessor.EXPECT().StartJobs().Return().Maybe()

		eh.HandleCompletedPeerFanIn(peerFanIn)

		// Verify it was stored
		_, exists := node.completedFanInCh.Load("node1")
		assert.True(t, exists)
	})

	t.Run("handles empty peers list", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		node := &nodeService{
			logger:          logger,
			completedFanInCh: sync.Map{},
			scheduler0Config: config.NewScheduler0Config(),
		}

		peerComm := newPeerCommunicator(node)
		serviceState := newServiceState(node)
		node.peerComm = peerComm
		node.serviceState = serviceState
		node.peersFromEtcd = []config.RaftNode{} // Empty peers

		eh := newEventHandler(node)

		peerFanIn := models.PeerFanIn{
			PeerNodeAddress: "node1",
			State:           models.PeerFanInStateGetExecutionsLogs,
		}

		// Should not panic with empty peers
		eh.HandleCompletedPeerFanIn(peerFanIn)
	})

	t.Run("does not start jobs when already accepting writes", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		mockJobProcessor := processor.NewMockJobProcessorService(t)

		node := &nodeService{
			logger:          logger,
			jobProcessor:    mockJobProcessor,
			completedFanInCh: sync.Map{},
			scheduler0Config: config.NewScheduler0Config(),
			acceptClientWrites: true, // Already accepting
		}

		peerComm := newPeerCommunicator(node)
		serviceState := newServiceState(node)
		node.peerComm = peerComm
		node.serviceState = serviceState

		eh := newEventHandler(node)

		peerFanIn := models.PeerFanIn{
			PeerNodeAddress: "node1",
			State:           models.PeerFanInStateGetExecutionsLogs,
		}

		// Should not call StartJobs since already accepting writes
		eh.HandleCompletedPeerFanIn(peerFanIn)

		mockJobProcessor.AssertNotCalled(t, "StartJobs")
	})
}

func TestEventHandler_HandleUncommittedAsyncTasks(t *testing.T) {
	t.Run("handles CreateJobAsyncTaskService with valid payload", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		mockJobRepo := mocks.NewMockJobRepo(t)
		mockAsyncTaskManager := async_task_service.NewMockAsyncTaskService(t)

		jobsPayload := []models.Job{
			{ID: 1, AccountId: 1},
			{ID: 2, AccountId: 1},
		}
		payloadBytes, _ := json.Marshal(jobsPayload)

		asyncTask := models.AsyncTask{
			Id:        1,
			RequestId: "req-1",
			State:     models.AsyncTaskNotStated,
			Service:   constants.CreateJobAsyncTaskService,
			Input:     string(payloadBytes),
		}

		jobIds := []uint64{1, 2}
		mockJobRepo.EXPECT().BatchInsertJobs(mock.Anything).Return(jobIds, (*utils.GenericError)(nil))
		mockAsyncTaskManager.EXPECT().UpdateTasksByRequestId("req-1", models.AsyncTaskSuccess, mock.Anything).Return((*utils.GenericError)(nil))

		node := &nodeService{
			logger:           logger,
			jobRepo:          mockJobRepo,
			asyncTaskManager: mockAsyncTaskManager,
		}

		eh := newEventHandler(node)
		eh.HandleUncommittedAsyncTasks([]models.AsyncTask{asyncTask})

		mockJobRepo.AssertExpectations(t)
		mockAsyncTaskManager.AssertExpectations(t)
	})

	t.Run("handles invalid JSON payload gracefully", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		mockJobRepo := mocks.NewMockJobRepo(t)
		mockAsyncTaskManager := async_task_service.NewMockAsyncTaskService(t)

		asyncTask := models.AsyncTask{
			Id:        1,
			RequestId: "req-1",
			State:     models.AsyncTaskNotStated,
			Service:   constants.CreateJobAsyncTaskService,
			Input:     "invalid json",
		}

		// When JSON unmarshal fails, BatchInsertJobs is still called with empty slice
		// and UpdateTasksByRequestId is still called
		mockJobRepo.EXPECT().BatchInsertJobs(mock.Anything).Return(nil, (*utils.GenericError)(nil))
		mockAsyncTaskManager.EXPECT().UpdateTasksByRequestId("req-1", models.AsyncTaskSuccess, mock.Anything).Return((*utils.GenericError)(nil))

		node := &nodeService{
			logger:           logger,
			jobRepo:          mockJobRepo,
			asyncTaskManager: mockAsyncTaskManager,
		}

		eh := newEventHandler(node)
		// Should not panic
		eh.HandleUncommittedAsyncTasks([]models.AsyncTask{asyncTask})
	})

	t.Run("handles JobExecutorAsyncTaskService", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		mockAsyncTaskManager := async_task_service.NewMockAsyncTaskService(t)

		asyncTask := models.AsyncTask{
			Id:        1,
			RequestId: "req-1",
			State:     models.AsyncTaskInProgress,
			Service:   constants.JobExecutorAsyncTaskService,
		}

		mockAsyncTaskManager.EXPECT().UpdateTasksByRequestId("req-1", models.AsyncTaskSuccess, "").Return((*utils.GenericError)(nil))

		node := &nodeService{
			logger:           logger,
			asyncTaskManager: mockAsyncTaskManager,
		}

		eh := newEventHandler(node)
		eh.HandleUncommittedAsyncTasks([]models.AsyncTask{asyncTask})

		mockAsyncTaskManager.AssertExpectations(t)
	})

	t.Run("handles batch insert error gracefully", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		mockJobRepo := mocks.NewMockJobRepo(t)
		mockAsyncTaskManager := async_task_service.NewMockAsyncTaskService(t)

		jobsPayload := []models.Job{{ID: 1}}
		payloadBytes, _ := json.Marshal(jobsPayload)

		asyncTask := models.AsyncTask{
			Id:        1,
			RequestId: "req-1",
			State:     models.AsyncTaskNotStated,
			Service:   constants.CreateJobAsyncTaskService,
			Input:     string(payloadBytes),
		}

		genericErr := &utils.GenericError{Message: "batch insert failed"}
		mockJobRepo.EXPECT().BatchInsertJobs(mock.Anything).Return(nil, genericErr)
		// Even when batch insert fails, UpdateTasksByRequestId is still called
		mockAsyncTaskManager.EXPECT().UpdateTasksByRequestId("req-1", models.AsyncTaskSuccess, mock.Anything).Return((*utils.GenericError)(nil))

		node := &nodeService{
			logger:           logger,
			jobRepo:          mockJobRepo,
			asyncTaskManager: mockAsyncTaskManager,
		}

		eh := newEventHandler(node)
		// Should not panic on error
		eh.HandleUncommittedAsyncTasks([]models.AsyncTask{asyncTask})
	})

	t.Run("handles empty async tasks list", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		node := &nodeService{
			logger: logger,
		}

		eh := newEventHandler(node)
		// Should not panic
		eh.HandleUncommittedAsyncTasks([]models.AsyncTask{})
	})

	t.Run("handles multiple async tasks with different states", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		mockJobRepo := mocks.NewMockJobRepo(t)
		mockAsyncTaskManager := async_task_service.NewMockAsyncTaskService(t)

		jobsPayload := []models.Job{{ID: 1}}
		payloadBytes, _ := json.Marshal(jobsPayload)

		asyncTasks := []models.AsyncTask{
			{
				Id:        1,
				RequestId: "req-1",
				State:     models.AsyncTaskNotStated,
				Service:   constants.CreateJobAsyncTaskService,
				Input:     string(payloadBytes),
			},
			{
				Id:        2,
				RequestId: "req-2",
				State:     models.AsyncTaskInProgress,
				Service:   constants.JobExecutorAsyncTaskService,
			},
		}

		mockJobRepo.EXPECT().BatchInsertJobs(mock.Anything).Return([]uint64{1}, (*utils.GenericError)(nil))
		mockAsyncTaskManager.EXPECT().UpdateTasksByRequestId("req-1", models.AsyncTaskSuccess, mock.Anything).Return((*utils.GenericError)(nil))
		mockAsyncTaskManager.EXPECT().UpdateTasksByRequestId("req-2", models.AsyncTaskSuccess, "").Return((*utils.GenericError)(nil))

		node := &nodeService{
			logger:           logger,
			jobRepo:          mockJobRepo,
			asyncTaskManager: mockAsyncTaskManager,
		}

		eh := newEventHandler(node)
		eh.HandleUncommittedAsyncTasks(asyncTasks)

		mockJobRepo.AssertExpectations(t)
		mockAsyncTaskManager.AssertExpectations(t)
	})
}

func TestEventHandler_ListenOnInputQueues(t *testing.T) {
	t.Run("handles leader change events", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockRaftStore := fsm.NewMockScheduler0RaftStore(t)
		leaderCh := make(chan bool, 1)
		mockRaftStore.EXPECT().GetLeaderChangeChannel().Return(leaderCh)

		node := &nodeService{
			logger:            logger,
			ctx:               ctx,
			scheduler0RaftStore: mockRaftStore,
			peerObserverChannels: make(chan raft.Observation, 10),
			fanInCh:           make(chan models.PeerFanIn, 10),
			postProcessingChannel: make(chan models.PostProcess, 10),
			scheduler0Config:  config.NewScheduler0Config(),
			leadershipDebounce: utils.NewDebounce(),
		}

		raftCluster := newRaftClusterManager(node)
		node.raftCluster = raftCluster
		eh := newEventHandler(node)

		// Send leader change event
		leaderCh <- true

		// Start listening in goroutine
		done := make(chan bool, 1)
		go func() {
			eh.ListenOnInputQueues()
			done <- true
		}()

		// Cancel context to stop
		time.Sleep(50 * time.Millisecond)
		cancel()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("ListenOnInputQueues did not stop after context cancellation")
		}
	})

	t.Run("handles peer observer channel events", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockRaftStore := fsm.NewMockScheduler0RaftStore(t)
		leaderCh := make(chan bool, 1)
		mockRaftStore.EXPECT().GetLeaderChangeChannel().Return(leaderCh)

		peerObsCh := make(chan raft.Observation, 1)
		node := &nodeService{
			logger:            logger,
			ctx:               ctx,
			scheduler0RaftStore: mockRaftStore,
			peerObserverChannels: peerObsCh,
			fanInCh:           make(chan models.PeerFanIn, 10),
			postProcessingChannel: make(chan models.PostProcess, 10),
			scheduler0Config:  config.NewScheduler0Config(),
		}

		raftCluster := newRaftClusterManager(node)
		node.raftCluster = raftCluster
		eh := newEventHandler(node)

		// Send peer observation
		peerObsCh <- raft.Observation{}

		// Start listening in goroutine
		done := make(chan bool, 1)
		go func() {
			eh.ListenOnInputQueues()
			done <- true
		}()

		// Cancel context to stop
		time.Sleep(50 * time.Millisecond)
		cancel()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("ListenOnInputQueues did not stop after context cancellation")
		}
	})

	t.Run("handles fan in channel events", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockRaftStore := fsm.NewMockScheduler0RaftStore(t)
		leaderCh := make(chan bool, 1)
		mockRaftStore.EXPECT().GetLeaderChangeChannel().Return(leaderCh)

		fanInCh := make(chan models.PeerFanIn, 1)
		node := &nodeService{
			logger:            logger,
			ctx:               ctx,
			scheduler0RaftStore: mockRaftStore,
			peerObserverChannels: make(chan raft.Observation, 10),
			fanInCh:           fanInCh,
			postProcessingChannel: make(chan models.PostProcess, 10),
			scheduler0Config:  config.NewScheduler0Config(),
			completedFanInCh: sync.Map{},
		}

		peerComm := newPeerCommunicator(node)
		node.peerComm = peerComm
		raftCluster := newRaftClusterManager(node)
		node.raftCluster = raftCluster
		eh := newEventHandler(node)

		// Send fan in event
		fanInCh <- models.PeerFanIn{PeerNodeAddress: "node1"}

		// Start listening in goroutine
		done := make(chan bool, 1)
		go func() {
			eh.ListenOnInputQueues()
			done <- true
		}()

		// Cancel context to stop
		time.Sleep(50 * time.Millisecond)
		cancel()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("ListenOnInputQueues did not stop after context cancellation")
		}
	})

	t.Run("handles post processing channel events", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockRaftStore := fsm.NewMockScheduler0RaftStore(t)
		leaderCh := make(chan bool, 1)
		mockRaftStore.EXPECT().GetLeaderChangeChannel().Return(leaderCh)

		mockJobExecutor := executor.NewMockJobExecutorService(t)
		mockAsyncTaskManager := async_task_service.NewMockAsyncTaskService(t)

		postProcessCh := make(chan models.PostProcess, 1)
		configs := config.NewScheduler0Config().GetConfigurations()
		configs.NodeId = 1

		node := &nodeService{
			logger:            logger,
			ctx:               ctx,
			scheduler0RaftStore: mockRaftStore,
			peerObserverChannels: make(chan raft.Observation, 10),
			fanInCh:           make(chan models.PeerFanIn, 10),
			postProcessingChannel: postProcessCh,
			scheduler0Config:  config.NewScheduler0Config(),
			jobExecutor:       mockJobExecutor,
			asyncTaskManager:  mockAsyncTaskManager,
		}

		raftCluster := newRaftClusterManager(node)
		node.raftCluster = raftCluster
		eh := newEventHandler(node)

		// Test QueueJob action
		mockJobExecutor.EXPECT().QueueExecutions(uint64(1), uint64(10)).Return().Maybe()
		postProcessCh <- models.PostProcess{
			Action:     constants.CommandActionQueueJob,
			TargetNodes: []uint64{1},
			Data:       models.SQLResponse{LastInsertedId: int64(1), RowsAffected: int64(10)},
		}

		// Start listening in goroutine
		done := make(chan bool, 1)
		go func() {
			eh.ListenOnInputQueues()
			done <- true
		}()

		// Cancel context to stop
		time.Sleep(50 * time.Millisecond)
		cancel()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("ListenOnInputQueues did not stop after context cancellation")
		}
	})
}

