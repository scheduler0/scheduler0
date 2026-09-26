package cmd

import (
	_ "embed"
	"log"
	"os"
	"scheduler0-private/pkg/secrets"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var SecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "create, view or modify scheduler0 secrets",
	Long:  ``,
}

// InitCmd initializes scheduler0 configuration
var initSecretsCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize secrets for the scheduler0 server",
	Long: `
`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)
		logger.Println("Initializing Scheduler0 Secrets")

		scheduler0Secrets := secrets.NewScheduler0Secrets()
		secrets := scheduler0Secrets.GetSecrets()

		if secrets.SecretKey != "" && secrets.AuthUsername != "" && secrets.AuthPassword != "" {
			recreateKey := promptui.Prompt{
				Label:       "Secrets already exist are you sure you want to re-create it[Y/n]:",
				HideEntered: false,
			}
			recreate, _ := recreateKey.Run()

			if strings.ToLower(recreate) == "n" || strings.ToLower(recreate) == "no" {
				return
			}
		}

		secretKeyPrompt := promptui.Prompt{
			Label:       "Secret Key",
			Mask:        '*',
			HideEntered: true,
		}
		SecretKey, _ := secretKeyPrompt.Run()
		secrets.SecretKey = SecretKey

		authUserNamePrompt := promptui.Prompt{
			Label:       "Auth Username",
			HideEntered: true,
		}
		usernameKey, _ := authUserNamePrompt.Run()
		secrets.AuthUsername = usernameKey

		passwordKeyPrompt := promptui.Prompt{
			Label:       "Auth Password",
			Mask:        '*',
			HideEntered: true,
		}
		passwordKey, _ := passwordKeyPrompt.Run()
		secrets.AuthPassword = passwordKey

		baseURLPrompt := promptui.Prompt{
			Label:       "Base URL (optional, e.g., http://leader.example.com:8080)",
			HideEntered: false,
			Default:     "",
		}
		baseURL, _ := baseURLPrompt.Run()
		secrets.BaseURL = strings.TrimSpace(baseURL)

		scheduler0Secrets.SaveSecrets(secrets)
		logger.Println("Scheduler0 Initialized")
	},
}

var showPasswordFlag bool

// ShowCmd show scheduler0 password configuration
var showSecretsCmd = &cobra.Command{
	Use:   "show",
	Short: "This will show the configurations that have been set.",
	Long: `
Using this secrets you can tell what secrets have been set.

Usage:

	scheduler0 secrets show

Use the --show-password flag if you want the password to be visible.
`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)
		scheduler0Secrets := secrets.NewScheduler0Secrets()
		secrets := scheduler0Secrets.GetSecrets()
		logger.Println("Secrets:")
		if showPasswordFlag {
			logger.Println("SecretKey:", secrets.SecretKey)
		} else {
			logger.Println("SecretKey: ********")
		}
		logger.Println("AuthUsername:", secrets.AuthUsername)
		if showPasswordFlag {
			logger.Println("AuthPassword:", secrets.AuthPassword)
		} else {
			logger.Println("AuthPassword: ********")
		}
		if secrets.BaseURL != "" {
			logger.Println("BaseURL:", secrets.BaseURL)
		}
	},
}

func init() {
	showSecretsCmd.Flags().BoolVar(&showPasswordFlag, "show-password", false, "Show password and secret key in plaintext")
	SecretsCmd.AddCommand(initSecretsCmd)
	SecretsCmd.AddCommand(showSecretsCmd)
}
