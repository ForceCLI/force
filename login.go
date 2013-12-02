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

  force login
`,
}

var cmdLogin1 = &Command{
	Run:   runLogin1,
	Usage: "login1",
	Short: "Log in to force.com",
	Long: `
Log in to force.com

Examples:

  force login1
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
		case "mobile1":
			endpoint = EndpointMobile1
		default:
			ErrorAndExit("no such endpoint: %s", args[0])
		}
	}
	_, err := ForceLoginAndSave(endpoint)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func runLogin1(cmd *Command, args []string) {
	var endpoint ForceEndpoint
	var un, pw string

	endpoint = EndpointProduction
	if len(args) > 1 {
		switch args[0] {
		case "test":
			endpoint = EndpointTest
		case "pre":
			endpoint = EndpointPrerelease
		case "mobile1":
			endpoint = EndpointMobile1
		default:
			endpoint = EndpointProduction
//			ErrorAndExit("no such endpoint: %s", args[0])
		}
	}
	if len(args) == 3 {
		un = args[1]
		pw = args[2]
		_, err := ForceLoginUsernamePassword(endpoint, un, pw)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	} else {
		_, err := ForceLoginAndSave(endpoint)
		if err != nil {
			ErrorAndExit(err.Error())
		}
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
