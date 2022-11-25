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

type Scheduler0Secrets struct {
	SecretKey    string `json:"secretKey" yaml:"SecretKey"`
	AuthUsername string `json:"authUsername" yaml:"AuthUsername"`
	AuthPassword string `json:"authPassword" yaml:"AuthPassword"`
}

var cachedSecrets *Scheduler0Secrets

// GetSecrets this will retrieve scheduler0 credentials stored on disk
func GetSecrets(logger *log.Logger) *Scheduler0Secrets {
	if cachedSecrets != nil {
		return cachedSecrets
	}

	binPath := config.GetBinPath(logger)

	fs := afero.NewOsFs()
	data, err := afero.ReadFile(fs, binPath+"/"+constants.SecretsFileName)
	utils.CheckErr(err)

	secrets := Scheduler0Secrets{}

	err = json.Unmarshal(data, &secrets)
	utils.CheckErr(err)

	cachedSecrets = &secrets

	return cachedSecrets
}

func SaveSecrets(logger *log.Logger, credentialsInput *Scheduler0Secrets) *Scheduler0Secrets {
	binPath := config.GetBinPath(logger)

	fs := afero.NewOsFs()
	data, err := json.Marshal(credentialsInput)
	utils.CheckErr(err)

	err = afero.WriteFile(fs, binPath+"/"+constants.SecretsFileName, data, os.ModePerm)
	utils.CheckErr(err)

	cachedSecrets = credentialsInput

	return cachedSecrets
}
