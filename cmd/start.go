package cmd

import (
	"github.com/spf13/cobra"
	"scheduler0/server/http_server"
	"scheduler0/utils"
)

var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a local version of the server",
	Long:  `The would run the server on your`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Info("Starting Server.")
		utils.SetPostgresCredentialsFromConfig()
		http_server.Start()
	},
}
