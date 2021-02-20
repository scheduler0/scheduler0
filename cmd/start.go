package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"scheduler0/server"
)

var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a local version of the server",
	Long:  `The would run the server on your`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting Server")
		server.Start()
	},
}
