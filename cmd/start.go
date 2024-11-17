package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"os"
	http_server "scheduler0/pkg/http/server"
)

// StartCmd http server command
var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start scheduler0 http server",
	Long: `
This start command will spin up the http server. 
The server will be ready to receive request on the Port specified during init otherwise use :9090

Usage: 

> scheduler0 start

The server needs to be running in order to execute jobs.
`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)
		logger.Println("Starting Server.")
		http_server.Start()
	},
}
