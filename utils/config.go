package utils

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
	Protocol              string `json:"protocol" yaml:"Protocol"`
	Host                  string `json:"host" yaml:"Host"`
	Port                  string `json:"port" yaml:"Port"`
	SecretKey             string `json:"secret_key" yaml:"Secret"`
	Replicas              []Peer `json:"replicas" yaml:"Replicas"`
	Bootstrap             string `json:"bootstrap" yaml:"Bootstrap"`
	NodeId                string `json:"nodeId" yaml:"NodeId"`
	RaftAddress           string `json:"raftAddress" yaml:"RaftAddress"`
	RaftTransportMaxPool  string `json:"raftTransportMaxPool" yaml:"RaftTransportMaxPool"`
	RaftTransportTimeout  string `json:"raftTransportTimeout" yaml:"RaftTransportTimeout"`
	RaftApplyTimeout      string `json:"raftApplyTimeout" yaml:"RaftApplyTimeout"`
	RaftSnapshotInterval  string `json:"raftSnapshotInterval" yaml:"RaftSnapshotInterval"`
	RaftSnapshotThreshold string `json:"raftSnapshotThreshold" yaml:"RaftSnapshotThreshold"`
}

var cachedConfig *Scheduler0Configurations

// GetScheduler0Configurations this will retrieve scheduler0 configurations stored on disk and set it as an os env
func GetScheduler0Configurations(logger *log.Logger) *Scheduler0Configurations {
	if cachedConfig != nil {
		return cachedConfig
	}

	binPath := getBinPath(logger)

	fs := afero.NewOsFs()
	data, err := afero.ReadFile(fs, binPath+"/"+constants.ConfigFileName)
	utils.CheckErr(err)

	config := Scheduler0Configurations{}

	err = yaml.Unmarshal(data, &config)
	utils.CheckErr(err)

	cachedConfig = &config

	return cachedConfig
}

func getBinPath(logger *log.Logger) string {
	e, err := os.Executable()
	if err != nil {
		logger.Fatalln(err)
	}
	return path.Dir(e)
}
