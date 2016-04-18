package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"sort"

	"github.com/heroku/force/salesforce"
	"github.com/heroku/force/util"
)

var cmdActive = &Command{
	Usage: "active -a [account]",
	Short: "Show or set the active force.com account",
	Long: `
Set the active force.com account

Examples:

  force active
  force active -a user@example.org
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
		account, _ := salesforce.Config.Load("current", "account")
		data, _ := salesforce.Config.Load("accounts", account)
		var creds salesforce.ForceCredentials
		json.Unmarshal([]byte(data), &creds)
		if tojson {
			fmt.Printf(fmt.Sprintf("{ \"login\": \"%s\", \"instanceUrl\": \"%s\", \"namespace\":\"%s\" }", account, creds.InstanceUrl, creds.Namespace))
		} else {
			fmt.Println(fmt.Sprintf("%s - %s - ns:%s", account, creds.InstanceUrl, creds.Namespace))
		}
	} else {
		//account := args[0]
		accounts, _ := salesforce.Config.List("accounts")
		i := sort.SearchStrings(accounts, account)
		if i < len(accounts) && accounts[i] == account {
			if runtime.GOOS == "windows" {
				cmd := exec.Command("title", account)
				cmd.Run()
			} else {
				title := fmt.Sprintf("\033];%s\007", account)
				fmt.Printf(title)
			}
			fmt.Printf("%s now active\n", account)
			salesforce.Config.Save("current", "account", account)
		} else {
			util.ErrorAndExit(fmt.Sprintf("no such account %s\n", account))
		}
	}
}
