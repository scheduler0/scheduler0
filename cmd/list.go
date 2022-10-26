package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"scheduler0/utils"
)

var entityType = ""

// TODO: Re-implement to use http-agent
func listCredentials() {
	//	conn, err := db.OpenConnection()
	//	dbConnection := conn.(*sql.DB)
	//	utils.CheckErr(err)
	//	peer := store2.NewStore(dbConnection, nil)
	//	credentialRepo := repository.NewCredentialRepo(&peer)
	//	ctx := context.Background()
	//	credentialService := service.NewCredentialService(credentialRepo, ctx)
	//	credentialTransformers, listError := credentialService.ListCredentials(0, 10, "date_created DESC")
	//	if listError != nil {
	//		utils.Error(listError.Message)
	//		return
	//	}
	//	for _, credentialTransformer := range credentialTransformers.Data {
	//
	//		utils.Info(fmt.Sprintf(
	//			`
	//-----------------------------------------
	//UUID = %v
	//Platform = %v
	//API Key = %v
	//API Secret = %v
	//HTTP Referrer Restriction = %v
	//IP Restriction = %v
	//iOS Bundle Restriction = %v
	//Android Package Name Restriction = %v
	//`,
	//			credentialTransformer.ID,
	//			credentialTransformer.Platform,
	//			credentialTransformer.ApiKey,
	//			credentialTransformer.ApiSecret,
	//			credentialTransformer.HTTPReferrerRestriction,
	//			credentialTransformer.IPRestriction,
	//			credentialTransformer.IOSBundleIDRestriction,
	//			credentialTransformer.AndroidPackageNameRestriction))
	//	}
}

// ListCmd returns list of scheduler0 credentials
var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "ListByJobID credentials, projects, jobs e.t.c",
	Long: `
Use this to list entities like credentials, projects, jobs and executions. 

Usage: 

> scheduler0 list credentials

This will list all the credentials that you can use in the client sdks
`,
	Run: func(cmd *cobra.Command, args []string) {
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
