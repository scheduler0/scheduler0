package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "scheduler0",
	Short: "Scheduler0 is a highly configurable and scalable scheduler",
	Long: `Scheduler0 can be deployed on AWS, Google Cloud and Azure. 
Read more documentation on https://scheduler0.com
`,
	Args:      cobra.OnlyValidArgs,
	ValidArgs: []string{"help", "version", "update", "setup", "help", "start", "deploy"},
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func init() {
	rootCmd.AddCommand(ListCmd)
	rootCmd.AddCommand(VersionCmd)
	rootCmd.AddCommand(StartCmd)
	rootCmd.AddCommand(SetupCmd)
	rootCmd.AddCommand(InitCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
