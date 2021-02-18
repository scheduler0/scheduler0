package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"io/ioutil"
	"os"
	"scheduler0/cmd"
)

func createConfigFile() {
	config := cmd.Config{}
	configByte, err := json.Marshal(config)
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	err = ioutil.WriteFile(fmt.Sprintf("%v/.scheduler0", os.Getenv("HOME")), configByte, os.ModeAppend)
	if err != nil {
		panic(fmt.Errorf("Fatal unable to save scheduler 0: %s \n", err))
	}
}

func setupViper() {
	viper.SetConfigName("scheduler0")
	viper.SetConfigType("json")
	viper.SetConfigFile(fmt.Sprintf("%v/.scheduler0", os.Getenv("HOME")))

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			createConfigFile()
		} else {

		}
	} else {
		fmt.Println(fmt.Sprintf("Hello %v", viper.Get("name")))
	}
}


func main() {
	setupViper()

	utils.Info(fmt.Sprintf("Hello %v", viper.Get("Name")))

	if err := cmd.Execute(); err != nil {
		utils.Error(err.Error())
	}
}