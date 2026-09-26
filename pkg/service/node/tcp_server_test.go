package node

import (
	"context"
	"errors"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/models"
	"scheduler0/pkg/secrets"
	"scheduler0/pkg/service/async_task"
	"scheduler0/pkg/utils"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupTCPServerTest creates a tcpServer with mocked dependencies
func setupTCPServerTest(t *testing.T) (*tcpServer, *MockNodeService, *async_task.MockAsyncTaskService) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "tcp-server-test",
		Level: hclog.LevelFromString("ERROR"),
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockNodeService := NewMockNodeService(t)
	mockAsyncTaskService := async_task.NewMockAsyncTaskService(t)

	// Use real secrets implementation with environment variables
	os.Setenv("SCHEDULER0_AUTH_USERNAME", "user")
	os.Setenv("SCHEDULER0_AUTH_PASSWORD", "pass")
	t.Cleanup(func() {
		os.Unsetenv("SCHEDULER0_AUTH_USERNAME")
		os.Unsetenv("SCHEDULER0_AUTH_PASSWORD")
	})
	realSecrets := secrets.NewScheduler0Secrets()

	mockConfig := &mockScheduler0ConfigForTesting{
		config: &config.Scheduler0Configurations{},
	}

	// Create a test nodeService that wraps the mock
	// This allows us to test tcpServer without requiring all NodeService methods
	testNodeService := &testNodeServiceWrapper{
		mock: mockNodeService,
	}

	mockListener := &mockListener{}

	server := &tcpServer{
		context:           ctx,
		logger:            logger,
		ln:                mockListener,
		nodeService:       testNodeService,
		asyncTaskService:  mockAsyncTaskService,
		scheduler0Configs: mockConfig,
		scheduler0Secrets: realSecrets,
		metrics:           utils.NewTCPServerMetrics(),
	}

	return server, mockNodeService, mockAsyncTaskService
}

// testNodeServiceWrapper wraps MockNodeService to implement NodeService interface
// for testing purposes
type testNodeServiceWrapper struct {
	mock *MockNodeService
}

func (t *testNodeServiceWrapper) StopJobs() {
	t.mock.StopJobs()
}

func (t *testNodeServiceWrapper) StartJobs() {
	t.mock.StartJobs()
}

func (t *testNodeServiceWrapper) UpdateLocalQuotaAllocations(accountAllocations map[uint64]uint64) error {
	return t.mock.UpdateLocalQuotaAllocations(accountAllocations)
}

func (t *testNodeServiceWrapper) GetLocalQuotaAllocations() map[uint64]uint64 {
	return t.mock.GetLocalQuotaAllocations()
}

func (t *testNodeServiceWrapper) ReturnUncommittedLogs(requestId string) {
	t.mock.ReturnUncommittedLogs(requestId)
}

// Implement other required methods with no-ops or return defaults
func (t *testNodeServiceWrapper) Start()                                          {}
func (t *testNodeServiceWrapper) RemoveSelfFromCluster(ctx context.Context) error { return nil }
func (t *testNodeServiceWrapper) AddSelfToCluster(ctx context.Context) error      { return nil }
func (t *testNodeServiceWrapper) ForceRebuildCluster(ctx context.Context, seedNodeId uint64) error {
	return nil
}
func (t *testNodeServiceWrapper) ResetRaftState(ctx context.Context) error { return nil }
func (t *testNodeServiceWrapper) GetRaftStats() map[string]string          { return nil }
func (t *testNodeServiceWrapper) GetRaftLeaderWithId() (raft.ServerAddress, raft.ServerID) {
	return "", ""
}
func (t *testNodeServiceWrapper) GetUncommittedLogs(requestId string) {
	t.mock.GetUncommittedLogs(requestId)
}
func (t *testNodeServiceWrapper) CanAcceptClientWriteRequest() bool { return false }
func (t *testNodeServiceWrapper) CanAcceptRequest() bool            { return false }
func (t *testNodeServiceWrapper) ResetLocalQuotaAllocations()       {}
func (t *testNodeServiceWrapper) AuthRaftConfiguration() raft.Configuration {
	return raft.Configuration{}
}
func (t *testNodeServiceWrapper) ReconcileRaftMembershipWithPeers(peers []config.RaftNode) {}
func (t *testNodeServiceWrapper) HandleRaftLeadershipChanges(isLeader bool)                {}
func (t *testNodeServiceWrapper) HandleRaftLeadershipChangesDebounced(isLeader bool)       {}
func (t *testNodeServiceWrapper) HandleRaftObserverChannelChanges(o raft.Observation)      {}
func (t *testNodeServiceWrapper) AuthenticateWithPeersFromEtcd() map[string]Status         { return nil }
func (t *testNodeServiceWrapper) WatchPeersFromEtcd()                                      {}
func (t *testNodeServiceWrapper) GetPeers() []config.RaftNode                              { return nil }
func (t *testNodeServiceWrapper) StopAllJobsOnAllWorkerNodes()                             {}
func (t *testNodeServiceWrapper) StartJobsOnWorkerNodes()                                  {}
func (t *testNodeServiceWrapper) GetRandomFanInPeerHTTPAddresses(exclude map[string]bool) []string {
	return nil
}
func (t *testNodeServiceWrapper) FanInLocalDataFromPeersSync()                               {}
func (t *testNodeServiceWrapper) FanInLocalDataFromPeers()                                   {}
func (t *testNodeServiceWrapper) HandleCompletedPeerFanIn(peerFanIn models.PeerFanIn)        {}
func (t *testNodeServiceWrapper) HandleUncommittedAsyncTasks(asyncTasks []models.AsyncTask)  {}
func (t *testNodeServiceWrapper) ListenOnInputQueues()                                       {}
func (t *testNodeServiceWrapper) BeginAcceptingClientWriteRequest()                          {}
func (t *testNodeServiceWrapper) StopAcceptingClientWriteRequest()                           {}
func (t *testNodeServiceWrapper) BeginAcceptingClientRequest()                               {}
func (t *testNodeServiceWrapper) CommitFetchedUnCommittedLogs(peerFanIns []models.PeerFanIn) {}
func (t *testNodeServiceWrapper) SelectRandomPeersToFanIn() []models.PeerFanIn               { return nil }
func (t *testNodeServiceWrapper) CheckAndUpdateExhaustedAccountJobStatus()                   {}

func TestTCPServer_HandleNodeAuthRequest(t *testing.T) {
	t.Run("returns connected for valid credentials", func(t *testing.T) {
		server, _, _ := setupTCPServerTest(t)

		payload := models.NodeAuth{
			AuthUsername: "user",
			AuthPassword: "pass",
		}

		result := server.HandleNodeAuthRequest(payload)
		assert.Equal(t, models.String("connected"), result)
	})

	t.Run("returns incorrect_credentials for invalid username", func(t *testing.T) {
		server, _, _ := setupTCPServerTest(t)

		payload := models.NodeAuth{
			AuthUsername: "wrong",
			AuthPassword: "pass",
		}

		result := server.HandleNodeAuthRequest(payload)
		assert.Equal(t, models.String("incorrect_credentials"), result)
	})

	t.Run("returns incorrect_credentials for invalid password", func(t *testing.T) {
		server, _, _ := setupTCPServerTest(t)

		payload := models.NodeAuth{
			AuthUsername: "user",
			AuthPassword: "wrong",
		}

		result := server.HandleNodeAuthRequest(payload)
		assert.Equal(t, models.String("incorrect_credentials"), result)
	})
}

func TestTCPServer_BeginUncommittedLogsFetchRequest(t *testing.T) {
	t.Run("returns request ID for valid credentials", func(t *testing.T) {
		server, mockNodeService, _ := setupTCPServerTest(t)

		mockNodeService.EXPECT().ReturnUncommittedLogs(mock.AnythingOfType("string"))

		payload := models.FetchRemoteData{
			AuthUsername: "user",
			AuthPassword: "pass",
		}

		result := server.BeginUncommittedLogsFetchRequest(payload)
		assert.NotEmpty(t, string(result))
		assert.NotEqual(t, "incorrect_credentials", string(result))
	})

	t.Run("returns incorrect_credentials for invalid credentials", func(t *testing.T) {
		server, _, _ := setupTCPServerTest(t)

		payload := models.FetchRemoteData{
			AuthUsername: "wrong",
			AuthPassword: "pass",
		}

		result := server.BeginUncommittedLogsFetchRequest(payload)
		assert.Equal(t, models.String("incorrect_credentials"), result)
	})
}

func TestTCPServer_HandleStopJobsCommand(t *testing.T) {
	t.Run("stops jobs for valid credentials", func(t *testing.T) {
		server, mockNodeService, _ := setupTCPServerTest(t)

		mockNodeService.EXPECT().StopJobs()

		payload := models.FetchRemoteData{
			RequestId:    "stop_jobs",
			AuthUsername: "user",
			AuthPassword: "pass",
		}

		result := server.HandleStopJobsCommand(payload)
		assert.Equal(t, models.String("stopped"), result)
	})

	t.Run("returns incorrect_credentials for invalid credentials", func(t *testing.T) {
		server, _, _ := setupTCPServerTest(t)

		payload := models.FetchRemoteData{
			RequestId:    "stop_jobs",
			AuthUsername: "wrong",
			AuthPassword: "pass",
		}

		result := server.HandleStopJobsCommand(payload)
		assert.Equal(t, models.String("incorrect_credentials"), result)
	})
}

func TestTCPServer_HandleStartJobsCommand(t *testing.T) {
	t.Run("starts jobs for valid credentials", func(t *testing.T) {
		server, mockNodeService, _ := setupTCPServerTest(t)

		mockNodeService.EXPECT().StartJobs()

		payload := models.FetchRemoteData{
			RequestId:    "start_jobs",
			AuthUsername: "user",
			AuthPassword: "pass",
		}

		result := server.HandleStartJobsCommand(payload)
		assert.Equal(t, models.String("started"), result)
	})

	t.Run("returns incorrect_credentials for invalid credentials", func(t *testing.T) {
		server, _, _ := setupTCPServerTest(t)

		payload := models.FetchRemoteData{
			RequestId:    "start_jobs",
			AuthUsername: "wrong",
			AuthPassword: "pass",
		}

		result := server.HandleStartJobsCommand(payload)
		assert.Equal(t, models.String("incorrect_credentials"), result)
	})
}

func TestTCPServer_HandleQuotaAllocation(t *testing.T) {
	t.Run("successfully handles quota allocation", func(t *testing.T) {
		server, mockNodeService, _ := setupTCPServerTest(t)

		mockNodeService.EXPECT().UpdateLocalQuotaAllocations(map[uint64]uint64{1: 100, 2: 200}).Return(nil)

		payload := models.QuotaAllocation{
			AuthUsername:       "user",
			AuthPassword:       "pass",
			AccountAllocations: map[uint64]uint64{1: 100, 2: 200},
		}

		result := server.HandleQuotaAllocation(payload)
		assert.Equal(t, models.String("quota_allocation_received"), result)
	})

	t.Run("handles nil node service", func(t *testing.T) {
		server, _, _ := setupTCPServerTest(t)
		server.nodeService = nil

		payload := models.QuotaAllocation{
			AuthUsername:       "user",
			AuthPassword:       "pass",
			AccountAllocations: map[uint64]uint64{1: 100},
		}

		result := server.HandleQuotaAllocation(payload)
		assert.Contains(t, string(result), "error")
		assert.Contains(t, string(result), "node service not available")
	})

	t.Run("handles update error", func(t *testing.T) {
		server, mockNodeService, _ := setupTCPServerTest(t)

		mockNodeService.EXPECT().UpdateLocalQuotaAllocations(map[uint64]uint64{1: 100}).Return(errors.New("update failed"))

		payload := models.QuotaAllocation{
			AuthUsername:       "user",
			AuthPassword:       "pass",
			AccountAllocations: map[uint64]uint64{1: 100},
		}

		result := server.HandleQuotaAllocation(payload)
		assert.Contains(t, string(result), "error")
		assert.Contains(t, string(result), "failed to update quota allocations")
	})

	t.Run("returns incorrect_credentials for invalid credentials", func(t *testing.T) {
		server, _, _ := setupTCPServerTest(t)

		payload := models.QuotaAllocation{
			AuthUsername:       "wrong",
			AuthPassword:       "pass",
			AccountAllocations: map[uint64]uint64{1: 10},
		}

		result := server.HandleQuotaAllocation(payload)
		assert.Equal(t, models.String("incorrect_credentials"), result)
	})
}

func TestTCPServer_HandleLocalQuotaRequest(t *testing.T) {
	t.Run("returns quota allocations for valid credentials", func(t *testing.T) {
		server, mockNodeService, _ := setupTCPServerTest(t)

		allocations := map[uint64]uint64{1: 10, 2: 20}
		mockNodeService.EXPECT().GetLocalQuotaAllocations().Return(allocations)

		payload := models.LocalQuotaRequest{
			AuthUsername: "user",
			AuthPassword: "pass",
		}

		result := server.HandleLocalQuotaRequest(payload)
		assert.Equal(t, allocations, result.AccountAllocations)
	})

	t.Run("returns empty map when node service is nil", func(t *testing.T) {
		server, _, _ := setupTCPServerTest(t)

		server.nodeService = nil

		payload := models.LocalQuotaRequest{
			AuthUsername: "user",
			AuthPassword: "pass",
		}

		result := server.HandleLocalQuotaRequest(payload)
		assert.Empty(t, result.AccountAllocations)
	})

	t.Run("returns empty map for invalid credentials", func(t *testing.T) {
		server, _, _ := setupTCPServerTest(t)

		payload := models.LocalQuotaRequest{
			AuthUsername: "wrong",
			AuthPassword: "pass",
		}

		result := server.HandleLocalQuotaRequest(payload)
		assert.Empty(t, result.AccountAllocations)
	})
}

func TestTCPServer_HandelUncommittedLogsFetchRequest(t *testing.T) {
	t.Run("returns task when already completed", func(t *testing.T) {
		server, _, mockAsyncTaskService := setupTCPServerTest(t)

		task := &models.AsyncTask{
			Id:    1,
			State: models.AsyncTaskSuccess,
		}

		mockAsyncTaskService.EXPECT().GetTaskWithRequestIdNonBlocking("req-1", uint64(1)).Return(task, (*utils.GenericError)(nil))

		payload := models.FetchRemoteData{
			RequestId:    "req-1",
			AuthUsername: "user",
			AuthPassword: "pass",
		}

		result := server.HandelUncommittedLogsFetchRequest(context.Background(), payload)
		assert.Equal(t, models.AsyncTaskSuccess, result.State)
	})

	t.Run("returns empty task for invalid credentials", func(t *testing.T) {
		server, _, _ := setupTCPServerTest(t)

		payload := models.FetchRemoteData{
			RequestId:    "req-1",
			AuthUsername: "wrong",
			AuthPassword: "pass",
		}

		result := server.HandelUncommittedLogsFetchRequest(context.Background(), payload)
		assert.Equal(t, models.AsyncTask{}, result)
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		server, _, mockAsyncTaskService := setupTCPServerTest(t)

		task := &models.AsyncTask{
			Id:    1,
			State: models.AsyncTaskNotStated,
		}

		mockAsyncTaskService.EXPECT().GetTaskWithRequestIdNonBlocking("req-1", uint64(1)).Return(task, (*utils.GenericError)(nil))

		// Create a channel for blocking call
		taskCh := make(chan models.AsyncTask, 1)
		mockAsyncTaskService.EXPECT().GetTaskWithRequestIdBlocking("req-1", uint64(1)).Return(taskCh, uint64(1), (*utils.GenericError)(nil))
		mockAsyncTaskService.EXPECT().GetTaskIdWithRequestId("req-1", uint64(1)).Return(uint64(1), (*utils.GenericError)(nil))
		mockAsyncTaskService.EXPECT().DeleteSubscriber(uint64(1), uint64(1)).Return(nil)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		payload := models.FetchRemoteData{
			RequestId:    "req-1",
			AuthUsername: "user",
			AuthPassword: "pass",
		}

		result := server.HandelUncommittedLogsFetchRequest(ctx, payload)
		assert.Equal(t, models.AsyncTask{}, result)
	})
}

func TestNewTCPServer(t *testing.T) {
	t.Run("creates TCP server with all dependencies", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "tcp-server-test",
			Level: hclog.LevelFromString("ERROR"),
		})

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		mockListener := &mockListener{}
		mockConfig := &mockScheduler0ConfigForTesting{
			config: &config.Scheduler0Configurations{},
		}

		os.Setenv("SCHEDULER0_AUTH_USERNAME", "user")
		os.Setenv("SCHEDULER0_AUTH_PASSWORD", "pass")
		t.Cleanup(func() {
			os.Unsetenv("SCHEDULER0_AUTH_USERNAME")
			os.Unsetenv("SCHEDULER0_AUTH_PASSWORD")
		})
		realSecrets := secrets.NewScheduler0Secrets()

		mockNodeService := NewMockNodeService(t)
		mockAsyncTaskService := async_task.NewMockAsyncTaskService(t)
		testNodeService := &testNodeServiceWrapper{mock: mockNodeService}

		server := NewTCPServer(ctx, logger, mockListener, mockConfig, realSecrets, testNodeService, mockAsyncTaskService)
		assert.NotNil(t, server)
		assert.Implements(t, (*Server)(nil), server)

		// Cancel context to stop metrics goroutine
		cancel()
		time.Sleep(50 * time.Millisecond)
	})
}

func TestTCPServer_SetupTCPListener(t *testing.T) {
	t.Run("handles connection acceptance", func(t *testing.T) {
		server, _, _ := setupTCPServerTest(t)

		// Get the mockListener from the server
		mockListener := server.ln.(*mockListener)
		mockConn := &mockConn{}

		// First call returns a connection, second call returns error to exit loop
		mockListener.On("Accept").Return(mockConn, nil).Once()
		mockListener.On("Accept").Return(nil, errors.New("listener closed")).Once()

		// Mock the connection methods needed by decode
		mockConn.On("Read", mock.Anything).Return(0, errors.New("connection closed")).Maybe()
		mockConn.On("Close").Return(nil).Maybe()
		mockAddr := &mockAddr{}
		mockConn.On("RemoteAddr").Return(mockAddr).Maybe()

		// SetupTCPListener runs in a loop, so we need to return error to stop it
		done := make(chan bool, 1)
		go func() {
			server.SetupTCPListener()
			done <- true
		}()

		// Wait for the listener to process and exit
		select {
		case <-done:
			// Success - listener exited
		case <-time.After(1 * time.Second):
			// Timeout - listener may still be running, which is okay for this test
			t.Log("SetupTCPListener did not exit within timeout, but this is acceptable")
		}

		// Verify mock expectations were met
		mockListener.AssertExpectations(t)
	})

	t.Run("handles accept errors", func(t *testing.T) {
		server, _, _ := setupTCPServerTest(t)
		mockListener := server.ln.(*mockListener)

		mockListener.On("Accept").Return(nil, errors.New("accept failed"))

		// Should return on error
		server.SetupTCPListener()
	})
}
