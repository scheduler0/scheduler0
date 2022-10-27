package main

import (
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"scheduler0/cmd"
)

func main() {
	logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)

	if err := cmd.Execute(); err != nil {
		logger.Fatalln(err.Error())
	}
}
