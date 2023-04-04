package secrets

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/afero"
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
func GetSecrets() *Scheduler0Secrets {
	if cachedSecrets != nil {
		return cachedSecrets
	}

	binPath := config.GetBinPath()

	fs := afero.NewOsFs()
	data, err := afero.ReadFile(fs, binPath+"/"+constants.SecretsFileName)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	secrets := Scheduler0Secrets{}

	if os.IsNotExist(err) {
		secrets = GetSecretsFromEnv()
		cachedSecrets = &secrets

		fmt.Println("cachedSecrets", cachedSecrets)

		return cachedSecrets
	}

	err = json.Unmarshal(data, &secrets)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	cachedSecrets = &secrets

	return cachedSecrets
}

func GetSecretsFromEnv() Scheduler0Secrets {
	secrets := Scheduler0Secrets{}

	// Set LogLevel
	if val, ok := os.LookupEnv("SCHEDULER0_SECRET_KEY"); ok {
		secrets.SecretKey = val
	}

	// Set Protocol
	if val, ok := os.LookupEnv("SCHEDULER0_AUTH_PASSWORD"); ok {
		secrets.AuthPassword = val
	}

	// Set Host
	if val, ok := os.LookupEnv("SCHEDULER0_AUTH_USERNAME"); ok {
		secrets.AuthUsername = val
	}

	return secrets
}

func SaveSecrets(credentialsInput *Scheduler0Secrets) *Scheduler0Secrets {
	binPath := config.GetBinPath()

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
