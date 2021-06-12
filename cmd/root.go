package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "scheduler0",
	Short: "Scheduler0 is a simple job scheduling server",
	Long: `
Simple job scheduling server 
Read more documentation on https://scheduler0.com
`,
	Run: func(cmd *cobra.Command, args []string) {},
}

func init() {
	rootCmd.AddCommand(CreateCmd)
	rootCmd.AddCommand(ListCmd)
	rootCmd.AddCommand(VersionCmd)
	rootCmd.AddCommand(StartCmd)
	rootCmd.AddCommand(ConfigCmd)
}

// Execute executes root command
func Execute() error {
	return rootCmd.Execute()
}
