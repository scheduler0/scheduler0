package cmd

import (
	"github.com/spf13/cobra"
	"scheduler0/server/http_server"
	"scheduler0/utils"
)

var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start scheduler0 http server",
	Long: `
This start command will spin up the http server. 
The server will be ready to receive request on the PORT specified during init otherwise use :9090

Usage: 

> scheduler0 start

The server needs to be running in order to execute jobs.
`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Info("Starting Server.")
		utils.SetScheduler0Configurations()
		http_server.Start()
	},
}
