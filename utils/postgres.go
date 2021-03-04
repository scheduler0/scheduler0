package utils

import (
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
}

const (
	PostgresAddressEnv  = "POSTGRES_ADDRESS"
	PostgresPasswordEnv = "POSTGRES_PASSWORD"
	PostgresDatabaseEnv = "POSTGRES_DATABASE"
	PostgresUserEnv     = "POSTGRES_USER"
	PostgresURLEnv      = "POSTGRES_URL"
	PortEnv             = "PORT"
)

// GetScheduler0Configurations this will retrieve scheduler0 configurations stored on disk and set it as an os env
func GetScheduler0Configurations() *Scheduler0Configurations {
	return &Scheduler0Configurations{
		PostgresAddress:  os.Getenv(PostgresAddressEnv),
		PostgresPassword: os.Getenv(PostgresPasswordEnv),
		PostgresDatabase: os.Getenv(PostgresDatabaseEnv),
		PostgresUser:     os.Getenv(PostgresUserEnv),
		PostgresURL:      os.Getenv(PostgresURLEnv),
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

	err = os.Setenv(PortEnv, viper.GetString("port"))
	utils.CheckErr(err)
}
