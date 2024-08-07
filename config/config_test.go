package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfigurations(t *testing.T) {
	// Call GetConfigurations for the first time
	configProvider := NewScheduler0Config()

	config1 := configProvider.GetConfigurations()

	// Call GetConfigurations for the second time
	config2 := configProvider.GetConfigurations()

	// Assert that the same configuration object is returned every time
	assert.Equal(t, config1, config2, "GetConfigurations should return the same configuration object every time it's called")
}

func TestGetConfigFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("SCHEDULER0_LOGLEVEL", "info")
	defer os.Unsetenv("SCHEDULER0_LOGLEVEL")
	os.Setenv("SCHEDULER0_PROTOCOL", "http")
	defer os.Unsetenv("SCHEDULER0_PROTOCOL")
	os.Setenv("SCHEDULER0_HOST", "localhost")
	defer os.Unsetenv("SCHEDULER0_HOST")
	os.Setenv("SCHEDULER0_PORT", "8080")
	defer os.Unsetenv("SCHEDULER0_PORT")
	os.Setenv("SCHEDULER0_REPLICAS", `[{"nodeId":1, "address":"localhost:12345"}]`)
	defer os.Unsetenv("SCHEDULER0_REPLICAS")
	os.Setenv("SCHEDULER0_PEER_AUTH_REQUEST_TIMEOUT_MS", "5000")
	defer os.Unsetenv("SCHEDULER0_PEER_AUTH_REQUEST_TIMEOUT_MS")
	os.Setenv("SCHEDULER0_PEER_CONNECT_RETRY_MAX", "3")
	defer os.Unsetenv("SCHEDULER0_PEER_CONNECT_RETRY_MAX")
	os.Setenv("SCHEDULER0_PEER_CONNECT_RETRY_DELAY_SECONDS", "1")
	defer os.Unsetenv("SCHEDULER0_PEER_CONNECT_RETRY_DELAY_SECONDS")
	os.Setenv("SCHEDULER0_BOOTSTRAP", "true")
	defer os.Unsetenv("SCHEDULER0_BOOTSTRAP")
	os.Setenv("SCHEDULER0_NODE_ID", "2")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	os.Setenv("SCHEDULER0_RAFT_ADDRESS", "localhost:12345")
	defer os.Unsetenv("SCHEDULER0_RAFT_ADDRESS")
	os.Setenv("SCHEDULER0_RAFT_TRANSPORT_MAX_POOL", "3")
	defer os.Unsetenv("SCHEDULER0_RAFT_TRANSPORT_MAX_POOL")
	os.Setenv("SCHEDULER0_RAFT_TRANSPORT_TIMEOUT", "1000")
	defer os.Unsetenv("SCHEDULER0_RAFT_TRANSPORT_TIMEOUT")
	os.Setenv("SCHEDULER0_RAFT_SNAPSHOT_INTERVAL", "1000")
	defer os.Unsetenv("SCHEDULER0_RAFT_SNAPSHOT_INTERVAL")
	os.Setenv("SCHEDULER0_RAFT_SNAPSHOT_THRESHOLD", "1000")
	defer os.Unsetenv("SCHEDULER0_RAFT_SNAPSHOT_THRESHOLD")
	os.Setenv("SCHEDULER0_RAFT_HEARTBEAT_TIMEOUT", "1000")
	defer os.Unsetenv("SCHEDULER0_RAFT_HEARTBEAT_TIMEOUT")
	os.Setenv("SCHEDULER0_RAFT_ELECTION_TIMEOUT", "1000")
	defer os.Unsetenv("SCHEDULER0_RAFT_ELECTION_TIMEOUT")
	os.Setenv("SCHEDULER0_RAFT_COMMIT_TIMEOUT", "1000")
	defer os.Unsetenv("SCHEDULER0_RAFT_COMMIT_TIMEOUT")
	os.Setenv("SCHEDULER0_RAFT_MAX_APPEND_ENTRIES", "1000")
	defer os.Unsetenv("SCHEDULER0_RAFT_MAX_APPEND_ENTRIES")
	os.Setenv("SCHEDULER0_JOB_EXECUTION_TIMEOUT", "1000")
	defer os.Unsetenv("SCHEDULER0_JOB_EXECUTION_TIMEOUT")
	os.Setenv("SCHEDULER0_JOB_EXECUTION_RETRY_DELAY", "1000")
	defer os.Unsetenv("SCHEDULER0_JOB_EXECUTION_RETRY_DELAY")
	os.Setenv("SCHEDULER0_JOB_EXECUTION_RETRY_MAX", "3")
	defer os.Unsetenv("SCHEDULER0_JOB_EXECUTION_RETRY_MAX")
	os.Setenv("SCHEDULER0_MAX_WORKERS", "10")
	defer os.Unsetenv("SCHEDULER0_MAX_WORKERS")
	os.Setenv("SCHEDULER0_JOB_QUEUE_DEBOUNCE_DELAY", "1000")
	defer os.Unsetenv("SCHEDULER0_JOB_QUEUE_DEBOUNCE_DELAY")
	os.Setenv("SCHEDULER0_MAX_MEMORY", "1024")
	defer os.Unsetenv("SCHEDULER0_MAX_MEMORY")
	os.Setenv("SCHEDULER0_EXECUTION_LOG_FETCH_FAN_IN", "2")
	defer os.Unsetenv("SCHEDULER0_EXECUTION_LOG_FETCH_FAN_IN")
	os.Setenv("SCHEDULER0_EXECUTION_LOG_FETCH_INTERVAL_SECONDS", "10")
	defer os.Unsetenv("SCHEDULER0_EXECUTION_LOG_FETCH_INTERVAL_SECONDS")
	os.Setenv("SCHEDULER0_JOB_INVOCATION_DEBOUNCE_DELAY", "1000")
	defer os.Unsetenv("SCHEDULER0_JOB_INVOCATION_DEBOUNCE_DELAY")
	os.Setenv("SCHEDULER0_HTTP_EXECUTOR_PAYLOAD_MAX_SIZE_MB", "5")
	defer os.Unsetenv("SCHEDULER0_HTTP_EXECUTOR_PAYLOAD_MAX_SIZE_MB")

	// Get configuration from environment variables
	config := getConfigFromEnv()

	// Check if the values are set correctly
	assert.NotNil(t, config)
	assert.Equal(t, "info", config.LogLevel)
	assert.Equal(t, "http", config.Protocol)
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, "8080", config.Port)

	expectedReplicas := []RaftNode{{NodeId: 1, Address: "localhost:12345"}}
	assert.Equal(t, expectedReplicas, config.Replicas)

	assert.Equal(t, uint64(5000), config.PeerAuthRequestTimeoutMs)
	assert.Equal(t, uint64(3), config.PeerConnectRetryMax)
	assert.Equal(t, uint64(1), config.PeerConnectRetryDelaySeconds)
	assert.Equal(t, true, config.Bootstrap)
	assert.Equal(t, uint64(2), config.NodeId)
	assert.Equal(t, "localhost:12345", config.RaftAddress)
	assert.Equal(t, uint64(3), config.RaftTransportMaxPool)
	assert.Equal(t, uint64(1000), config.RaftTransportTimeout)
	assert.Equal(t, uint64(1000), config.RaftSnapshotInterval)
	assert.Equal(t, uint64(1000), config.RaftSnapshotThreshold)
	assert.Equal(t, uint64(1000), config.RaftHeartbeatTimeout)
	assert.Equal(t, uint64(1000), config.RaftElectionTimeout)
	assert.Equal(t, uint64(1000), config.RaftCommitTimeout)
	assert.Equal(t, uint64(1000), config.RaftMaxAppendEntries)
	assert.Equal(t, uint64(1000), config.JobExecutionTimeout)
	assert.Equal(t, uint64(1000), config.JobExecutionRetryDelay)
	assert.Equal(t, uint64(3), config.JobExecutionRetryMax)
	assert.Equal(t, uint64(10), config.MaxWorkers)
	assert.Equal(t, uint64(1024), config.MaxMemory)
	assert.Equal(t, uint64(2), config.ExecutionLogFetchFanIn)
	assert.Equal(t, uint64(10), config.ExecutionLogFetchIntervalSeconds)
	assert.Equal(t, uint64(5), config.HTTPExecutorPayloadMaxSizeMb)
}
