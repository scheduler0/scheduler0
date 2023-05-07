package config

import (
	"encoding/json"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"path"
	"scheduler0/constants"
	"strconv"
	"sync"
)

// RaftNode represents a node in a Raft cluster, providing the necessary
// information for communication between nodes and identification within the cluster.
type RaftNode struct {
	Address     string `json:"address" yaml:"Address"`          // Network address of the Raft node for client communication
	RaftAddress string `json:"raft_address" yaml:"RaftAddress"` // Address for the Raft protocol communication between nodes in the cluster
	NodeId      uint64 `json:"nodeId" yaml:"NodeId"`            // Unique identifier for the Raft node within the cluster
}

//go:generate mockery --name Scheduler0Config
type Scheduler0Config interface {
	GetConfigurations() *scheduler0Configurations
}

// scheduler0Configurations global configurations
type scheduler0Configurations struct {
	LogLevel                         string     `json:"logLevel" yaml:"LogLevel"`                                                 // Logging verbosity level
	Protocol                         string     `json:"protocol" yaml:"Protocol"`                                                 // Communication protocol used
	Host                             string     `json:"host" yaml:"Host"`                                                         // Host address
	Port                             string     `json:"port" yaml:"Port"`                                                         // Port number
	Replicas                         []RaftNode `json:"replicas" yaml:"Replicas"`                                                 // List of replicas in the Raft cluster
	PeerAuthRequestTimeoutMs         uint64     `json:"PeerAuthRequestTimeoutMs" yaml:"PeerAuthRequestTimeoutMs"`                 // Peer authentication request timeout in milliseconds
	PeerConnectRetryMax              uint64     `json:"peerConnectRetryMax" yaml:"PeerConnectRetryMax"`                           // Maximum number of retries for connecting to peers
	PeerConnectRetryDelaySeconds     uint64     `json:"peerConnectRetryDelay" yaml:"PeerConnectRetryDelaySeconds"`                // Delay between retries for connecting to peers, in seconds
	Bootstrap                        bool       `json:"bootstrap" yaml:"Bootstrap"`                                               // Whether the scheduler should start in bootstrap mode
	NodeId                           uint64     `json:"nodeId" yaml:"NodeId"`                                                     // Unique identifier for the scheduler node
	RaftAddress                      string     `json:"raftAddress" yaml:"RaftAddress"`                                           // Address used for Raft communication
	RaftTransportMaxPool             uint64     `json:"raftTransportMaxPool" yaml:"RaftTransportMaxPool"`                         // Maximum size of the Raft transport pool
	RaftTransportTimeout             uint64     `json:"raftTransportTimeout" yaml:"RaftTransportTimeout"`                         // Timeout for Raft transport operations
	RaftApplyTimeout                 uint64     `json:"raftApplyTimeout" yaml:"RaftApplyTimeout"`                                 // Timeout for applying Raft log entries
	RaftSnapshotInterval             uint64     `json:"raftSnapshotInterval" yaml:"RaftSnapshotInterval"`                         // Interval between Raft snapshots
	RaftSnapshotThreshold            uint64     `json:"raftSnapshotThreshold" yaml:"RaftSnapshotThreshold"`                       // Threshold for Raft snapshot creation
	RaftHeartbeatTimeout             uint64     `json:"raftHeartbeatTimeout" yaml:"RaftHeartbeatTimeout"`                         // Timeout for Raft heartbeat
	RaftElectionTimeout              uint64     `json:"raftElectionTimeout" yaml:"RaftElectionTimeout"`                           // Timeout for Raft leader election
	RaftCommitTimeout                uint64     `json:"raftCommitTimeout" yaml:"RaftCommitTimeout"`                               // Timeout for Raft commit operation
	RaftMaxAppendEntries             uint64     `json:"raftMaxAppendEntries" yaml:"RaftMaxAppendEntries"`                         // Maximum number of entries to append in a single Raft operation
	JobExecutionTimeout              uint64     `json:"jobExecutionTimeout" yaml:"JobExecutionTimeout"`                           // Timeout for job execution
	JobExecutionRetryDelay           uint64     `json:"jobExecutionRetryDelay" yaml:"JobExecutionRetryDelay"`                     // Delay between retries for job execution
	JobExecutionRetryMax             uint64     `json:"jobExecutionRetryMax" yaml:"JobExecutionRetryMax"`                         // Maximum number of retries for job execution
	MaxWorkers                       uint64     `json:"maxWorkers" yaml:"MaxWorkers"`                                             // Maximum number of concurrent workers
	MaxQueue                         uint64     `json:"maxQueue" yaml:"MaxQueue"`                                                 // Maximum size of the job queue
	JobQueueDebounceDelay            uint64     `json:"jobQueueDebounceDelay" yaml:"JobQueueDebounceDelay"`                       // Delay for debouncing the job queue
	MaxMemory                        uint64     `json:"maxMemory" yaml:"MaxMemory"`                                               // Maximum amount of memory to be used by the scheduler
	ExecutionLogFetchFanIn           uint64     `json:"executionLogFetchFanIn" yaml:"ExecutionLogFetchFanIn"`                     // Fan-in factor for fetching execution logs
	ExecutionLogFetchIntervalSeconds uint64     `json:"executionLogFetchIntervalSeconds" yaml:"ExecutionLogFetchIntervalSeconds"` // Interval between log fetches, in seconds
	JobInvocationDebounceDelay       uint64     `json:"jobInvocationDebounceDelay" yaml:"JobInvocationDebounceDelay"`             // Delay for debouncing job invocation
	HTTPExecutorPayloadMaxSizeMb     uint64     `json:"httpExecutorPayloadMaxSizeMb" yaml:"HTTPExecutorPayloadMaxSizeMb"`         // Maximum payload size for HTTP executor, in megabytes

}

var cachedConfig *Scheduler0Configurations
var once sync.Once

// GetConfigurations returns the cached Scheduler0Configurations if it exists,
// otherwise it reads the configuration file and caches it.
func GetConfigurations() *Scheduler0Configurations {
	// Check if cachedConfig is not nil, then return it
	if cachedConfig != nil {
		return cachedConfig
	}

	// Ensure that the configuration is read and cached only once
	once.Do(func() {
		// Get the binary path
		binPath := GetBinPath()

		// Create a new file system
		fs := afero.NewOsFs()
		// Read the configuration file
		data, err := afero.ReadFile(fs, binPath+"/"+constants.ConfigFileName)
		// If there is an error and it's not due to the file not existing, panic
		if err != nil && !os.IsNotExist(err) {
			panic(err)
		}

		// Initialize an empty Scheduler0Configurations struct
		config := Scheduler0Configurations{}

		// If the error is due to the file not existing, get the configuration from environment variables
		if os.IsNotExist(err) {
			config = *GetConfigFromEnv()
		}

		// Unmarshal the YAML data into the config struct
		err = yaml.Unmarshal(data, &config)
		// If there is an error in unmarshaling, panic
		if err != nil {
			panic(err)
		}
		// Cache the configuration
		cachedConfig = &config
	})

	// Return the cached configuration
	return cachedConfig
}

// GetConfigFromEnv gets scheduler0 configurations from env
func GetConfigFromEnv() *Scheduler0Configurations {
	config := &Scheduler0Configurations{}

	// Set LogLevel
	if val, ok := os.LookupEnv("SCHEDULER0_LOGLEVEL"); ok {
		config.LogLevel = val
	}

	// Set Protocol
	if val, ok := os.LookupEnv("SCHEDULER0_PROTOCOL"); ok {
		config.Protocol = val
	}

	// Set Host
	if val, ok := os.LookupEnv("SCHEDULER0_HOST"); ok {
		config.Host = val
	}

	// Set Port
	if val, ok := os.LookupEnv("SCHEDULER0_PORT"); ok {
		config.Port = val
	}

	// Set Replicas
	if val, ok := os.LookupEnv("SCHEDULER0_REPLICAS"); ok {
		replicas := []RaftNode{}
		err := json.Unmarshal([]byte(val), &replicas)
		if err != nil {
			log.Fatalf("Error unmarshaling replicas: %v", err)
		}
		config.Replicas = replicas
	}

	// Set PeerAuthRequestTimeoutMs
	if val, ok := os.LookupEnv("SCHEDULER0_PEER_AUTH_REQUEST_TIMEOUT_MS"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_PEER_AUTH_REQUEST_TIMEOUT_MS: %v", err)
		}
		config.PeerAuthRequestTimeoutMs = parsed
	}

	// Set PeerConnectRetryMax
	if val, ok := os.LookupEnv("SCHEDULER0_PEER_CONNECT_RETRY_MAX"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_PEER_CONNECT_RETRY_MAX: %v", err)
		}
		config.PeerConnectRetryMax = parsed
	}

	// Set PeerConnectRetryDelaySeconds
	if val, ok := os.LookupEnv("SCHEDULER0_PEER_CONNECT_RETRY_DELAY_SECONDS"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_PEER_CONNECT_RETRY_DELAY_SECONDS: %v", err)
		}
		config.PeerConnectRetryDelaySeconds = parsed
	}

	// Set Bootstrap
	if val, ok := os.LookupEnv("SCHEDULER0_BOOTSTRAP"); ok {
		parsed, err := strconv.ParseBool(val)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_BOOTSTRAP: %v", err)
		}
		config.Bootstrap = parsed
	}

	// Set NodeId
	if val, ok := os.LookupEnv("SCHEDULER0_NODE_ID"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_NODE_ID: %v", err)
		}
		config.NodeId = parsed
	}

	// Set RaftAddress
	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_ADDRESS"); ok {
		config.RaftAddress = val
	}

	// Set RaftTransportMaxPool
	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_TRANSPORT_MAX_POOL"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT")
		}
		config.RaftTransportMaxPool = parsed
	}

	// Set RaftTransportMaxPool
	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_TRANSPORT_TIMEOUT"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT")
		}
		config.RaftTransportTimeout = parsed
	}

	// Set RaftApplyTimeout
	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_APPLY_TIMEOUT"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_APPLY_TIMEOUT: %v", err)
		}
		config.RaftApplyTimeout = parsed
	}

	// Set RaftSnapshotInterval
	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_SNAPSHOT_INTERVAL"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_SNAPSHOT_INTERVAL: %v", err)
		}
		config.RaftSnapshotInterval = parsed
	}

	// Set RaftSnapshotThreshold
	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_SNAPSHOT_THRESHOLD"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_SNAPSHOT_THRESHOLD: %v", err)
		}
		config.RaftSnapshotThreshold = parsed
	}

	// Set RaftHeartbeatTimeout
	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_HEARTBEAT_TIMEOUT"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_HEARTBEAT_TIMEOUT: %v", err)
		}
		config.RaftHeartbeatTimeout = parsed
	}

	// Set RaftElectionTimeout
	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_ELECTION_TIMEOUT"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_ELECTION_TIMEOUT: %v", err)
		}
		config.RaftElectionTimeout = parsed
	}

	// Set RaftCommitTimeout
	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_COMMIT_TIMEOUT"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_COMMIT_TIMEOUT: %v", err)
		}
		config.RaftCommitTimeout = parsed
	}

	// Set RaftMaxAppendEntries
	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_MAX_APPEND_ENTRIES"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_MAX_APPEND_ENTRIES: %v", err)
		}
		config.RaftMaxAppendEntries = parsed
	}

	// Set JobExecutionTimeout
	if val, ok := os.LookupEnv("SCHEDULER0_JOB_EXECUTION_TIMEOUT"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_JOB_EXECUTION_TIMEOUT: %v", err)
		}
		config.JobExecutionTimeout = parsed
	}

	// Set JobExecutionRetryDelay
	if val, ok := os.LookupEnv("SCHEDULER0_JOB_EXECUTION_RETRY_DELAY"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_JOB_EXECUTION_RETRY_DELAY: %v", err)
		}
		config.JobExecutionRetryDelay = parsed
	}

	// Set JobExecutionRetryMax
	if val, ok := os.LookupEnv("SCHEDULER0_JOB_EXECUTION_RETRY_MAX"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_JOB_EXECUTION_RETRY_MAX: %v", err)
		}
		config.JobExecutionRetryMax = parsed
	}

	// Set MaxWorkers
	if val, ok := os.LookupEnv("SCHEDULER0_MAX_WORKERS"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_MAX_WORKERS: %v", err)
		}
		config.MaxWorkers = parsed
	}

	// Set JobQueueDebounceDelay
	if val, ok := os.LookupEnv("SCHEDULER0_JOB_QUEUE_DEBOUNCE_DELAY"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_JOB_QUEUE_DEBOUNCE_DELAY: %v", err)
		}
		config.JobQueueDebounceDelay = parsed
	}

	// Set MaxMemory
	if val, ok := os.LookupEnv("SCHEDULER0_MAX_MEMORY"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_MAX_MEMORY: %v", err)
		}
		config.MaxMemory = parsed
	}

	// Set ExecutionLogFetchFanIn
	if val, ok := os.LookupEnv("SCHEDULER0_EXECUTION_LOG_FETCH_FAN_IN"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_EXECUTION_LOG_FETCH_FAN_IN: %v", err)
		}
		config.ExecutionLogFetchFanIn = parsed
	}

	// Set ExecutionLogFetchIntervalSeconds
	if val, ok := os.LookupEnv("SCHEDULER0_EXECUTION_LOG_FETCH_INTERVAL_SECONDS"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_EXECUTION_LOG_FETCH_INTERVAL_SECONDS: %v", err)
		}
		config.ExecutionLogFetchIntervalSeconds = parsed
	}

	// Set JobInvocationDebounceDelay
	if val, ok := os.LookupEnv("SCHEDULER0_JOB_INVOCATION_DEBOUNCE_DELAY"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_JOB_INVOCATION_DEBOUNCE_DELAY: %v", err)
		}
		config.JobInvocationDebounceDelay = parsed
	}

	// Set HTTPExecutorPayloadMaxSizeMb
	if val, ok := os.LookupEnv("SCHEDULER0_HTTP_EXECUTOR_PAYLOAD_MAX_SIZE_MB"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_HTTP_EXECUTOR_PAYLOAD_MAX_SIZE_MB: %v", err)
		}
		config.HTTPExecutorPayloadMaxSizeMb = parsed
	}

	return config
}

func GetBinPath() string {
	e, err := os.Executable()
	if err != nil {
		log.Fatalln("failed to get path of scheduler0 binary", err.Error())
	}
	return path.Dir(e)
}
