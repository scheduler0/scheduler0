package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"gopkg.in/yaml.v2"
	"os"
	"path"
	"strings"
)

// Scheduler0Configurations global configurations
type Scheduler0Configurations struct {
	PostgresAddress  string   `json:"postgres_address" yaml:"PostgresHost"`
	PostgresUser     string   `json:"postgres_user" yaml:"PostgresUser"`
	PostgresPassword string   `json:"postgres_password" yaml:"PostgresPassword"`
	PostgresDatabase string   `json:"postgres_database" yaml:"PostgresDatabase"`
	PORT             string   `json:"port" yaml:"Port"`
	MaxMemory        string   `json:"max_memory" yaml:"MaxMemory"`
	SecretKey        string   `json:"secret_key" yaml:"Secret"`
	Replicas         []string `json:"replicas" yaml:"Replicas"`
}

const (
	PostgresAddressEnv  = "SCHEDULER0_POSTGRES_ADDRESS"
	PostgresPasswordEnv = "SCHEDULER0_POSTGRES_PASSWORD"
	PostgresDatabaseEnv = "SCHEDULER0_POSTGRES_DATABASE"
	PostgresUserEnv     = "SCHEDULER0_POSTGRES_USER"
	SecretKeyEnv        = "SCHEDULER0_SECRET_KEY"
	MaxMemory           = "SCHEDULER0_MAX_MEMORY"
	Replicas            = "SCHEDULER0_REPLICAS"
	PortEnv             = "SCHEDULER0_PORT"
)

var requiredEnvs = []string{
	"SCHEDULER0_POSTGRES_ADDRESS",
	"SCHEDULER0_POSTGRES_PASSWORD",
	"SCHEDULER0_POSTGRES_DATABASE",
	"SCHEDULER0_POSTGRES_USER",
	"SCHEDULER0_SECRET_KEY",
}

// GetScheduler0Configurations this will retrieve scheduler0 configurations stored on disk and set it as an os env
func GetScheduler0Configurations() *Scheduler0Configurations {
	replicasStr := os.Getenv(Replicas)
	replicas := strings.Split(replicasStr, ",")

	return &Scheduler0Configurations{
		PostgresAddress:  os.Getenv(PostgresAddressEnv),
		PostgresPassword: os.Getenv(PostgresPasswordEnv),
		PostgresDatabase: os.Getenv(PostgresDatabaseEnv),
		PostgresUser:     os.Getenv(PostgresUserEnv),
		SecretKey:        os.Getenv(SecretKeyEnv),
		Replicas:         replicas,
		MaxMemory:        os.Getenv(MaxMemory),
		PORT:             os.Getenv(PortEnv),
	}
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

func getBinPath() string {
	e, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return path.Dir(e)
}

func ReadRootConfigFile() {
	binPath := getBinPath()

	fs := afero.NewOsFs()
	data, err := afero.ReadFile(fs, binPath+"/config.yml")
	utils.CheckErr(err)

	config := Scheduler0Configurations{}

	err = yaml.Unmarshal(data, &config)
	utils.CheckErr(err)

	err = os.Setenv(PostgresAddressEnv, config.PostgresAddress)
	utils.CheckErr(err)

	err = os.Setenv(PostgresUserEnv, config.PostgresUser)
	utils.CheckErr(err)

	err = os.Setenv(PostgresPasswordEnv, config.PostgresPassword)
	utils.CheckErr(err)

	err = os.Setenv(PostgresDatabaseEnv, config.PostgresDatabase)
	utils.CheckErr(err)

	err = os.Setenv(SecretKeyEnv, config.SecretKey)
	utils.CheckErr(err)

	err = os.Setenv(PortEnv, config.PORT)
	utils.CheckErr(err)
}

// ReadInitCMDConfigIntoProcessEnv reads the content of the config file created by the init command
func ReadInitCMDConfigIntoProcessEnv() {
	viper.SetConfigName("scheduler0")
	viper.SetConfigType("json")
	viper.SetConfigFile(fmt.Sprintf("%v/.scheduler0", os.Getenv("HOME")))

	if err := viper.ReadInConfig(); err != nil {
		return
	}

	err := os.Setenv(PostgresAddressEnv, viper.GetString("postgres_address"))
	utils.CheckErr(err)

	err = os.Setenv(PostgresUserEnv, viper.GetString("postgres_user"))
	utils.CheckErr(err)

	err = os.Setenv(PostgresPasswordEnv, viper.GetString("postgres_password"))
	utils.CheckErr(err)

	err = os.Setenv(PostgresDatabaseEnv, viper.GetString("postgres_database"))
	utils.CheckErr(err)

	err = os.Setenv(SecretKeyEnv, viper.GetString("secret_key"))
	utils.CheckErr(err)

	err = os.Setenv(PortEnv, viper.GetString("port"))
	utils.CheckErr(err)
}

// SetScheduler0Configurations this will set the server environment variables to initialized values
func SetScheduler0Configurations() {
	check, missingEnv := CheckRequiredEnvs()
	if check {
		return
	}

	ReadRootConfigFile()
	check, _ = CheckRequiredEnvs()
	if check {
		return
	}

	ReadInitCMDConfigIntoProcessEnv()
	lastCheck, missingEnv := CheckRequiredEnvs()
	if !lastCheck  {
		panic(errors.New(fmt.Sprintf("the following required environment variable has not been provided: %v", strings.Join(missingEnv, ","))))
	}
}

// SetTestScheduler0Configurations this will set the server environment variables to initialized values
func SetTestScheduler0Configurations() {
	bytes := make([]byte, 32) //generate a random 32 byte key for AES-256
	if _, err := rand.Read(bytes); err != nil {
		panic(err.Error())
	}
	key := hex.EncodeToString(bytes) //encode key in bytes to string for saving

	err := os.Setenv(PostgresAddressEnv, "localhost:5432")
	utils.CheckErr(err)

	err = os.Setenv(PostgresUserEnv, "core")
	utils.CheckErr(err)

	err = os.Setenv(PostgresPasswordEnv, "localdev")
	utils.CheckErr(err)

	err = os.Setenv(PostgresDatabaseEnv, "scheduler0_test")
	utils.CheckErr(err)

	err = os.Setenv(SecretKeyEnv, key)
	utils.CheckErr(err)

	err = os.Setenv(PortEnv, "9090")
	utils.CheckErr(err)
}
