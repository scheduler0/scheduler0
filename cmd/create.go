package cmd

import (
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"scheduler0/http_server/middlewares/auth"
	"scheduler0/models"
	"scheduler0/utils"
	"strings"
)

var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create a resource like credential, projects or jobs",
	Long: `
Use this 

Usage:
	create credential
`,
}

var credentialCmd = &cobra.Command{
	Use:   "credential",
	Short: "Creates a new credential",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)

		configs := utils.GetScheduler0Configurations(logger)
		credentials := utils.GetScheduler0Credentials(logger)

		credentialModel := models.CredentialModel{}
		typePrompt := promptui.Select{
			Label: "Platform",
			Items: []string{"server", "web", "ios", "android"},
		}
		_, credentialType, err := typePrompt.Run()
		if err != nil {
			logger.Fatalln(err)
		}
		credentialModel.Platform = credentialType

		if credentialType != "server" {
			// TODO:  handle none server credentials
		} else {
			apiKey, apiSecret := utils.GenerateApiAndSecretKey(credentials.SecretKey)
			credentialModel.ApiKey = apiKey
			credentialModel.ApiSecret = apiSecret
		}

		data, err := credentialModel.ToJSON()
		if err != nil {
			logger.Fatalln(err)
		}

		client := &http.Client{}
		req, err := http.NewRequest(
			"POST",
			fmt.Sprintf("%s://%s:%s/credentials", configs.Protocol, configs.Host, configs.Port),
			strings.NewReader(string(data)),
		)

		req.SetBasicAuth(credentials.AuthUsername, credentials.AuthPassword)
		req.Header.Add(auth.PeerHeader, "cmd")
		req.Header.Add("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			logger.Fatalln(err)
		}
		defer res.Body.Close()

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			logger.Fatalln(err)
		}

		if res.StatusCode != http.StatusCreated {
			logger.Fatalln("failed to create new credential:error:", string(body))
		} else {
			logger.Println("successfully created a credential")
			err := credentialModel.FromJSON(body)
			if err != nil {
				logger.Fatalln(err)
			}
			logger.Println(credentialModel)
		}
	},
}

func init() {
	CreateCmd.AddCommand(credentialCmd)
}
