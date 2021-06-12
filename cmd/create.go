package cmd

import (
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"scheduler0/server/db"
	"scheduler0/server/service"
	"scheduler0/server/transformers"
	"scheduler0/utils"
	"strings"
)

// CreateCmd create job, credentials and projects command
var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create a credential or project",
	Long: `
Use this command to create credentials for any client or a project. 

Usage:
	scheduler0 create credentials --client server

This will create an api key and api secret for a server side backend.
For other clients such as Android, iOS and Web an restriction key is required.
`,
}

var clientFlag = ""

// CredentialCmd create credential command
var CredentialCmd = &cobra.Command{
	Use:   "credential",
	Short: "create credential",
	Long: `
Use this command to create a new credential for any client.

Usage:
	scheduler0 create credential --client server

The client flag has to be one of android, ios, web, server.
For android, ios, web credential at least one restriction will be required.
`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.SetScheduler0Configurations()

		validClients := strings.Join([]string{"android", "ios", "web", "server"}, ",")
		isValidClientFlag := strings.Contains(validClients, clientFlag)

		if !isValidClientFlag {
			utils.Error(fmt.Sprintf("%v is not a valid client", clientFlag))
			return
		}
		pool, err := utils.NewPool(db.OpenConnection, 1)
		if err != nil {
			utils.Error(err.Error())
			return
		}
		credentialService := service.Credential{Pool: pool}
		credentialTransformer := transformers.Credential{
			Platform: clientFlag,
		}

		if clientFlag == "ios" {
			iOSBundleRestrictionPrompt := promptui.Prompt{
				Label:       "iOS Bundle Restriction",
				AllowEdit:   true,
				HideEntered: true,
			}
			iOSBundleRestriction, _ := iOSBundleRestrictionPrompt.Run()

			if len(iOSBundleRestriction) < 1 {
				utils.Error("please enter a valid ios bundle id")
			} else {
				credentialTransformer.IOSBundleIDRestriction = iOSBundleRestriction
			}
		} else if clientFlag == "android" {
			androidRestrictionPrompt := promptui.Prompt{
				Label:       "Android Package Name Restriction",
				AllowEdit:   true,
				HideEntered: true,
			}
			restriction, _ := androidRestrictionPrompt.Run()

			if len(restriction) < 1 {
				utils.Error("please enter a valid android package name")
			} else {
				credentialTransformer.AndroidPackageNameRestriction = restriction
			}
		} else if clientFlag == "web" {
			webRestrictionPrompt := promptui.Prompt{
				Label:       "Web http referral restriction",
				AllowEdit:   true,
				HideEntered: true,
			}
			httpRestriction, _ := webRestrictionPrompt.Run()

			ipRestrictionPrompt := promptui.Prompt{
				Label:       "Web http referral restriction",
				AllowEdit:   true,
				HideEntered: true,
			}
			ipRestriction, _ := ipRestrictionPrompt.Run()

			if len(ipRestriction) < 1 || len(httpRestriction) < 1 {
				utils.Error("please enter a valid bundle description")
			} else {
				credentialTransformer.HTTPReferrerRestriction = httpRestriction
				credentialTransformer.IPRestrictionRestriction = ipRestriction
			}
		}
		credentialUUID, creteCredentialError := credentialService.CreateNewCredential(credentialTransformer)
		if creteCredentialError != nil {
			utils.Error(creteCredentialError.Message)
		} else {
			utils.Info(fmt.Sprintf("Successfully created a new credential with ID = %v", credentialUUID))
		}
	},
}

func init() {
	CredentialCmd.Flags().StringVarP(&clientFlag, "client", "c", "", "scheduler0 create credential --client server")
	CreateCmd.AddCommand(CredentialCmd)
}
