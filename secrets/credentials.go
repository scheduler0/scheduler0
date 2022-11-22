package secrets

import (
	"encoding/json"
	"github.com/spf13/afero"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"log"
	"os"
	"scheduler0/config"
	"scheduler0/constants"
)

type Scheduler0Credentials struct {
	SecretKey    string `json:"secretKey" yaml:"SecretKey"`
	AuthUsername string `json:"authUsername" yaml:"AuthUsername"`
	AuthPassword string `json:"authPassword" yaml:"AuthPassword"`
}

var cachedCredentials *Scheduler0Credentials

// GetScheduler0Credentials this will retrieve scheduler0 credentials stored on disk
func GetScheduler0Credentials(logger *log.Logger) *Scheduler0Credentials {
	if cachedCredentials != nil {
		return cachedCredentials
	}

	binPath := config.GetBinPath(logger)

	fs := afero.NewOsFs()
	data, err := afero.ReadFile(fs, binPath+"/"+constants.CredentialsFileName)
	utils.CheckErr(err)

	credentials := Scheduler0Credentials{}

	err = json.Unmarshal(data, &credentials)
	utils.CheckErr(err)

	cachedCredentials = &credentials

	return cachedCredentials
}

func SaveCredentials(logger *log.Logger, credentialsInput *Scheduler0Credentials) *Scheduler0Credentials {
	binPath := config.GetBinPath(logger)

	fs := afero.NewOsFs()
	data, err := json.Marshal(credentialsInput)
	utils.CheckErr(err)

	err = afero.WriteFile(fs, binPath+"/"+constants.CredentialsFileName, data, os.ModePerm)
	utils.CheckErr(err)

	cachedCredentials = credentialsInput

	return cachedCredentials
}
