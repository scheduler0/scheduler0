package cmd

import (
	"bytes"
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"scheduler0/config"
	"scheduler0/http_server/middlewares/auth"
	"scheduler0/models"
	"scheduler0/utils"
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

		configs := config.GetScheduler0Configurations(logger)
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

		if credentialType == models.WebPlatform {
			httpReferrerPrompt := promptui.Prompt{
				Label: "HTTP Referrer Restriction",
			}
			httpReferrerRestriction, err := httpReferrerPrompt.Run()
			if err != nil {
				logger.Fatalln(err)
			}
			credentialModel.HTTPReferrerRestriction = httpReferrerRestriction
			ipRestrictionPrompt := promptui.Prompt{
				Label: "IP Restriction",
			}
			ipRestriction, err := ipRestrictionPrompt.Run()
			if err != nil {
				logger.Fatalln(err)
			}
			credentialModel.HTTPReferrerRestriction = httpReferrerRestriction
			credentialModel.IPRestriction = ipRestriction
		}

		if credentialType == models.IOSPlatform {
			iOSBundlePrompt := promptui.Prompt{
				Label: "iOS Bundle Id Restriction",
			}
			iOSBundleRestriction, err := iOSBundlePrompt.Run()
			if err != nil {
				logger.Fatalln(err)
			}
			credentialModel.IOSBundleIDRestriction = iOSBundleRestriction
		}

		if credentialType == models.AndroidPlatform {
			androidPackageIdPrompt := promptui.Prompt{
				Label: "Android Package Restriction",
			}
			androidPackageIdRestriction, err := androidPackageIdPrompt.Run()
			if err != nil {
				logger.Fatalln(err)
			}
			credentialModel.AndroidPackageNameRestriction = androidPackageIdRestriction
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
				req.SetBasicAuth(credentials.AuthUsername, credentials.AuthPassword)
				req.Header.Add(auth.PeerHeader, "cmd")
				req.Header.Add("Content-Type", "application/json")

				if len(via) > 5 {
					logger.Fatalln("Too many redirects")
				}

				return nil
			},
		}
		req, err := http.NewRequest(
			"POST",
			fmt.Sprintf("%s://%s:%s/credentials", configs.Protocol, configs.Host, configs.Port),
			bytes.NewReader(data),
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
			logger.Println("successfully created a credential ", string(body))
		}
	},
}

func init() {
	CreateCmd.AddCommand(credentialCmd)
}
