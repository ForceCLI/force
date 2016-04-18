package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/heroku/force/salesforce"
)

var cmdLogins = &Command{
	Run:   runLogins,
	Usage: "logins",
	Short: "List force.com logins used",
	Long: `
List force.com accounts

Examples:

  force logins
`,
}

func runLogins(cmd *Command, args []string) {
	active, _ := ActiveLogin()
	accounts, _ := salesforce.Config.List("accounts")
	if len(accounts) == 0 {
		fmt.Println("no logins")
	} else {
		w := new(tabwriter.Writer)
		w.Init(os.Stdout, 1, 0, 1, ' ', 0)

		for _, account := range accounts {
			if !strings.HasPrefix(account, ".") {
				var creds salesforce.ForceCredentials
				data, err := salesforce.Config.Load("accounts", account)
				json.Unmarshal([]byte(data), &creds)

				if err != nil {
					return
				}

				var banner = fmt.Sprintf("\t%s", creds.InstanceUrl)
				if account == active {
					account = fmt.Sprintf("\x1b[31;1m%s (active)\x1b[0m", account)
				} else {
					account = fmt.Sprintf("%s \x1b[31;1m\x1b[0m", account)
				}
				fmt.Fprintln(w, fmt.Sprintf("%s%s", account, banner))
			}
		}
		fmt.Fprintln(w)
		w.Flush()
	}

}

func ActiveLogin() (account string, err error) {
	account, err = salesforce.Config.Load("current", "account")
	if err != nil {
		accounts, _ := salesforce.Config.List("accounts")
		if len(accounts) > 0 {
			SetActiveLoginDefault()
		}
	}
	return
}

func ActiveCredentials() (creds salesforce.ForceCredentials, err error) {
	account, err := ActiveLogin()
	if err != nil {
		return
	}
	data, err := salesforce.Config.Load("accounts", account)
	json.Unmarshal([]byte(data), &creds)

	return
}

func ActiveForce() (force *salesforce.Force, err error) {
	creds, err := ActiveCredentials()
	if err != nil {
		return
	}
	force = salesforce.NewForce(creds)
	return
}

func SetActiveLoginDefault() (account string) {
	accounts, _ := salesforce.Config.List("accounts")
	if len(accounts) > 0 {
		account = accounts[0]
		SetActiveLogin(account)
	}
	return
}

func SetActiveLogin(account string) (err error) {
	err = salesforce.Config.Save("current", "account", account)
	return
}
