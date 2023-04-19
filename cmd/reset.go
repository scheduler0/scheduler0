package cmd

import (
	_ "embed"
	"github.com/spf13/cobra"
	"log"
	"os"
	"scheduler0/utils"
)

var ResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "resets raft state or db",
	Long:  ``,
}

var raftCmd = &cobra.Command{
	Use:   "raft",
	Short: "resets raft state",
	Long:  `delete the raft dir`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)
		logger.Println("Reset ")

		utils.RecreateRaftDir()
		logger.Println("Cleared raft state")
	},
}

func init() {
	ResetCmd.AddCommand(raftCmd)
}
