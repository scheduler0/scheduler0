package main

import (
	"scheduler0/server/http_server"
	"scheduler0/utils"
)

func main() {
	utils.SetScheduler0Configurations()
	http_server.Start()
}
