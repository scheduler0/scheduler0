package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"os"
)

// VersionCmd used to get current version of scheduler0
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of scheduler0",
	Long:  `All software has versions. This is scheduler0's`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)
		logger.Println("scheduler0 v0.0.1")
	},
}
