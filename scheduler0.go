package main

import (
	"scheduler0/cmd"
	"scheduler0/utils"
)

func main() {
	if err := cmd.Execute(); err != nil {
		utils.Error(err.Error())
	}
}
