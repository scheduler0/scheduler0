package cmd

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"scheduler0/pkg/constants"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var ResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "resets raft state or db",
	Long:  ``,
}

var nodeIdFlag string

var raftCmd = &cobra.Command{
	Use:   "raft",
	Short: "resets raft state",
	Long:  `delete the raft dir`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)

		// Check if raft directory exists
		dir, err := os.Getwd()
		if err != nil {
			logger.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
		}
		fs := afero.NewOsFs()

		var raftDirPath string
		var warningMsg string

		if nodeIdFlag != "" {
			// Reset specific node subdirectory
			raftDirPath = fmt.Sprintf("%v/%v/%v", dir, constants.RaftDir, nodeIdFlag)
			warningMsg = fmt.Sprintf("WARNING: This will DELETE Raft consensus state for node %s. This may break cluster consistency if run on an active node. Are you sure you want to continue? [y/N]:", nodeIdFlag)
		} else {
			// Reset entire raft directory
			raftDirPath = fmt.Sprintf("%v/%v", dir, constants.RaftDir)
			warningMsg = "WARNING: This will DELETE all Raft consensus state. This may break cluster consistency if run on an active node. Are you sure you want to continue? [y/N]:"
		}

		raftExists, err := afero.DirExists(fs, raftDirPath)
		if err != nil {
			logger.Fatalln(fmt.Errorf("Fatal error checking raft dir exist: %s \n", err))
		}

		// Prompt for confirmation if directory exists
		if raftExists {
			confirmPrompt := promptui.Prompt{
				Label:       warningMsg,
				HideEntered: false,
				Default:     "N",
			}
			confirm, err := confirmPrompt.Run()
			if err != nil {
				logger.Fatalln("Failed to read confirmation:", err)
			}

			confirm = strings.ToLower(strings.TrimSpace(confirm))
			if confirm != "y" && confirm != "yes" {
				logger.Println("Operation cancelled.")
				return
			}
		} else {
			if nodeIdFlag != "" {
				logger.Printf("Raft directory for node %s does not exist. Nothing to reset.\n", nodeIdFlag)
			} else {
				logger.Println("Raft directory does not exist. Nothing to reset.")
			}
			return
		}

		// Remove the directory
		removeErr := fs.RemoveAll(raftDirPath)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			logger.Fatalln(fmt.Errorf("Fatal failed to remove raft dir: %s \n", removeErr))
		}

		if nodeIdFlag != "" {
			logger.Printf("Cleared raft state for node %s\n", nodeIdFlag)
		} else {
			logger.Println("Cleared raft state")
		}
	},
}

func init() {
	raftCmd.Flags().StringVarP(&nodeIdFlag, "nodeId", "n", "", "Node ID to reset (if not provided, resets entire raft directory)")
	ResetCmd.AddCommand(raftCmd)
}
