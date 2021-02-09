package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var SetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set configurations",
	Long:  `Configurations are required to start and deploy your Scheduler0`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("setup command")
	},
}