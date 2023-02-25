package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"os"
)

var entityType = ""

// ListCmd returns list of scheduler0 credentials
var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "list credentials, job, projects and executions",
	Long:  ``,
}

var listCredentialCmd = &cobra.Command{
	Use: "credential",
	Long: `Use this to list entities like credentials, projects, jobs and executions. 

Usage: 

> scheduler0 list credentials

This will list all the credentials that you can use in the client sdks`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)
		logger.Println("not implemented")
	},
}

func init() {
	ListCmd.AddCommand(listCredentialCmd)
}
