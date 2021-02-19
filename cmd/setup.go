package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var SetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set configurations for cloud provider",
	Long:  `Configurations are required to deploy your Scheduler0`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("setup command")
	},
}
