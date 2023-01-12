package config

import (
	"github.com/spf13/afero"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"path"
	"scheduler0/constants"
	"sync"
)

type RaftNode struct {
	Address     string `json:"address" yaml:"Address"`
	RaftAddress string `json:"raft_address" yaml:"RaftAddress"`
	NodeId      int    `json:"nodeId" yaml:"NodeId"`
}

// Scheduler0Configurations global configurations
type Scheduler0Configurations struct {
	Protocol                             string     `json:"protocol" yaml:"Protocol"`
	Host                                 string     `json:"host" yaml:"Host"`
	Port                                 string     `json:"port" yaml:"Port"`
	Replicas                             []RaftNode `json:"replicas" yaml:"Replicas"`
	PeerAuthRequestTimeoutMs             int        `json:"PeerAuthRequestTimeoutMs" yaml:"PeerAuthRequestTimeoutMs"`
	PeerConnectRetryMax                  int        `json:"peerConnectRetryMax" yaml:"PeerConnectRetryMax"`
	PeerConnectRetryDelaySeconds         int        `json:"peerConnectRetryDelay" yaml:"PeerConnectRetryDelaySeconds"`
	Bootstrap                            bool       `json:"bootstrap" yaml:"Bootstrap"`
	NodeId                               int        `json:"nodeId" yaml:"NodeId"`
	RaftAddress                          string     `json:"raftAddress" yaml:"RaftAddress"`
	RaftTransportMaxPool                 int        `json:"raftTransportMaxPool" yaml:"RaftTransportMaxPool"`
	RaftTransportTimeout                 int        `json:"raftTransportTimeout" yaml:"RaftTransportTimeout"`
	RaftApplyTimeout                     int        `json:"raftApplyTimeout" yaml:"RaftApplyTimeout"`
	RaftSnapshotInterval                 int        `json:"raftSnapshotInterval" yaml:"RaftSnapshotInterval"`
	RaftSnapshotThreshold                int        `json:"raftSnapshotThreshold" yaml:"RaftSnapshotThreshold"`
	JobExecutionTimeout                  int        `json:"jobExecutionTimeout" yaml:"JobExecutionTimeout"`
	JobExecutionRetryDelay               int        `json:"jobExecutionRetryDelay" yaml:"JobExecutionRetryDelay"`
	JobExecutionRetryMax                 int        `json:"jobExecutionRetryMax" yaml:"JobExecutionRetryMax"`
	UncommittedExecutionLogsFetchTimeout int        `json:"uncommittedExecutionLogsFetchTimeout" yaml:"UncommittedExecutionLogsFetchTimeout"`
	IncomingRequestMaxWorkers            int        `json:"incomingRequestMaxWorkers" yaml:"IncomingRequestMaxWorkers"`
	IncomingRequestMaxQueue              int        `json:"incomingRequestMaxQueue" yaml:"IncomingRequestMaxQueue"`
	JobQueueDebounceDelay                int64      `json:"jobQueueDebounceDelay" yaml:"JobQueueDebounceDelay"`
	MaxMemory                            int64      `json:"maxMemory" yaml:"MaxMemory"`
	ExecutionLogFetchFanIn               int64      `json:"executionLogFetchFanIn" yaml:"ExecutionLogFetchFanIn"`
	ExecutionLogFetchIntervalSeconds     int64      `json:"executionLogFetchIntervalSeconds" yaml:"ExecutionLogFetchIntervalSeconds"`
	JobInvocationDebounceDelay           int64      `json:"jobInvocationDebounceDelay" yaml:"JobInvocationDebounceDelay"`
	HTTPExecutorPayloadMaxSizeMb         int64      `json:"httpExecutorPayloadMaxSizeMb" yaml:"HTTPExecutorPayloadMaxSizeMb"`
}

var cachedConfig *Scheduler0Configurations
var once sync.Once

// GetConfigurations this will retrieve scheduler0 configurations stored on disk
func GetConfigurations(logger *log.Logger) *Scheduler0Configurations {
	if cachedConfig != nil {
		return cachedConfig
	}

	once.Do(func() {
		binPath := GetBinPath(logger)

		fs := afero.NewOsFs()
		data, err := afero.ReadFile(fs, binPath+"/"+constants.ConfigFileName)
		utils.CheckErr(err)

		config := Scheduler0Configurations{}

		err = yaml.Unmarshal(data, &config)
		utils.CheckErr(err)

		cachedConfig = &config
	})

	return cachedConfig
}

func GetBinPath(logger *log.Logger) string {
	e, err := os.Executable()
	if err != nil {
		logger.Fatalln(err)
	}
	return path.Dir(e)
}
