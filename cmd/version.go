package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

// VersionCmd used to get current version of scheduler0
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of scheduler0",
	Long:  `All software has versions. This is scheduler0's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("scheduler0 v0.0.1")
	},
}
