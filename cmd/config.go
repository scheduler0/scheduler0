package cmd

import (
	"database/sql"
	_ "embed"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"log"
	"os"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/db"
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

func runMigration(cmdLogger hclog.Logger, fs afero.Fs, dir string) {
	dbDirPath := fmt.Sprintf("%s/%s", dir, constants.SqliteDir)
	dbFilePath := fmt.Sprintf("%s/%s/%s", dir, constants.SqliteDir, constants.SqliteDbFileName)

	err := fs.Remove(dbFilePath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalln(fmt.Errorf("Fatal db delete error: %s \n", err))
	}

	err = fs.Mkdir(dbDirPath, os.ModePerm)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal db file creation error: %s \n", err))
	}

	_, err = fs.Create(dbFilePath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal db file creation error: %s \n", err))
	}

	datastore := db.NewSqliteDbConnection(cmdLogger, dbFilePath)
	conn := datastore.OpenConnection()

	dbConnection := conn.(*sql.DB)

	trx, dbConnErr := dbConnection.Begin()
	if dbConnErr != nil {
		log.Fatalln(fmt.Errorf("Fatal open db transaction error: %s \n", dbConnErr))
	}

	_, execErr := trx.Exec(db.GetSetupSQL())
	if execErr != nil {
		errRollback := trx.Rollback()
		if errRollback != nil {
			log.Fatalln(fmt.Errorf("Fatal rollback error: %s \n", execErr))
		}
		log.Fatalln(fmt.Errorf("Fatal open db transaction error: %s \n", execErr))
	}

	errCommit := trx.Commit()
	if errCommit != nil {
		log.Fatalln(fmt.Errorf("Fatal commit error: %s \n", errCommit))
	}
}

func recreateDb(fs afero.Fs, dir string) {
	dirPath := fmt.Sprintf("%s/%s/%s", dir, constants.SqliteDir)
	filePath := fmt.Sprintf("%s/%s/%s", dir, constants.SqliteDir, constants.SqliteDbFileName)
	exists, err := afero.DirExists(fs, dirPath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error checking dir exist: %s \n", err))
	}
	if exists {
		removeErr := fs.RemoveAll(dirPath)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			log.Fatalln(fmt.Errorf("Fatal failed to remove raft dir: %s \n", err))
		}
		removeErr = fs.RemoveAll(filePath)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			log.Fatalln(fmt.Errorf("Fatal failed to remove raft dir: %s \n", err))
		}
	}
}

func recreateRaftDir(fs afero.Fs, dir string) {
	dirPath := fmt.Sprintf("%v/%v", dir, constants.RaftDir)
	exists, err := afero.DirExists(fs, dirPath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error checking dir exist: %s \n", err))
	}
	if exists {
		removeErr := fs.RemoveAll(dirPath)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			log.Fatalln(fmt.Errorf("Fatal failed to remove raft dir: %s \n", err))
		}
	}
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

		dir, err := os.Getwd()
		if err != nil {
			log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
		}
		fs := afero.NewOsFs()

		recreateDb(fs, dir)
		recreateRaftDir(fs, dir)
		runMigration(cmdLogger, fs, dir)

		logger.Println("Scheduler0 Initialized")
	},
}

func init() {
	ConfigCmd.AddCommand(initCmd)
}
