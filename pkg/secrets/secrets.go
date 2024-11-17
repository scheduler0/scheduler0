package secrets

import (
	"encoding/json"
	"github.com/spf13/afero"
	"os"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/utils"
)

type Scheduler0Secrets interface {
	GetSecrets() *scheduler0Secrets
	SaveSecrets(credentialsInput *scheduler0Secrets) *scheduler0Secrets
}

type scheduler0Secrets struct {
	SecretKey    string `json:"secretKey" yaml:"SecretKey"`
	AuthUsername string `json:"authUsername" yaml:"AuthUsername"`
	AuthPassword string `json:"authPassword" yaml:"AuthPassword"`
}

func NewScheduler0Secrets() *scheduler0Secrets {
	return &scheduler0Secrets{}
}

var cachedSecrets *scheduler0Secrets

// GetSecrets this will retrieve scheduler0 credentials stored on disk
func (_ *scheduler0Secrets) GetSecrets() *scheduler0Secrets {
	if cachedSecrets != nil {
		return cachedSecrets
	}

	binPath := utils.GetBinPath()

	fs := afero.NewOsFs()
	data, err := afero.ReadFile(fs, binPath+"/"+constants.SecretsFileName)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	secrets := scheduler0Secrets{}

	if os.IsNotExist(err) {
		secrets = getSecretsFromEnv()
	} else {
		err = json.Unmarshal(data, &secrets)
		if err != nil && !os.IsNotExist(err) {
			panic(err)
		}
	}

	cachedSecrets = &secrets
	return cachedSecrets
}

// SaveSecrets saves the secrets into a .scheduler0 file
func (_ *scheduler0Secrets) SaveSecrets(credentialsInput *scheduler0Secrets) *scheduler0Secrets {
	binPath := utils.GetBinPath()

	fs := afero.NewOsFs()
	data, err := json.Marshal(credentialsInput)
	if err != nil {
		panic(err)
	}

	err = afero.WriteFile(fs, binPath+"/"+constants.SecretsFileName, data, os.ModePerm)
	if err != nil {
		panic(err)
	}

	cachedSecrets = credentialsInput

	return cachedSecrets
}

func getSecretsFromEnv() scheduler0Secrets {
	secrets := scheduler0Secrets{}

	if val, ok := os.LookupEnv("SCHEDULER0_SECRET_KEY"); ok {
		secrets.SecretKey = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_AUTH_PASSWORD"); ok {
		secrets.AuthPassword = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_AUTH_USERNAME"); ok {
		secrets.AuthUsername = val
	}

	return secrets
}
