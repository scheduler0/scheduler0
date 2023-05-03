package cmd

import (
	_ "embed"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"log"
	"os"
	"scheduler0/config"
	"scheduler0/db"
	"scheduler0/utils"
)

// ConfigCmd configuration protobuffs
var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "create, view or modify scheduler0 configurations",
	Long: `
The database configurations are required to run the scheduler0 server, 
the port configuration is optional by default the server will use :9090.

Usage:

	config init

This will go through the configuration init flow.
Note that starting the server without going through the init flow will not work.
`,
}

// InitCmd initializes scheduler0 configuration
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize database configurations and port for the scheduler0 server",
	Long: `
Initialize postgres credentials for the Scheduler0 server,
you will be prompted to provide postgres credentials

Usage:

	scheduler0 init

Note that the Port is optional. By default the server will use :9090
`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)

		logger.Println("Initializing Scheduler0 Configuration")
		config := config.GetConfigurations()

		cmdLogger := hclog.New(&hclog.LoggerOptions{
			Name:  "scheduler0-cmd",
			Level: hclog.LevelFromString(config.LogLevel),
		})

		utils.RemoveSqliteDbDir()
		utils.RemoveRaftDir()
		db.RunMigration(cmdLogger)

		logger.Println("Scheduler0 Initialized")
	},
}

func init() {
	ConfigCmd.AddCommand(initCmd)
}
