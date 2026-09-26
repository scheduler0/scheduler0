package node

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/models"
	"scheduler0/pkg/secrets"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockListener is a mock network.Listener for testing
type mockListener struct {
	mock.Mock
}

func (m *mockListener) Accept() (net.Conn, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(net.Conn), args.Error(1)
}

func (m *mockListener) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockListener) Addr() net.Addr {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(net.Addr)
}

func (m *mockListener) Dial(address string, timeout time.Duration) (net.Conn, error) {
	args := m.Called(address, timeout)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(net.Conn), args.Error(1)
}

// mockConn is a mock net.Conn for testing
type mockConn struct {
	mock.Mock
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

func (m *mockConn) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockConn) RemoteAddr() net.Addr {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(net.Addr)
}

func (m *mockConn) LocalAddr() net.Addr {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(net.Addr)
}

func (m *mockConn) SetDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

// setupTCPClientTest creates a tcpClient with mocked dependencies
func setupTCPClientTest(t *testing.T) (*tcpClient, *mockListener) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "tcp-client-test",
		Level: hclog.LevelFromString("ERROR"),
	})

	mockListener := &mockListener{}
	mockConfig := &mockScheduler0ConfigForTesting{
		config: &config.Scheduler0Configurations{
			RaftTransportTimeout: 5,
		},
	}

	// Set up secrets via environment
	os.Setenv("SCHEDULER0_AUTH_USERNAME", "user")
	os.Setenv("SCHEDULER0_AUTH_PASSWORD", "pass")
	t.Cleanup(func() {
		os.Unsetenv("SCHEDULER0_AUTH_USERNAME")
		os.Unsetenv("SCHEDULER0_AUTH_PASSWORD")
	})
	realSecrets := secrets.NewScheduler0Secrets()

	client := &tcpClient{
		logger:            logger,
		ln:                mockListener,
		scheduler0Configs: mockConfig,
		scheduler0Secrets: realSecrets,
	}

	return client, mockListener
}

func Test_isRetriableTCPErr(t *testing.T) {
	t.Run("returns false for nil error", func(t *testing.T) {
		result := isRetriableTCPErr(nil)
		assert.False(t, result)
	})

	t.Run("returns false for context canceled", func(t *testing.T) {
		err := context.Canceled
		result := isRetriableTCPErr(err)
		assert.False(t, result)
	})

	t.Run("returns false for context deadline exceeded", func(t *testing.T) {
		err := context.DeadlineExceeded
		result := isRetriableTCPErr(err)
		assert.False(t, result)
	})

	t.Run("returns true for connection reset error", func(t *testing.T) {
		err := errors.New("connection reset by peer")
		result := isRetriableTCPErr(err)
		assert.True(t, result)
	})

	t.Run("returns true for EOF error", func(t *testing.T) {
		err := io.EOF
		result := isRetriableTCPErr(err)
		assert.True(t, result)
	})

	t.Run("returns true for timeout error", func(t *testing.T) {
		err := &net.OpError{
			Op:  "read",
			Net: "tcp",
			Err: &timeoutError{timeout: true},
		}
		result := isRetriableTCPErr(err)
		assert.True(t, result)
	})

	t.Run("returns true for temporary error", func(t *testing.T) {
		err := &net.OpError{
			Op:  "read",
			Net: "tcp",
			Err: &temporaryError{temporary: true},
		}
		result := isRetriableTCPErr(err)
		assert.True(t, result)
	})
}

// Helper types for testing
type timeoutError struct {
	timeout bool
}

func (e *timeoutError) Error() string {
	return "timeout"
}

func (e *timeoutError) Timeout() bool {
	return e.timeout
}

func (e *timeoutError) Temporary() bool {
	return false
}

type temporaryError struct {
	temporary bool
}

func (e *temporaryError) Error() string {
	return "temporary"
}

func (e *temporaryError) Timeout() bool {
	return false
}

func (e *temporaryError) Temporary() bool {
	return e.temporary
}

func TestNewTCPClient(t *testing.T) {
	t.Run("creates tcp client", func(t *testing.T) {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "tcp-client-test",
			Level: hclog.LevelFromString("ERROR"),
		})

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

		client := NewTCPClient(logger, mockListener, mockConfig, realSecrets)
		assert.NotNil(t, client)
		assert.Implements(t, (*Client)(nil), client)
	})
}

// mockReadWriteCloser is a mock that implements net.Conn using bytes.Buffer
type mockReadWriteCloser struct {
	*bytes.Buffer
	closed bool
}

func newMockReadWriteCloser() *mockReadWriteCloser {
	return &mockReadWriteCloser{
		Buffer: bytes.NewBuffer(nil),
		closed: false,
	}
}

func (m *mockReadWriteCloser) Close() error {
	m.closed = true
	return nil
}

func (m *mockReadWriteCloser) LocalAddr() net.Addr {
	return &mockAddr{}
}

func (m *mockReadWriteCloser) RemoteAddr() net.Addr {
	return &mockAddr{}
}

func (m *mockReadWriteCloser) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockReadWriteCloser) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockReadWriteCloser) SetWriteDeadline(t time.Time) error {
	return nil
}

type mockAddr struct{}

func (m *mockAddr) Network() string {
	return "tcp"
}

func (m *mockAddr) String() string {
	return "127.0.0.1:8080"
}

func TestTCPClient_ConnectNode(t *testing.T) {
	t.Run("handles connection error", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(nil, errors.New("connection failed"))

		status, err := client.ConnectNode(replica)
		assert.Error(t, err)
		assert.Nil(t, status)
	})

	t.Run("handles dial timeout", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		// Simulate timeout error
		timeoutErr := &net.OpError{
			Op:  "dial",
			Net: "tcp",
			Err: &timeoutError{timeout: true},
		}
		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(nil, timeoutErr)

		status, err := client.ConnectNode(replica)
		assert.Error(t, err)
		assert.Nil(t, status)
	})

	t.Run("successfully connects with valid credentials", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		mockConn := newMockReadWriteCloser()
		response := models.String("connected")
		_, _ = response.WriteTo(mockConn)

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		status, err := client.ConnectNode(replica)
		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.True(t, status.IsAuth)
		assert.True(t, status.IsAlive)
	})

	t.Run("handles authentication failure", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		mockConn := newMockReadWriteCloser()
		response := models.String("incorrect_credentials")
		_, _ = response.WriteTo(mockConn)

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		status, err := client.ConnectNode(replica)
		assert.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "authentication failed")
	})
}

func TestTCPClient_FetchUncommittedLogsFromPeersPhase1(t *testing.T) {
	t.Run("handles connection error", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		ctx := context.Background()
		node := &nodeService{
			logger:  logger,
			ctx:     ctx,
			fanIns:  sync.Map{},
			fanInCh: make(chan models.PeerFanIn, 10),
		}

		peerFanIn := models.PeerFanIn{
			PeerNodeAddress: "node1:8080",
			State:           models.PeerFanInStateNotStated,
		}

		mockListener.On("Dial", peerFanIn.PeerNodeAddress, mock.AnythingOfType("time.Duration")).Return(nil, errors.New("connection failed"))

		// Should not panic, just log error and retry
		client.FetchUncommittedLogsFromPeersPhase1(ctx, node, []models.PeerFanIn{peerFanIn})
	})

	t.Run("handles empty peer list", func(t *testing.T) {
		client, _ := setupTCPClientTest(t)

		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		ctx := context.Background()
		node := &nodeService{
			logger:  logger,
			ctx:     ctx,
			fanIns:  sync.Map{},
			fanInCh: make(chan models.PeerFanIn, 10),
		}

		// Should handle empty list gracefully
		client.FetchUncommittedLogsFromPeersPhase1(ctx, node, []models.PeerFanIn{})
	})

	t.Run("successfully fetches request id", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		ctx := context.Background()
		node := &nodeService{
			logger:  logger,
			ctx:     ctx,
			fanIns:  sync.Map{},
			fanInCh: make(chan models.PeerFanIn, 10),
		}

		peerFanIn := models.PeerFanIn{
			PeerNodeAddress: "node1:8080",
			State:           models.PeerFanInStateNotStated,
		}

		mockConn := newMockReadWriteCloser()
		response := models.String("test-request-id")
		_, _ = response.WriteTo(mockConn)

		mockListener.On("Dial", peerFanIn.PeerNodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		// Should not panic
		client.FetchUncommittedLogsFromPeersPhase1(ctx, node, []models.PeerFanIn{peerFanIn})

		// Note: The actual implementation updates fanIns asynchronously via withRetry
		// So we just verify the method was called without panic
		time.Sleep(100 * time.Millisecond) // Give time for async operations
	})

	t.Run("handles authentication failure", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		ctx := context.Background()
		node := &nodeService{
			logger:  logger,
			ctx:     ctx,
			fanIns:  sync.Map{},
			fanInCh: make(chan models.PeerFanIn, 10),
		}

		peerFanIn := models.PeerFanIn{
			PeerNodeAddress: "node1:8080",
			State:           models.PeerFanInStateNotStated,
		}

		mockConn := newMockReadWriteCloser()
		response := models.String("incorrect_credentials")
		_, _ = response.WriteTo(mockConn)

		mockListener.On("Dial", peerFanIn.PeerNodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		// Should not panic, just log error
		client.FetchUncommittedLogsFromPeersPhase1(ctx, node, []models.PeerFanIn{peerFanIn})
	})
}

func TestTCPClient_FetchUncommittedLogsFromPeersPhase2(t *testing.T) {
	t.Run("handles connection error", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		ctx := context.Background()
		node := &nodeService{
			logger:  logger,
			ctx:     ctx,
			fanIns:  sync.Map{},
			fanInCh: make(chan models.PeerFanIn, 10),
		}

		peerFanIn := models.PeerFanIn{
			PeerNodeAddress: "node1:8080",
			RequestId:       "test-request-id",
			State:           models.PeerFanInStateGetRequestId,
		}

		mockListener.On("Dial", peerFanIn.PeerNodeAddress, mock.AnythingOfType("time.Duration")).Return(nil, errors.New("connection failed"))

		// Should not panic, just log error and retry
		client.FetchUncommittedLogsFromPeersPhase2(ctx, node, []models.PeerFanIn{peerFanIn})
	})

	t.Run("handles empty peer list", func(t *testing.T) {
		client, _ := setupTCPClientTest(t)

		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		ctx := context.Background()
		node := &nodeService{
			logger:  logger,
			ctx:     ctx,
			fanIns:  sync.Map{},
			fanInCh: make(chan models.PeerFanIn, 10),
		}

		// Should handle empty list gracefully
		client.FetchUncommittedLogsFromPeersPhase2(ctx, node, []models.PeerFanIn{})
	})

	t.Run("successfully fetches async task", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "test",
			Level: hclog.LevelFromString("ERROR"),
		})

		ctx := context.Background()
		node := &nodeService{
			logger:  logger,
			ctx:     ctx,
			fanIns:  sync.Map{},
			fanInCh: make(chan models.PeerFanIn, 10),
		}

		peerFanIn := models.PeerFanIn{
			PeerNodeAddress: "node1:8080",
			RequestId:       "test-request-id",
			State:           models.PeerFanInStateGetRequestId,
		}

		localData := models.LocalData{
			ExecutionLogs: []models.JobExecutionLog{{Id: 1}},
			AsyncTasks:    []models.AsyncTask{{Id: 1}},
		}
		outputBytes, _ := json.Marshal(localData)

		asyncTask := models.AsyncTask{
			Id:        1,
			RequestId: "test-request-id",
			Output:    string(outputBytes),
		}

		mockConn := newMockReadWriteCloser()
		_, _ = asyncTask.WriteTo(mockConn)

		mockListener.On("Dial", peerFanIn.PeerNodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		// Should not panic
		client.FetchUncommittedLogsFromPeersPhase2(ctx, node, []models.PeerFanIn{peerFanIn})

		// Note: The actual implementation updates fanIns asynchronously via withRetry
		// So we just verify the method was called without panic
		time.Sleep(100 * time.Millisecond) // Give time for async operations
	})
}

func TestTCPClient_StopJobs(t *testing.T) {
	t.Run("handles connection error", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(nil, errors.New("connection failed"))

		err := client.StopJobs(context.Background(), nil, replica)
		assert.Error(t, err)
	})

	t.Run("successfully stops jobs", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		mockConn := newMockReadWriteCloser()
		response := models.String("stopped")
		_, _ = response.WriteTo(mockConn)

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		err := client.StopJobs(context.Background(), nil, replica)
		assert.NoError(t, err)
	})

	t.Run("handles authentication failure", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		mockConn := newMockReadWriteCloser()
		response := models.String("incorrect_credentials")
		_, _ = response.WriteTo(mockConn)

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		err := client.StopJobs(context.Background(), nil, replica)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authentication failed")
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(nil, errors.New("connection failed"))

		err := client.StopJobs(ctx, nil, replica)
		assert.Error(t, err)
	})
}

func TestTCPClient_StartJobs(t *testing.T) {
	t.Run("handles connection error", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(nil, errors.New("connection failed"))

		err := client.StartJobs(context.Background(), nil, replica)
		assert.Error(t, err)
	})

	t.Run("successfully starts jobs", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		mockConn := newMockReadWriteCloser()
		response := models.String("started")
		_, _ = response.WriteTo(mockConn)

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		err := client.StartJobs(context.Background(), nil, replica)
		assert.NoError(t, err)
	})

	t.Run("handles authentication failure", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		mockConn := newMockReadWriteCloser()
		response := models.String("incorrect_credentials")
		_, _ = response.WriteTo(mockConn)

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		err := client.StartJobs(context.Background(), nil, replica)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authentication failed")
	})
}

func TestTCPClient_SendQuotaAllocation(t *testing.T) {
	t.Run("handles connection error", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		allocations := map[uint64]uint64{1: 100}

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(nil, errors.New("connection failed"))

		err := client.SendQuotaAllocation(context.Background(), replica, allocations)
		assert.Error(t, err)
	})

	t.Run("successfully sends quota allocation", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		allocations := map[uint64]uint64{1: 100, 2: 200}

		mockConn := newMockReadWriteCloser()
		response := models.String("quota_allocation_received")
		_, _ = response.WriteTo(mockConn)

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		err := client.SendQuotaAllocation(context.Background(), replica, allocations)
		assert.NoError(t, err)
	})

	t.Run("handles authentication failure", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		allocations := map[uint64]uint64{1: 100}

		mockConn := newMockReadWriteCloser()
		response := models.String("incorrect_credentials")
		_, _ = response.WriteTo(mockConn)

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		err := client.SendQuotaAllocation(context.Background(), replica, allocations)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authentication failed")
	})

	t.Run("handles error response", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		allocations := map[uint64]uint64{1: 100}

		mockConn := newMockReadWriteCloser()
		response := models.String("error: failed to update")
		_, _ = response.WriteTo(mockConn)

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		err := client.SendQuotaAllocation(context.Background(), replica, allocations)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "quota allocation failed")
	})
}

func TestTCPClient_RequestLocalQuotaAllocations(t *testing.T) {
	t.Run("handles connection error", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(nil, errors.New("connection failed"))

		allocations, err := client.RequestLocalQuotaAllocations(context.Background(), replica)
		assert.Error(t, err)
		assert.Nil(t, allocations)
	})

	t.Run("successfully requests local quota allocations", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		expectedAllocations := map[uint64]uint64{1: 100, 2: 200}
		mockConn := newMockReadWriteCloser()
		response := models.LocalQuotaResponse{
			AccountAllocations: expectedAllocations,
		}
		_, _ = response.WriteTo(mockConn)

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		allocations, err := client.RequestLocalQuotaAllocations(context.Background(), replica)
		assert.NoError(t, err)
		assert.Equal(t, expectedAllocations, allocations)
	})

	t.Run("handles unexpected response type", func(t *testing.T) {
		client, mockListener := setupTCPClientTest(t)

		replica := config.RaftNode{
			NodeId:      1,
			NodeAddress: "node1:8080",
		}

		mockConn := newMockReadWriteCloser()
		response := models.String("unexpected")
		_, _ = response.WriteTo(mockConn)

		mockListener.On("Dial", replica.NodeAddress, mock.AnythingOfType("time.Duration")).Return(mockConn, nil)

		allocations, err := client.RequestLocalQuotaAllocations(context.Background(), replica)
		assert.Error(t, err)
		assert.Nil(t, allocations)
		assert.Contains(t, err.Error(), "unexpected response type")
	})
}

func Test_decode(t *testing.T) {
	t.Run("decodes String payload", func(t *testing.T) {
		mockConn := newMockReadWriteCloser()
		payload := models.String("test")
		_, _ = payload.WriteTo(mockConn)

		result, err := decode(mockConn)
		assert.NoError(t, err)
		assert.IsType(t, (*models.String)(nil), result)
	})

	t.Run("decodes LocalQuotaResponse payload", func(t *testing.T) {
		mockConn := newMockReadWriteCloser()
		payload := models.LocalQuotaResponse{
			AccountAllocations: map[uint64]uint64{1: 100},
		}
		_, _ = payload.WriteTo(mockConn)

		result, err := decode(mockConn)
		assert.NoError(t, err)
		assert.IsType(t, (*models.LocalQuotaResponse)(nil), result)
	})

	t.Run("returns error for unknown type", func(t *testing.T) {
		mockConn := newMockReadWriteCloser()
		// Write an unknown type byte
		mockConn.Write([]byte{255}) // Invalid type

		result, err := decode(mockConn)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unknown type")
	})

	t.Run("handles read errors", func(t *testing.T) {
		mockConn := newMockReadWriteCloser()
		// Close the connection to simulate read error
		mockConn.Close()

		result, err := decode(mockConn)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
