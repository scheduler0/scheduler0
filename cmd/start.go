package cmd

import (
	"fmt"
	"log"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/db"
	http_server "scheduler0/pkg/http/server"
	"scheduler0/pkg/utils"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start scheduler0 http server",
	Long: `
This start command will spin up the http server. 
The server will be ready to receive request on the Port specified during init otherwise use :9090

Usage: 

> scheduler0 start

The server needs to be running in order to execute jobs.
`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)
		logger.Println("Starting Server.")
		config := config.NewScheduler0Config().GetConfigurations()
		cmdLogger := hclog.New(&hclog.LoggerOptions{
			Name:  "scheduler0-cmd",
			Level: hclog.LevelFromString(config.LogLevel),
		})
		dbDirPath, dbFilePath := utils.GetSqliteDbDirAndDbFilePath()
		fs := afero.NewOsFs()

		exists, err := afero.DirExists(fs, dbDirPath)
		if err != nil {
			log.Fatalln(fmt.Errorf("Fatal failed to check id sqlite dir exist: %s", err))
		}
		if !exists {
			err = fs.Mkdir(dbDirPath, os.ModePerm)
			if err != nil {
				log.Fatalln(fmt.Errorf("Fatal failed to create sqlite dir: %s", err))
			}

			_, err = fs.Create(dbFilePath)
			if err != nil {
				log.Fatalln(fmt.Errorf("Fatal db file creation error: %s", err))
			}
		}

		db.RunMigrations(cmdLogger, dbFilePath)
		http_server.Start()
	},
}
