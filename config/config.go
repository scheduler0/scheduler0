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

type RaftNode struct {
	Address     string `json:"address" yaml:"Address"`
	RaftAddress string `json:"raft_address" yaml:"RaftAddress"`
	NodeId      uint64 `json:"nodeId" yaml:"NodeId"`
}

// Scheduler0Configurations global configurations
type Scheduler0Configurations struct {
	LogLevel                             string     `json:"logLevel" yaml:"LogLevel"`
	Protocol                             string     `json:"protocol" yaml:"Protocol"`
	Host                                 string     `json:"host" yaml:"Host"`
	Port                                 string     `json:"port" yaml:"Port"`
	Replicas                             []RaftNode `json:"replicas" yaml:"Replicas"`
	PeerAuthRequestTimeoutMs             uint64     `json:"PeerAuthRequestTimeoutMs" yaml:"PeerAuthRequestTimeoutMs"`
	PeerConnectRetryMax                  uint64     `json:"peerConnectRetryMax" yaml:"PeerConnectRetryMax"`
	PeerConnectRetryDelaySeconds         uint64     `json:"peerConnectRetryDelay" yaml:"PeerConnectRetryDelaySeconds"`
	Bootstrap                            bool       `json:"bootstrap" yaml:"Bootstrap"`
	NodeId                               uint64     `json:"nodeId" yaml:"NodeId"`
	RaftAddress                          string     `json:"raftAddress" yaml:"RaftAddress"`
	RaftTransportMaxPool                 uint64     `json:"raftTransportMaxPool" yaml:"RaftTransportMaxPool"`
	RaftTransportTimeout                 uint64     `json:"raftTransportTimeout" yaml:"RaftTransportTimeout"`
	RaftApplyTimeout                     uint64     `json:"raftApplyTimeout" yaml:"RaftApplyTimeout"`
	RaftSnapshotInterval                 uint64     `json:"raftSnapshotInterval" yaml:"RaftSnapshotInterval"`
	RaftSnapshotThreshold                uint64     `json:"raftSnapshotThreshold" yaml:"RaftSnapshotThreshold"`
	JobExecutionTimeout                  uint64     `json:"jobExecutionTimeout" yaml:"JobExecutionTimeout"`
	JobExecutionRetryDelay               uint64     `json:"jobExecutionRetryDelay" yaml:"JobExecutionRetryDelay"`
	JobExecutionRetryMax                 uint64     `json:"jobExecutionRetryMax" yaml:"JobExecutionRetryMax"`
	UncommittedExecutionLogsFetchTimeout uint64     `json:"uncommittedExecutionLogsFetchTimeout" yaml:"UncommittedExecutionLogsFetchTimeout"`
	MaxWorkers                           uint64     `json:"maxWorkers" yaml:"MaxWorkers"`
	MaxQueue                             uint64     `json:"maxQueue" yaml:"MaxQueue"`
	JobQueueDebounceDelay                uint64     `json:"jobQueueDebounceDelay" yaml:"JobQueueDebounceDelay"`
	MaxMemory                            uint64     `json:"maxMemory" yaml:"MaxMemory"`
	ExecutionLogFetchFanIn               uint64     `json:"executionLogFetchFanIn" yaml:"ExecutionLogFetchFanIn"`
	ExecutionLogFetchIntervalSeconds     uint64     `json:"executionLogFetchIntervalSeconds" yaml:"ExecutionLogFetchIntervalSeconds"`
	JobInvocationDebounceDelay           uint64     `json:"jobInvocationDebounceDelay" yaml:"JobInvocationDebounceDelay"`
	HTTPExecutorPayloadMaxSizeMb         uint64     `json:"httpExecutorPayloadMaxSizeMb" yaml:"HTTPExecutorPayloadMaxSizeMb"`
}

var cachedConfig *Scheduler0Configurations
var once sync.Once

// GetConfigurations this will retrieve scheduler0 configurations stored on disk
func GetConfigurations() *Scheduler0Configurations {
	if cachedConfig != nil {
		return cachedConfig
	}

	once.Do(func() {
		binPath := GetBinPath()

		fs := afero.NewOsFs()
		data, err := afero.ReadFile(fs, binPath+"/"+constants.ConfigFileName)
		if err != nil && !os.IsNotExist(err) {
			panic(err)
		}

		config := Scheduler0Configurations{}

		if os.IsNotExist(err) {
			config = *GetConfigFromEnv()
		}

		err = yaml.Unmarshal(data, &config)
		if err != nil {
			panic(err)
		}
		cachedConfig = &config
	})

	return cachedConfig
}

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

	// Set UncommittedExecutionLogsFetchTimeout
	if val, ok := os.LookupEnv("SCHEDULER0_UNCOMMITTED_EXECUTION_LOGS_FETCH_TIMEOUT"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_UNCOMMITTED_EXECUTION_LOGS_FETCH_TIMEOUT: %v", err)
		}
		config.UncommittedExecutionLogsFetchTimeout = parsed
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
