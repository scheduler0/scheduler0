package utils

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/spf13/viper"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"os"
)

type Scheduler0Configurations struct {
	PostgresAddress  string `json:"postgres_address"`
	PostgresUser     string `json:"postgres_user"`
	PostgresPassword string `json:"postgres_password"`
	PostgresDatabase string `json:"postgres_database"`
	PostgresURL      string `json:"postgres_url"`
	PORT             string `json:"port"`
	SecretKey        string `json:"secret_key"`
}

const (
	PostgresAddressEnv  = "SCHEDULER0_POSTGRES_ADDRESS"
	PostgresPasswordEnv = "SCHEDULER0_POSTGRES_PASSWORD"
	PostgresDatabaseEnv = "SCHEDULER0_POSTGRES_DATABASE"
	PostgresUserEnv     = "SCHEDULER0_POSTGRES_USER"
	PostgresURLEnv      = "SCHEDULER0_POSTGRES_URL"
	SecretKeyEnv        = "SCHEDULER0_SECRET_KEY"
	PortEnv             = "SCHEDULER0_PORT"
)

// GetScheduler0Configurations this will retrieve scheduler0 configurations stored on disk and set it as an os env
func GetScheduler0Configurations() *Scheduler0Configurations {
	return &Scheduler0Configurations{
		PostgresAddress:  os.Getenv(PostgresAddressEnv),
		PostgresPassword: os.Getenv(PostgresPasswordEnv),
		PostgresDatabase: os.Getenv(PostgresDatabaseEnv),
		PostgresUser:     os.Getenv(PostgresUserEnv),
		PostgresURL:      os.Getenv(PostgresURLEnv),
		SecretKey:        os.Getenv(SecretKeyEnv),
		PORT:             os.Getenv(PortEnv),
	}
}

// SetScheduler0Configurations this will set the server environment variables to initialized values
func SetScheduler0Configurations() {
	SetupConfig()

	err := os.Setenv(PostgresAddressEnv, viper.GetString("postgres_address"))
	utils.CheckErr(err)

	err = os.Setenv(PostgresUserEnv, viper.GetString("postgres_user"))
	utils.CheckErr(err)

	err = os.Setenv(PostgresPasswordEnv, viper.GetString("postgres_password"))
	utils.CheckErr(err)

	err = os.Setenv(PostgresDatabaseEnv, viper.GetString("postgres_database"))
	utils.CheckErr(err)

	err = os.Setenv(PostgresURLEnv, viper.GetString("postgres_url"))
	utils.CheckErr(err)

	err = os.Setenv(SecretKeyEnv, viper.GetString("secret_key"))
	utils.CheckErr(err)

	err = os.Setenv(PortEnv, viper.GetString("port"))
	utils.CheckErr(err)
}

// SetTestScheduler0Configurations this will set the server environment variables to initialized values
func SetTestScheduler0Configurations() {
	SetupConfig()

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
