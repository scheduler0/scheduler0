package main

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"scheduler0/cmd"
	"scheduler0/server/src/utils"
)

func setupViper() {
	viper.SetConfigName("scheduler0")
	viper.SetConfigType("json")
	viper.SetConfigFile(fmt.Sprintf("%v/.scheduler0", os.Getenv("HOME")))

	if err := viper.ReadInConfig(); err != nil {
		utils.Error(err.Error())
	}
}

func main() {
	setupViper()
	if err := cmd.Execute(); err != nil {
		utils.Error(err.Error())
	}
}
