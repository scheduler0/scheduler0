package main

import (
	_ "github.com/mattn/go-sqlite3"
	"scheduler0/cmd"
	"scheduler0/utils"
)

func main() {
	if err := cmd.Execute(); err != nil {
		utils.Error(err.Error())
	}
}
