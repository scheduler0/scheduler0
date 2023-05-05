package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfigurations(t *testing.T) {
	// Call GetConfigurations for the first time
	config1 := GetConfigurations()

	// Call GetConfigurations for the second time
	config2 := GetConfigurations()

	// Assert that the same configuration object is returned every time
	assert.Equal(t, config1, config2, "GetConfigurations should return the same configuration object every time it's called")
}

func TestGetConfigFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("SCHEDULER0_LOGLEVEL", "info")
	os.Setenv("SCHEDULER0_PROTOCOL", "http")
	os.Setenv("SCHEDULER0_HOST", "localhost")
	os.Setenv("SCHEDULER0_PORT", "8080")
	os.Setenv("SCHEDULER0_REPLICAS", `[{"nodeId":1, "address":"localhost:12345"}]`)
	os.Setenv("SCHEDULER0_PEER_AUTH_REQUEST_TIMEOUT_MS", "5000")
	os.Setenv("SCHEDULER0_PEER_CONNECT_RETRY_MAX", "3")
	os.Setenv("SCHEDULER0_PEER_CONNECT_RETRY_DELAY_SECONDS", "1")
	os.Setenv("SCHEDULER0_BOOTSTRAP", "true")
	os.Setenv("SCHEDULER0_NODE_ID", "1")
	os.Setenv("SCHEDULER0_RAFT_ADDRESS", "localhost:12345")
	os.Setenv("SCHEDULER0_RAFT_TRANSPORT_MAX_POOL", "3")
	os.Setenv("SCHEDULER0_RAFT_TRANSPORT_TIMEOUT", "1000")
	os.Setenv("SCHEDULER0_RAFT_APPLY_TIMEOUT", "1000")
	os.Setenv("SCHEDULER0_RAFT_SNAPSHOT_INTERVAL", "1000")
	os.Setenv("SCHEDULER0_RAFT_SNAPSHOT_THRESHOLD", "1000")
	os.Setenv("SCHEDULER0_RAFT_HEARTBEAT_TIMEOUT", "1000")
	os.Setenv("SCHEDULER0_RAFT_ELECTION_TIMEOUT", "1000")
	os.Setenv("SCHEDULER0_RAFT_COMMIT_TIMEOUT", "1000")
	os.Setenv("SCHEDULER0_RAFT_MAX_APPEND_ENTRIES", "1000")
	os.Setenv("SCHEDULER0_JOB_EXECUTION_TIMEOUT", "1000")
	os.Setenv("SCHEDULER0_JOB_EXECUTION_RETRY_DELAY", "1000")
	os.Setenv("SCHEDULER0_JOB_EXECUTION_RETRY_MAX", "3")
	os.Setenv("SCHEDULER0_MAX_WORKERS", "10")
	os.Setenv("SCHEDULER0_JOB_QUEUE_DEBOUNCE_DELAY", "1000")
	os.Setenv("SCHEDULER0_MAX_MEMORY", "1024")
	os.Setenv("SCHEDULER0_EXECUTION_LOG_FETCH_FAN_IN", "2")
	os.Setenv("SCHEDULER0_EXECUTION_LOG_FETCH_INTERVAL_SECONDS", "10")
	os.Setenv("SCHEDULER0_JOB_INVOCATION_DEBOUNCE_DELAY", "1000")
	os.Setenv("SCHEDULER0_HTTP_EXECUTOR_PAYLOAD_MAX_SIZE_MB", "5")

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
	assert.Equal(t, uint64(1), config.NodeId)
	assert.Equal(t, "localhost:12345", config.RaftAddress)
	assert.Equal(t, uint64(3), config.RaftTransportMaxPool)
	assert.Equal(t, uint64(1000), config.RaftTransportTimeout)
	assert.Equal(t, uint64(1000), config.RaftApplyTimeout)
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
	assert.Equal(t, uint64(1000), config.JobQueueDebounceDelay)
	assert.Equal(t, uint64(1024), config.MaxMemory)
	assert.Equal(t, uint64(2), config.ExecutionLogFetchFanIn)
	assert.Equal(t, uint64(10), config.ExecutionLogFetchIntervalSeconds)
	assert.Equal(t, uint64(1000), config.JobInvocationDebounceDelay)
	assert.Equal(t, uint64(5), config.HTTPExecutorPayloadMaxSizeMb)
}
