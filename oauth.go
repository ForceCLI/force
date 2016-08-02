package main

import (
	"fmt"

	"github.com/heroku/force/util"
)

var cmdOauth = &Command{
	Run:   runOauth,
	Usage: "oauth <command> [<args>]",
	Short: "Manage ConnectedApp credentials",
	Long: `
Manage ConnectedApp credentials

Usage:

  force oauth create <name> <callback>

Examples:

  force oauth create MyApp https://myapp.herokuapp.com/auth/callback
`,
}

func runOauth(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
	} else {
		switch args[0] {
		case "create", "add":
			runOauthCreate(cmd, args[1:])
		default:
			util.ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func runOauthCreate(cmd *Command, args []string) {
	if len(args) != 2 {
		util.ErrorAndExit("must specify name and callback")
	}
	force, _ := ActiveForce()
	err := force.Metadata.CreateConnectedApp(args[0], args[1])
	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	apps, err := force.Metadata.ListConnectedApps()
	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	runFetch(cmd, []string{"ConnectedApp", args[0]})
	for _, app := range apps {
		if app.Name == args[0] {
			//url := fmt.Sprintf("%s/%s", force.Credentials.InstanceUrl, app.Id)
			//Open(url)
		}
	}
	fmt.Println("OAuth credentials created")
}
