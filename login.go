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

func runLogin(cmd *Command, args []string) {
	ForceLoginAndSave()
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

func ForceLoginAndSave() (username string, err error) {
	creds, err := ForceLogin()
	if err != nil {
		return
	}
	force := NewForce(creds)
	user, err := force.GetRecord("User", creds.Id)
	if err != nil {
		return
	}
	body, err := json.Marshal(creds)
	if err != nil {
		return
	}
	username = user["Username"].(string)
	Config.Save("accounts", username, string(body))
	Config.Save("current", "account", username)
	return
}
