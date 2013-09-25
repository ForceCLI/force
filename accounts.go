package main

import (
	"encoding/json"
	"fmt"
)

var cmdAccounts = &Command{
	Run:   runAccounts,
	Usage: "accounts",
	Short: "List force.com accounts",
	Long: `
List force.com accounts

Examples:

  force accounts
`,
}

func runAccounts(cmd *Command, args []string) {
	active, _ := ActiveAccount()
	accounts, _ := Config.List("accounts")
	if len(accounts) == 0 {
		fmt.Println("no accounts")
	} else {
		for _, account := range accounts {
			var banner string
			if account == active {
				banner = " (active)"
			}
			fmt.Printf("%s%s\n", account, banner)
		}
	}
}

func ActiveAccount() (account string, err error) {
	account, err = Config.Load("current", "account")
	if err != nil {
		accounts, _ := Config.List("accounts")
		if len(accounts) > 0 {
			SetActiveAccountDefault()
		} else {
			account, err = ForceLoginAndSave()
			SetActiveAccount(account)
		}
	}
	return
}

func ActiveCredentials() (creds ForceCredentials, err error) {
	account, err := ActiveAccount()
	if err != nil {
		return
	}
	data, err := Config.Load("accounts", account)
	json.Unmarshal([]byte(data), &creds)
	return
}

func ActiveForce() (force *Force, err error) {
	creds, err := ActiveCredentials()
	if err != nil {
		return
	}
	force = NewForce(creds)
	return
}

func SetActiveAccountDefault() (account string) {
	accounts, _ := Config.List("accounts")
	if len(accounts) > 0 {
		account = accounts[0]
		SetActiveAccount(account)
	}
	return
}

func SetActiveAccount(account string) (err error) {
	err = Config.Save("current", "account", account)
	return
}
