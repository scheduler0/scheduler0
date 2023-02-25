package cmd

import (
	_ "embed"
	"fmt"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"log"
	"os"
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

		dir, err := os.Getwd()
		if err != nil {
			log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
		}
		fs := afero.NewOsFs()

		recreateRaftDir(fs, dir)
		logger.Println("Cleared raft state")
	},
}

func init() {
	ResetCmd.AddCommand(raftCmd)
}
