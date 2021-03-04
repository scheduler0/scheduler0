package cmd

import "github.com/spf13/cobra"

var CreateCmd = &cobra.Command{
	Use: "create",
	Short: "create a credential or project",
	Long: `
Use this command to create credentials for any client or a project. 

Usage:
	scheduler0 create credentials --client server

This will an api key and api secret for a server side backend.
For other clients such as Android, iOS and Web an restriction key is required.
`,
	Run: func(cmd *cobra.Command, args []string) {
		
	},
}
