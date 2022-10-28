package cmd

import (
	"database/sql"
	_ "embed"
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"log"
	"os"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/utils"
)

//go:embed db.init.sql
var migrations string

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

func runMigration(fs afero.Fs, dir string) *sql.DB {
	dbFilePath := fmt.Sprintf("%v/%v", dir, constants.SqliteDbFileName)

	err := fs.Remove(dbFilePath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal db delete error: %s \n", err))
	}

	_, err = fs.Create(dbFilePath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal db file creation error: %s \n", err))
	}

	datastore := db.NewSqliteDbConnection(dbFilePath)

	conn, openDBConnErr := datastore.OpenConnection()
	if openDBConnErr != nil {
		log.Fatalln(fmt.Errorf("Fatal open db connection: %s \n", openDBConnErr))
	}

	dbConnection := conn.(*sql.DB)

	trx, dbConnErr := dbConnection.Begin()
	if dbConnErr != nil {
		log.Fatalln(fmt.Errorf("Fatal open db transaction error: %s \n", dbConnErr))
	}

	_, execErr := trx.Exec(migrations)
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

	return dbConnection
}

func recreateDb(fs afero.Fs, dir string) {
	dirPath := fmt.Sprintf("%v/%v", dir, constants.SqliteDbFileName)
	exists, err := afero.DirExists(fs, dirPath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error checking dir exist: %s \n", err))
	}
	if exists {
		err := fs.RemoveAll(dirPath)
		if err != nil {
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
		err := fs.RemoveAll(dirPath)
		if err != nil {
			log.Fatalln(fmt.Errorf("Fatal failed to remove raft dir: %s \n", err))
		}
	}
}

// InitCmd initializes scheduler0 configuration
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
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)
		logger.Println("Initializing Scheduler0")

		config := utils.GetScheduler0Configurations(logger)

		if config.Port == "" {
			portPrompt := promptui.Prompt{
				Label:       "Port",
				HideEntered: true,
				Default:     "9090",
			}
			Port, _ := portPrompt.Run()
			config.Port = Port
		}
		setEnvErr := os.Setenv(config.Port, config.Port)
		if setEnvErr != nil {
			logger.Fatalln(setEnvErr)
		}

		if config.SecretKey == "" {
			secretKeyPrompt := promptui.Prompt{
				Label:       "Secret Key",
				HideEntered: true,
			}
			SecretKey, _ := secretKeyPrompt.Run()
			config.SecretKey = SecretKey
		}
		err := os.Setenv(config.SecretKey, config.SecretKey)
		if setEnvErr != nil {
			logger.Fatalln(setEnvErr)
		}
		dir, err := os.Getwd()
		if err != nil {
			log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
		}
		fs := afero.NewOsFs()

		recreateDb(fs, dir)
		recreateRaftDir(fs, dir)

		logger.Println("Scheduler0 Initialized")
	},
}

// ShowCmd show scheduler0 password configuration
var ShowCmd = &cobra.Command{
	Use:   "show",
	Short: "This will show the configurations that have been set.",
	Long: `
Using this protobuffs you can tell what configurations have been set.

Usage:

	scheduler0 config show

Use the --show-password flag if you want the password to be visible.
`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)
		configs := utils.GetScheduler0Configurations(logger)
		logger.Println("Configurations:")
		logger.Println("NodeId:", configs.NodeId)
		logger.Println("Bootstrap:", configs.Bootstrap)
		logger.Println("RaftAddress:", configs.RaftAddress)
		logger.Println("Replicas:", configs.Replicas)
		logger.Println("Port:", configs.Port)
		logger.Println("RaftTransportMaxPool:", configs.RaftTransportMaxPool)
		logger.Println("RaftTransportTimeout:", configs.RaftTransportTimeout)
		logger.Println("RaftApplyTimeout:", configs.RaftApplyTimeout)
		logger.Println("RaftSnapshotInterval:", configs.RaftSnapshotInterval)
		logger.Println("RaftSnapshotThreshold:", configs.RaftSnapshotThreshold)
	},
}

func init() {
	ConfigCmd.AddCommand(InitCmd)
	ConfigCmd.AddCommand(ShowCmd)
}
