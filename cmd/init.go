package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"os"
)

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Scheduler0",
	Long:  `Initialize configurations are required to start and deploy your Scheduler0`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("-----------------------")
		prompt := promptui.Prompt{
			Label: "Name",
		}
		name, _ := prompt.Run()
		prompt = promptui.Prompt{
			Label: "Email",
		}
		email, _ := prompt.Run()
		selectPrompt := promptui.Select{
			Label: "Cloud Providers",
			Items: []string{"AWS", "Google Cloud", "Azure"},
		}
		_, cloudProvider, _ := selectPrompt.Run()

		config := Config{
			Name:          name,
			Email:         email,
			CloudProvider: cloudProvider,
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
	},
}
