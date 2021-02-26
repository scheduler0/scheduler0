package utils

import (
	"github.com/spf13/viper"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"os"
)

type PostgresCredentials struct {
	Addr     string `json:"addr"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

// GetPostgresCredentials this will retrieve the postgres credentials stored on disk and set it as an os env
func GetPostgresCredentials() *PostgresCredentials {
	return &PostgresCredentials{
		Addr:     os.Getenv("POSTGRES_ADDRESS"),
		Password: os.Getenv("POSTGRES_PASSWORD"),
		Database: os.Getenv("POSTGRES_DATABASE"),
		User:     os.Getenv("POSTGRES_USER"),
	}
}

// SetPostgresCredentialsFromConfig this will set the postgres environment variables to what what provided during initialization
func SetPostgresCredentialsFromConfig() {
	SetupConfig()

	err := os.Setenv("POSTGRES_ADDRESS", viper.GetString("Addr"))
	utils.CheckErr(err)

	err = os.Setenv("POSTGRES_USER", viper.GetString("User"))
	utils.CheckErr(err)

	err = os.Setenv("POSTGRES_PASSWORD", viper.GetString("Password"))
	utils.CheckErr(err)

	err = os.Setenv("POSTGRES_DATABASE", viper.GetString("Database"))
	utils.CheckErr(err)
}