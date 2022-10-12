package command

import (
	"fmt"

	. "github.com/ForceCLI/force/error"
	"github.com/spf13/cobra"
)

func init() {
	oauthCmd.AddCommand(oauthCreateCmd)

	RootCmd.AddCommand(oauthCmd)
}

var oauthCmd = &cobra.Command{
	Use:   "oauth <command> [<args>]",
	Short: "Manage ConnectedApp credentials",
	Long: `
Manage ConnectedApp credentials

Usage:

  force oauth create <name> <callback>
  `,

	Example: `
  force oauth create MyApp http://localhost:3835/oauth/callback
`,
}

var oauthCreateCmd = &cobra.Command{
	Use:   "create <name> <callback>",
	Short: "Create ConnectedApp",
	Long: `
Create ConnectedApp

Usage:

  force oauth create <name> <callback>
  `,
	Example: `
  force oauth create MyApp http://localhost:3835/oauth/callback
`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		runOauthCreate(args[0], args[1])
	},
}

func runOauthCreate(name, callback string) {
	err := force.Metadata.CreateConnectedApp(name, callback)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Connected App created")
}
