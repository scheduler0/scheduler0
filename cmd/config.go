package cmd

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/manifoldco/promptui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"log"
	"os"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/models"
	repository2 "scheduler0/repository"
	"scheduler0/utils"
	"time"
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

func createServerCredential(secretKey string, dbConnection *sql.DB) (string, string) {
	utils.GenerateApiAndSecretKey(secretKey)
	credential := models.CredentialModel{
		Platform: "server",
	}

	apiKey, apiSecret := utils.GenerateApiAndSecretKey(secretKey)

	insertBuilder := sq.Insert(repository2.CredentialTableName).
		Columns(
			repository2.PlatformColumn,
			repository2.ArchivedColumn,
			repository2.ApiKeyColumn,
			repository2.ApiSecretColumn,
			repository2.IPRestrictionColumn,
			repository2.HTTPReferrerRestrictionColumn,
			repository2.IOSBundleIdReferrerRestrictionColumn,
			repository2.AndroidPackageIDReferrerRestrictionColumn,
			repository2.JobsDateCreatedColumn,
		).
		Values(
			credential.Platform,
			credential.Archived,
			apiKey,
			apiSecret,
			credential.IPRestriction,
			credential.HTTPReferrerRestriction,
			credential.IOSBundleIDRestriction,
			credential.AndroidPackageNameRestriction,
			time.Now().String(),
		).
		RunWith(dbConnection)

	_, err := insertBuilder.Exec()
	if err != nil {
		log.Fatalln(err)
	}

	return apiKey, apiSecret
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
		fmt.Println("Initializing Scheduler0")

		config := utils.GetScheduler0Configurations()

		if config.Port == "" {
			portPrompt := promptui.Prompt{
				Label:       "Port",
				HideEntered: true,
				Default:     "9090",
			}
			Port, _ := portPrompt.Run()
			config.Port = Port
		}
		setEnvErr := os.Setenv(utils.PortEnv, config.Port)
		utils.CheckErr(setEnvErr)

		if config.SecretKey == "" {
			secretKeyPrompt := promptui.Prompt{
				Label:       "Secret Key",
				HideEntered: true,
				Default:     "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94",
			}
			SecretKey, _ := secretKeyPrompt.Run()
			config.SecretKey = SecretKey
		}
		err := os.Setenv(utils.SecretKeyEnv, config.SecretKey)
		utils.CheckErr(setEnvErr)

		dir, err := os.Getwd()
		if err != nil {
			log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
		}
		fs := afero.NewOsFs()

		recreateDb(fs, dir)
		recreateRaftDir(fs, dir)

		db := runMigration(fs, dir)
		apiKey, apiSecret := createServerCredential(config.SecretKey, db)

		credentials := utils.Scheduler0Credentials{
			ApiKey:    apiKey,
			ApiSecret: apiSecret,
		}

		configFilePath := fmt.Sprintf("%v/%v", dir, constants.CredentialsFileName)

		configByte, err := json.Marshal(credentials)
		if err != nil {
			log.Fatalln(fmt.Errorf("Fatal error config file: %s \n", err))
		}

		file, err := fs.Create(configFilePath)
		if err != nil {
			log.Fatalln(fmt.Errorf("Fatal cannot create file: %s \n", err))
		}
		_, err = file.Write(configByte)
		if err != nil {
			log.Fatalln(fmt.Errorf("Fatal cannot write to file: %s \n", err))
		}

		fmt.Println("Scheduler0 Initialized")
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
		configs := utils.GetScheduler0Configurations()
		fmt.Println("Configurations:")
		fmt.Println("NodeId:", configs.NodeId)
		fmt.Println("Bootstrap:", configs.Bootstrap)
		fmt.Println("RaftAddress:", configs.RaftAddress)
		fmt.Println("Replicas:", configs.Replicas)
		fmt.Println("Port:", configs.Port)
		fmt.Println("RaftTransportMaxPool:", configs.RaftTransportMaxPool)
		fmt.Println("RaftTransportTimeout:", configs.RaftTransportTimeout)
		fmt.Println("RaftApplyTimeout:", configs.RaftApplyTimeout)
		fmt.Println("RaftSnapshotInterval:", configs.RaftSnapshotInterval)
		fmt.Println("RaftSnapshotThreshold:", configs.RaftSnapshotThreshold)
		fmt.Println("--------------------------")
		fmt.Println("Credentials:")
		credentials := utils.ReadCredentialsFile()
		fmt.Println("ApiSecret:", credentials.ApiSecret)
		fmt.Println("ApiKey:", credentials.ApiKey)
	},
}

func init() {
	ConfigCmd.AddCommand(InitCmd)
	ConfigCmd.AddCommand(ShowCmd)
}
