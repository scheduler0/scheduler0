package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"os"
	"scheduler0/server/db"
	"scheduler0/utils"
)

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

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize database configurations and port for the scheduler0 server",
	Long: `
Initialize postgres credentials for the Scheduler0 server,
you will be prompted to provide postgres credentials

Usage:

	scheduler0 init

Note that the PORT is optional. By default the server will use :9090
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Initialize Scheduler0")

		postgresAddrPrompt := promptui.Prompt{
			Label:       "Postgres Address",
			AllowEdit:   true,
			HideEntered: true,
		}
		Addr, _ := postgresAddrPrompt.Run()

		postgresUserPrompt := promptui.Prompt{
			Label:       "Postgres PostgresUser",
			AllowEdit:   true,
			HideEntered: true,
		}
		User, _ := postgresUserPrompt.Run()

		postgresDBPrompt := promptui.Prompt{
			Label:       "Postgres PostgresDatabase",
			AllowEdit:   true,
			HideEntered: true,
		}
		DB, _ := postgresDBPrompt.Run()

		postgresPassPrompt := promptui.Prompt{
			Label:       "Postgres PostgresPassword",
			HideEntered: true,
			Mask:        '*',
		}
		Pass, _ := postgresPassPrompt.Run()

		portPrompt := promptui.Prompt{
			Label:       "Port",
			HideEntered: true,
			Default:     "9090",
		}
		Port, _ := portPrompt.Run()

		secretKeyPrompt := promptui.Prompt{
			Label:       "Secret Key",
			HideEntered: true,
			Default:     "9090",
		}
		SecretKey, _ := secretKeyPrompt.Run()

		err := os.Setenv(utils.PostgresAddressEnv, Addr)
		utils.CheckErr(err)

		err = os.Setenv(utils.PostgresUserEnv, User)
		utils.CheckErr(err)

		err = os.Setenv(utils.PostgresPasswordEnv, Pass)
		utils.CheckErr(err)

		err = os.Setenv(utils.PostgresDatabaseEnv, DB)
		utils.CheckErr(err)

		err = os.Setenv(utils.PortEnv, Port)
		utils.CheckErr(err)

		_, err = db.OpenConnection()
		if err != nil {
			utils.Error("Cannot connect to the database")
			utils.Error(err.Error())
			return
		} else {
			config := utils.Scheduler0Configurations{
				PostgresAddress:  Addr,
				PostgresUser:     User,
				PostgresDatabase: DB,
				PostgresPassword: Pass,
				PORT:             Port,
				SecretKey:        SecretKey,
			}

			configByte, err := json.Marshal(config)
			if err != nil {
				panic(fmt.Errorf("Fatal error config file: %s \n", err))
			}
			configFilePath := fmt.Sprintf("%v/.scheduler0", os.Getenv("HOME"))

			fs := afero.NewOsFs()
			file, err := fs.Create(configFilePath)
			if err != nil {
				panic(fmt.Errorf("Fatal unable to save scheduler 0: %s \n", err))
			}

			_, err = file.Write(configByte)
			if err != nil {
				panic(fmt.Errorf("Fatal unable to save scheduler 0: %s \n", err))
			}
		}
	},
}

var showPassword = false

var ShowCmd = &cobra.Command{
	Use:   "show",
	Short: "This will show the configurations that have been set.",
	Long: `
Using this command you can tell what configurations have been  excluding the database password.

Usage:

	scheduler0 config show

Use the --show-password flag if you want the password to be visible.
`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.SetScheduler0Configurations()
		configs := utils.GetScheduler0Configurations()

		if showPassword {
			utils.Info(fmt.Sprintf(
				"PORT = %v "+
					"POSTGRES_ADDRESS = %v "+
					"POSTGRES_USER = %v "+
					"POSTGRES_PASSWORD = %v "+
					"POSTGRES_DATABASE = %v ",
				configs.PORT,
				configs.PostgresAddress,
				configs.PostgresUser,
				configs.PostgresPassword,
				configs.PostgresDatabase))
		} else {
			utils.Info(fmt.Sprintf(
				"PORT = %v "+
					"POSTGRES_ADDRESS = %v "+
					"POSTGRES_USER = %v "+
					"POSTGRES_DATABASE = %v ",
				configs.PORT,
				configs.PostgresAddress,
				configs.PostgresUser,
				configs.PostgresDatabase))
		}

	},
}

func init() {
	ShowCmd.Flags().BoolVarP(&showPassword, "show-password", "s", false, "scheduler0 config show --show-password")

	ConfigCmd.AddCommand(InitCmd)
	ConfigCmd.AddCommand(ShowCmd)
}
