package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"scheduler0-private/pkg/constants"
	"scheduler0-private/pkg/db"
	"scheduler0-private/pkg/utils"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var InitCmd = &cobra.Command{
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
		cmdLogger := hclog.New(&hclog.LoggerOptions{
			Name:  "scheduler0-cmd",
			Level: hclog.LevelFromString("DEBUG"),
		})

		cmdLogger.Info("Initializing Scheduler0 Configuration")

		dbDirPath, _ := utils.GetSqliteDbDirAndDbFilePath()
		fs := afero.NewOsFs()
		dbExists, _ := afero.DirExists(fs, dbDirPath)

		dir, _ := os.Getwd()
		raftDirPath := fmt.Sprintf("%v/%v", dir, constants.RaftDir)
		raftExists, _ := afero.DirExists(fs, raftDirPath)
		if dbExists || raftExists {
			var warningMsg strings.Builder
			warningMsg.WriteString("WARNING: This operation will DELETE existing data:\n")
			if dbExists {
				warningMsg.WriteString("  - SQLite database directory\n")
			}
			if raftExists {
				warningMsg.WriteString("  - Raft state directory\n")
			}
			warningMsg.WriteString("\nAre you sure you want to continue? [y/N]:")

			fmt.Println(warningMsg.String())
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "yes" {
				fmt.Println("Operation cancelled.")
				return
			}
		}

		utils.RemoveSqliteDbDir()
		utils.RemoveRaftDir()
		dbDirPath, dbFilePath := utils.GetSqliteDbDirAndDbFilePath()

		exists, err := afero.DirExists(fs, dbDirPath)
		if err != nil {
			cmdLogger.Error("Fatal failed to check id sqlite dir exist: %s", err)
		}
		if !exists {
			err = fs.Mkdir(dbDirPath, os.ModePerm)
			if err != nil {
				cmdLogger.Error("Fatal failed to create sqlite dir: %s", err)
			}

			_, err = fs.Create(dbFilePath)
			if err != nil {
				cmdLogger.Error("Fatal db file creation error: %s", err)
			}
		}
		db.RunMigrations(cmdLogger, dbFilePath)

		cmdLogger.Info("Scheduler0 Initialized")
	},
}
