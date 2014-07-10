package main

import ( 
	"fmt"
	"sort"
)

var cmdActive = &Command{
	Run:   runActive,
	Usage: "active [account]",
	Short: "Show or set the active force.com account",
	Long: `
Set the active force.com account

Examples:

  force active
  force active user@example.org
`,
}

func runActive(cmd *Command, args []string) {
	if len(args) == 0 {
		account, _ := Config.Load("current", "account")
		fmt.Println(account)
	} else {
		account := args[0]
		accounts, _ := Config.List("accounts")
		i := sort.SearchStrings(accounts, account)
		if i < len(accounts) && accounts[i] == account {
			Config.Save("current", "account", account)
		} else {
			ErrorAndExit(fmt.Sprintf("no such account %s\n", account))
		}
	}
}
