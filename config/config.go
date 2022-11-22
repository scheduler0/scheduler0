package config

import (
	"github.com/spf13/afero"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"path"
	"scheduler0/constants"
)

type Peer struct {
	Address     string `json:"address" yaml:"Address"`
	RaftAddress string `json:"raft_address" yaml:"RaftAddress"`
}

// Scheduler0Configurations global configurations
type Scheduler0Configurations struct {
	Protocol                    string `json:"protocol" yaml:"Protocol"`
	Host                        string `json:"host" yaml:"Host"`
	Port                        string `json:"port" yaml:"Port"`
	Replicas                    []Peer `json:"replicas" yaml:"Replicas"`
	PeerCronJobCheckInterval    int    `json:"PeerCronJobCheckInterval" yaml:"PeerCronJobCheckInterval"`
	PeerAuthRequestTimeout      int    `json:"PeerAuthRequestTimeout" yaml:"PeerAuthRequestTimeout"`
	MonitorRaftStateInterval    int    `json:"monitorRaftStateInterval" yaml:"MonitorRaftStateInterval"`
	PeerConnectRetryMax         int    `json:"peerConnectRetryMax" yaml:"PeerConnectRetryMax"`
	PeerConnectRetryDelay       int    `json:"peerConnectRetryDelay" yaml:"PeerConnectRetryDelay"`
	StableRaftStateTimeout      int    `json:"stableRaftStateTimeout" yaml:"StableRaftStateTimeout"`
	Bootstrap                   string `json:"bootstrap" yaml:"Bootstrap"`
	NodeId                      string `json:"nodeId" yaml:"NodeId"`
	RaftAddress                 string `json:"raftAddress" yaml:"RaftAddress"`
	RaftTransportMaxPool        string `json:"raftTransportMaxPool" yaml:"RaftTransportMaxPool"`
	RaftTransportTimeout        string `json:"raftTransportTimeout" yaml:"RaftTransportTimeout"`
	RaftApplyTimeout            string `json:"raftApplyTimeout" yaml:"RaftApplyTimeout"`
	RaftSnapshotInterval        string `json:"raftSnapshotInterval" yaml:"RaftSnapshotInterval"`
	RaftSnapshotThreshold       string `json:"raftSnapshotThreshold" yaml:"RaftSnapshotThreshold"`
	JobExecutionTimeout         int    `json:"jobExecutionTimeout" yaml:"JobExecutionTimeout"`
	JobExecutionLogBackupSizeKb int    `json:"jobExecutionLogBackupSizeKb" yaml:"JobExecutionLogBackupSizeKb"`
	RequireNetworkTimeProtocol  int    `json:"requireNetworkTimeProtocol" yaml:"RequireNetworkTimeProtocol"`
	NetworkTimeProtocolHost     string `json:"networkTimeProtocolHost" yaml:"NetworkTimeProtocolHost"`
	JobExecutionRetryDelay      int    `json:"jobExecutionRetryDelay" yaml:"JobExecutionRetryDelay"`
	JobExecutionRetryMax        int    `json:"jobExecutionRetryMax" yaml:"JobExecutionRetryMax"`
}

var cachedConfig *Scheduler0Configurations

// GetScheduler0Configurations this will retrieve scheduler0 configurations stored on disk
func GetScheduler0Configurations(logger *log.Logger) *Scheduler0Configurations {
	if cachedConfig != nil {
		return cachedConfig
	}

	binPath := GetBinPath(logger)

	fs := afero.NewOsFs()
	data, err := afero.ReadFile(fs, binPath+"/"+constants.ConfigFileName)
	utils.CheckErr(err)

	config := Scheduler0Configurations{}

	err = yaml.Unmarshal(data, &config)
	utils.CheckErr(err)

	cachedConfig = &config

	return cachedConfig
}

func GetBinPath(logger *log.Logger) string {
	e, err := os.Executable()
	if err != nil {
		logger.Fatalln(err)
	}
	return path.Dir(e)
}
