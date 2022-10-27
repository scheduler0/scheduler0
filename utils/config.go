package utils

import (
	"fmt"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
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

type Scheduler0Credentials struct {
	ApiKey    string `json:"api_key"`
	ApiSecret string `json:"api_secret"`
}

const (
	SecretKeyEnv             = "SCHEDULER0_SECRET_KEY"
	PortEnv                  = "SCHEDULER0_PORT"
	BootstrapEnv             = "SCHEDULER0_RAFT_BOOTSTRAP"
	NodeIdEnv                = "SCHEDULER0_NODE_ID_ENV"
	RaftAddressEnv           = "SCHEDULER0_RAFT_ADDRESS_ENV"
	RaftTransportMaxPoolEnv  = "SCHEDULER0_RAFT_TRANSPORT_MAX_POOL_ENV"
	RaftTransportTimeoutEnv  = "SCHEDULER0_RAFT_TRANSPORT_TIMEOUT_ENV"
	RaftApplyTimeoutEnv      = "SCHEDULER0_RAFT_APPLY_TIMEOUT"
	RaftSnapshotThresholdEnv = "SCHEDULER0_RAFT_SNAPSHOT_THRESHOLD"
	RaftSnapshotIntervalEnv  = "SCHEDULER0_RAFT_SNAPSHOT_INTERVAL"
)

var requiredEnvs = []string{
	SecretKeyEnv,
	BootstrapEnv,
	NodeIdEnv,
	RaftAddressEnv,
	PortEnv,
	RaftTransportMaxPoolEnv,
	RaftTransportTimeoutEnv,
	RaftApplyTimeoutEnv,
	RaftSnapshotThresholdEnv,
	RaftSnapshotIntervalEnv,
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

// CheckRequiredEnvs returns true if all the envs requires are set otherwise returns a list of missing required env
func CheckRequiredEnvs() (bool, []string) {
	missingRequiredEnvs := []string{}

	for _, requiredEnv := range requiredEnvs {
		if len(os.Getenv(requiredEnv)) < 1 {
			missingRequiredEnvs = append(missingRequiredEnvs, requiredEnv)
		}
	}

	if len(missingRequiredEnvs) > 0 {
		return false, missingRequiredEnvs
	}

	return true, nil
}

func getBinPath(logger *log.Logger) string {
	e, err := os.Executable()
	if err != nil {
		logger.Fatalln(err)
	}
	return path.Dir(e)
}

// ReadCredentialsFile reads the content of the config file created by the init protobuffs
func ReadCredentialsFile(logger *log.Logger) Scheduler0Credentials {
	defer func() {
		if r := recover(); r != nil {
			logger.Println("Recovered in f", r)
		}
	}()
	viper.SetConfigName("scheduler0")
	viper.SetConfigType("json")

	dir, err := os.Getwd()
	if err != nil {
		logger.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	configFilePath := fmt.Sprintf("%v/%v", dir, constants.CredentialsFileName)

	viper.SetConfigFile(configFilePath)
	if viperErr := viper.ReadInConfig(); viperErr != nil {
		logger.Fatalln(fmt.Errorf("Fatal error read credentials file: %s \n", err))
	}

	credentials := Scheduler0Credentials{
		ApiSecret: viper.GetString("api_secret"),
		ApiKey:    viper.GetString("api_key"),
	}

	return credentials
}
