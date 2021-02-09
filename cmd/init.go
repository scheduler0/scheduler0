package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Scheduler0",
	Long:  `Initialize configurations are required to start and deploy your Scheduler0`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("setup command")
	},
}