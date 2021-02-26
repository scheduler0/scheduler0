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

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize postgres credentials for the Scheduler0 server",
	Long:  `Initialize postgres credentials for the Scheduler0 server,
you will be prompted to provide postgres credentials

Usage:

> scheduler0 init
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Initialize Scheduler0")

		postgresAddrPrompt := promptui.Prompt{
			Label: "Postgres Address",
			AllowEdit: true,
			HideEntered: true,
		}
		Addr, _ := postgresAddrPrompt.Run()

		postgresUserPrompt := promptui.Prompt{
			Label: "Postgres User",
			AllowEdit: true,
			HideEntered: true,
		}
		User, _ := postgresUserPrompt.Run()

		postgresDBPrompt := promptui.Prompt{
			Label: "Postgres Database",
			AllowEdit: true,
			HideEntered: true,
		}
		DB, _ := postgresDBPrompt.Run()

		postgresPassPrompt := promptui.Prompt{
			Label: "Postgres Password",
			HideEntered: true,
			Mask: '*',
		}
		Pass, _ := postgresPassPrompt.Run()

		err := os.Setenv("POSTGRES_ADDRESS", Addr)
		utils.CheckErr(err)

		err = os.Setenv("POSTGRES_USER", User)
		utils.CheckErr(err)

		err = os.Setenv("POSTGRES_PASSWORD", Pass)
		utils.CheckErr(err)

		err = os.Setenv("POSTGRES_DATABASE", DB)
		utils.CheckErr(err)

		_, err = db.OpenConnection()
		if err != nil {
			utils.Error("Cannot connect to the database")
			utils.Error(err.Error())
			return
		} else {
			config := utils.PostgresCredentials{
				Addr: Addr,
				User: User,
				Database: DB,
				Password: Pass,
			}

			configByte, err := json.Marshal(config)
			if err != nil {
				panic(fmt.Errorf("Fatal error config file: %s \n", err))
			}
			fs := afero.NewOsFs()
			file, err := fs.Create(fmt.Sprintf("%v/.scheduler0", os.Getenv("HOME")))
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
