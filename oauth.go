package main

import (
	"fmt"
	"sort"
)

var cmdOauth = &Command{
	Run:   runOauth,
	Usage: "oauth <command> [<args>]",
	Short: "Manage ConnectedApp credentials",
	Long: `
Manage ConnectedApp credentials

Usage:

  force oauth create <name> <callback>

  force oauth list

  force oauth get <name>

  force oauth delete <name>

Examples:

  force oauth create MyApp https://myapp.herokuapp.com/auth/callback

  force oauth list

  force oauth get MyApp

  force oauth delete MyApp
`,
}

func runOauth(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
	} else {
		switch args[0] {
		case "create":
			runOauthCreate(args[1:])
		case "get":
			runOauthGet(args[1:])
		case "list":
			runOauthList(args[1:])
		case "delete":
			runOauthDelete(args[1:])
		default:
			ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func runOauthGet(args []string) {
	if len(args) != 1 {
		ErrorAndExit("must specify name")
	}
	force, _ := ActiveForce()
	app, err := force.Metadata.GetConnectedApp(args[0])
	fmt.Println("app", app, "err", err)
}

func runOauthCreate(args []string) {
	if len(args) != 2 {
		ErrorAndExit("must specify name and callback")
	}
	force, _ := ActiveForce()
	app, err := force.Metadata.CreateConnectedApp(args[0], args[1])
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("app", app)
}

func runOauthList(args []string) {
	force, _ := ActiveForce()
	apps, err := force.Metadata.ListConnectedApps()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if len(apps) == 0 {
		fmt.Println("No OAuth Credentials")
	} else {
		sort.Sort(apps)
		for _, app := range apps {
			fmt.Println(app.Name)
		}
	}
}

func runOauthDelete(args []string) {
	/* if len(args) != 2 {*/
	/*   ErrorAndExit("must specify object and id")*/
	/* }*/
	/* force, _ := ActiveForce()*/
	/* err := force.DeleteOauth(args[0], args[1])*/
	/* if err != nil {*/
	/*   ErrorAndExit(err.Error())*/
	/* }*/
	/* fmt.Println("Oauth deleted")*/
}
