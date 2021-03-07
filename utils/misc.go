package utils

import (
	"fmt"
	"github.com/spf13/viper"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"os"
)

// GetPort returns the port in which the server should be running
func GetPort() string {
	port := os.Getenv("PORT")

	if len(port) == 0 {
		port = "9090"
	}

	return ":" + port
}


// GetAuthentication returns basic auth used to for identifying request from the dashboard
func GetAuthentication() (string, string) {
	usernameEnv := os.Getenv("username")
	passwordEnv := os.Getenv("password")

	username := "admin"
	password := "admin"

	if len(usernameEnv) == 0 {
		username += usernameEnv
	}

	if len(passwordEnv) == 0 {
		password += passwordEnv
	}

	return username, password
}

// SetupConfig this will read the config file created by the initialize command
func SetupConfig() {
	viper.SetConfigName("scheduler0")
	viper.SetConfigType("json")
	viper.SetConfigFile(fmt.Sprintf("%v/.scheduler0", os.Getenv("HOME")))

	if err := viper.ReadInConfig(); err != nil {
		utils.Info("To fix this error try executing the init command")
		utils.Error(err.Error())
	}
}