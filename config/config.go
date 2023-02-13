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
		utils.CheckErr(err)

		config := Scheduler0Configurations{}

		err = yaml.Unmarshal(data, &config)
		utils.CheckErr(err)

		cachedConfig = &config
	})

	return cachedConfig
}

func GetBinPath() string {
	e, err := os.Executable()
	if err != nil {
		log.Fatalln("failed to get path of scheduler0 binary", err.Error())
	}
	return path.Dir(e)
}
