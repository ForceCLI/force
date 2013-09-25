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

func ForceLoginAndSave() (username string, err error) {
	creds, err := ForceLogin()
	if err != nil {
		return
	}
	force := NewForce(creds)
	user, err := force.Get("User", creds.Id)
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
