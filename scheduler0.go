package main

import (
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"scheduler0/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		utils.Error(err.Error())
	}
}
