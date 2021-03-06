package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"scheduler0/server/db"
	"scheduler0/server/service"
	"scheduler0/utils"
)

var entityType = ""

func listCredentials() {
	pool, err := utils.NewPool(db.OpenConnection, 1)
	if err != nil {
		utils.Error(err.Error())
		return
	}
	credentialService := service.Credential{
		Pool: pool,
	}
	credentialTransformers, listError := credentialService.ListCredentials(0, 10, "date_created DESC")
	if listError != nil {
		utils.Error(listError.Message)
		return
	}
	for _, credentialTransformer := range credentialTransformers.Data {

		utils.Info(fmt.Sprintf(
`
-----------------------------------------
UUID = %v
Platform = %v
API Key = %v
API Secret = %v
HTTP Referrer Restriction = %v
IP Restriction = %v
iOS Bundle Restriction = %v
Android Package Name Restriction = %v
`,
			credentialTransformer.UUID,
			credentialTransformer.Platform,
			credentialTransformer.ApiKey,
			credentialTransformer.ApiSecret,
			credentialTransformer.HTTPReferrerRestriction,
			credentialTransformer.IPRestrictionRestriction,
			credentialTransformer.IOSBundleIDRestriction,
			credentialTransformer.AndroidPackageNameRestriction))
	}
}

var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List credentials, projects, jobs e.t.c",
	Long: `
Use this to list entities like credentials, projects, jobs and executions. 

Usage: 

> scheduler0 list credentials

This will list all the credentials that you can use in the client sdks
`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.SetScheduler0Configurations()
		switch entityType {
		case "credentials":
			listCredentials()
		default:
			utils.Error(fmt.Sprintf("%v is not a valid table", entityType))
		}
	},
}

func init() {
	ListCmd.Flags().StringVarP(&entityType, "table", "t", "", "entity type to list")
}
