package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/constants/headers"
	"scheduler0/pkg/models"
	"scheduler0/pkg/secrets"
	"strings"

	"github.com/spf13/cobra"
)

var accountId uint64

var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create a resource like credential, projects or jobs",
	Long: `
Use this 

Usage:
	create credential --account-id 1
`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if accountId == 0 {
			log.Fatal("--account-id is required")
		}
	},
}

var credentialCmd = &cobra.Command{
	Use:   "credential",
	Short: "Creates a new credential",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stdout, "[cmd] ", log.LstdFlags)

		configs := config.NewScheduler0Config().GetConfigurations()
		secrets := secrets.NewScheduler0Secrets().GetSecrets()

		if secrets == nil {
			logger.Println("Scheduler0 secrets have not been set. Run ./scheduler0 config init to setup your secrets.")
			return
		}

		credentialModel := models.Credential{
			CreatedBy: secrets.AuthUsername,
		}
		data, err := credentialModel.ToJSON()
		if err != nil {
			logger.Fatalln(err)
		}

		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				req.Method = http.MethodPost
				body := bytes.NewReader(data)
				rc := ioutil.NopCloser(body)
				req.Body = rc
				req.SetBasicAuth(secrets.AuthUsername, secrets.AuthPassword)
				req.Header.Add(headers.PeerHeader, headers.PeerHeaderCMDValue)
				req.Header.Add(headers.AccountIDHeader, fmt.Sprintf("%d", accountId))
				req.Header.Add("Content-Type", "application/json")

				if len(via) > 5 {
					logger.Fatalln("too many redirects")
				}

				return nil
			},
		}

		// Determine the base URL - use base URL from secrets if available, otherwise fall back to config
		var requestURL string
		if secrets.BaseURL != "" && strings.TrimSpace(secrets.BaseURL) != "" {
			// Use base URL as-is
			baseURL := strings.TrimRight(secrets.BaseURL, "/")
			requestURL = fmt.Sprintf("%s/%s/credentials", baseURL, constants.APIV1Base)
		} else {
			// Fall back to config-based URL construction
			protocol := "http"
			if configs.HTTPCert != "" {
				protocol = "https"
			}
			baseURL := fmt.Sprintf("%s://%s:%s", protocol, configs.Host, configs.ClientPort)
			requestURL = fmt.Sprintf("%s/%s/credentials", baseURL, constants.APIV1Base)
		}

		req, err := http.NewRequest(
			"POST",
			requestURL,
			bytes.NewReader(data),
		)
		if err != nil {
			logger.Fatalln(err)
		}

		req.SetBasicAuth(secrets.AuthUsername, secrets.AuthPassword)
		req.Header.Add(headers.PeerHeader, headers.PeerHeaderCMDValue)
		req.Header.Add(headers.AccountIDHeader, fmt.Sprintf("%d", accountId))
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
			fmt.Println(string(body))
		}
	},
}

func init() {
	CreateCmd.PersistentFlags().Uint64Var(&accountId, "account-id", 0, "Account ID for the resource")
	CreateCmd.AddCommand(credentialCmd)
}
