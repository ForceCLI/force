package main

import (
	"encoding/json"
)

var cmdLogin = &Command{
	Run:   runLogin,
	Usage: "login",
	Short: "Log in to force.com",
	Long: `
Log in to force.com

Examples:

  force login       # log in to production or developer org

  force login test  # log in to sandbox org

  force login pre   # log in to prerelease org
  
`,
}

func runLogin(cmd *Command, args []string) {
	var endpoint ForceEndpoint
	endpoint = EndpointProduction
	if len(args) > 0 {
		switch args[0] {
		case "test":
			endpoint = EndpointTest
		case "pre":
			endpoint = EndpointPrerelease
		case "dev":
			endpoint = EndpointDev
		default:
			ErrorAndExit("no such endpoint: %s", args[0])
		}
	}
	_, err := ForceLoginAndSave(endpoint)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

var cmdLogout = &Command{
	Run:   runLogout,
	Usage: "logout <account>",
	Short: "Log out from force.com",
	Long: `
Log out from force.com

Examples:

  force logout user@example.org
`,
}

func runLogout(cmd *Command, args []string) {
	if len(args) != 1 {
		ErrorAndExit("must specify account to log out")
	}
	account := args[0]
	Config.Delete("accounts", account)
	if active, _ := Config.Load("current", "account"); active == account {
		Config.Delete("current", "account")
		SetActiveAccountDefault()
	}
}

func ForceLoginAndSave(endpoint ForceEndpoint) (username string, err error) {
	creds, err := ForceLogin(endpoint)
	if err != nil {
		return
	}
	force := NewForce(creds)
	login, err := force.Get(creds.Id)
	if err != nil {
		return
	}
	body, err := json.Marshal(creds)
	if err != nil {
		return
	}
	username = login["username"].(string)
	Config.Save("accounts", username, string(body))
	Config.Save("current", "account", username)
	return
}
