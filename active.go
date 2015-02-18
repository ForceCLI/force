package main

import (
	"encoding/json"
	"fmt"
	"sort"
)

var cmdActive = &Command{
	Usage: "active [account]",
	Short: "Show or set the active force.com account",
	Long: `
Set the active force.com account

Examples:

  force active
  force active user@example.org
`,
}
var (
	tojson  bool
	account string
)

func init() {
	cmdActive.Flag.BoolVar(&tojson, "j", false, "output to json")
	cmdActive.Flag.BoolVar(&tojson, "json", false, "output to json")
	cmdActive.Flag.StringVar(&account, "a", "", "output to json")
	cmdActive.Flag.StringVar(&account, "account", "", "output to json")
	cmdActive.Run = runActive
}

func runActive(cmd *Command, args []string) {
	if account == "" {
		account, _ := Config.Load("current", "account")
		data, _ := Config.Load("accounts", account)
		var creds ForceCredentials
		json.Unmarshal([]byte(data), &creds)
		if tojson {
			fmt.Printf(fmt.Sprintf("{ \"login\": \"%s\", \"instanceUrl\": \"%s\", \"namespace\":\"%s\" }", account, creds.InstanceUrl, creds.Namespace))
		} else {
			fmt.Println(fmt.Sprintf("%s - %s - ns:%s", account, creds.InstanceUrl, creds.Namespace))
		}
	} else {
		//account := args[0]
		accounts, _ := Config.List("accounts")
		i := sort.SearchStrings(accounts, account)
		if i < len(accounts) && accounts[i] == account {
			Config.Save("current", "account", account)
		} else {
			ErrorAndExit(fmt.Sprintf("no such account %s\n", account))
		}
	}
}
