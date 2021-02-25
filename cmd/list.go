package cmd

import (
	"fmt"
	table "github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"os"
	server "scheduler0/server/src"
	"scheduler0/server/src/db"
	"scheduler0/server/src/service"
	"scheduler0/server/src/utils"
)

var entityType = ""


func listCredentials() {
	pool, err := utils.NewPool(server.GetDatabaseConnectionForEnvironment, db.MaxConnections)
	if err != nil {
		utils.Error(err.Error())
		return
	}
	credentialService := service.CredentialService{
		Pool: pool,
	}
	credentialTransformers, listError := credentialService.ListCredentials(0, 10, "date_created DESC")
	if listError != nil {
		utils.Error(listError.Message)
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "UUID", "HTTP Referrer Restriction", "Api Key"})

	for index, credentialTransformer := range credentialTransformers.Data {
		t.AppendSeparator()
		t.AppendRow([]interface{}{index + 1, credentialTransformer.UUID, credentialTransformer.HTTPReferrerRestriction, credentialTransformer.ApiKey})
	}

	t.AppendFooter(table.Row{"", "", "Total", credentialTransformers.Total })
	t.AppendFooter(table.Row{"", "", "Offset", credentialTransformers.Offset })
	t.AppendFooter(table.Row{"", "", "Limit", credentialTransformers.Limit })
	t.Render()
}

var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List credentials, projects, jobs e.t.c",
	Long:  `
Use this to list entities like credentials, projects, jobs and executions. 

Usage: 

> scheduler0 list credentials

This will list all the credentials that you can use in the client sdks
`,
	Run: func(cmd *cobra.Command, args []string) {
		os.Setenv("POSTGRES_ADDRESS", "localhost:5432")
		os.Setenv("POSTGRES_DATABASE", "scheduler0_test")
		os.Setenv("POSTGRES_USER", "core")
		os.Setenv("POSTGRES_PASSWORD", "localdev")

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