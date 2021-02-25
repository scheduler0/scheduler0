package cmd

import "github.com/spf13/cobra"


var KeyFlag = ""

var SetEnvCmd = &cobra.Command{
	Use:   "set",
	Short: "Set value of any configuration",
	Long:  `
Change your default environment from local to staging or production.
	
Usage:
> scheduler0 set --key env

For production and staging environment make sure to use the setup command 
to configure your database and deployment strategies.
`,
	Run: func(cmd *cobra.Command, args []string) {

		switch KeyFlag {
		case "env":
		case "name":
		case "cp" :
		case "email":
		default:

		}
		
	},
}


func init()  {
	SetEnvCmd.AddCommand(&cobra.Command{
		Use:   "env",
		Short: "Set value of any configuration",
	})

	SetEnvCmd.Flags().StringVarP(&KeyFlag, "key", "key", "", "change environment to either local, staging, and production")
}