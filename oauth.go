package main

import (
	"fmt"
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
			runOauthCreate(args[1:])
		default:
			ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func runOauthCreate(args []string) {
	if len(args) != 2 {
		ErrorAndExit("must specify name and callback")
	}
	force, _ := ActiveForce()
	err := force.Metadata.CreateConnectedApp(args[0], args[1])
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("OAuth credentials created")
}
