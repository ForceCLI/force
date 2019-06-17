package command

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"

	. "github.com/ForceCLI/force/config"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdActive = &Command{
	Usage: "active [account]",
	Short: "Show or set the active force.com account",
	Long: `
Set the active force.com account

Examples:

  force active
  force active user@example.org

Options:
  -json, -j     Output in JSON format
  -local, -l    Set active account locally, for current directory
  -session, -s  Output session id
`,
	MaxExpectedArgs: 1,
}
var (
	tojson    bool
	account   string
	local     bool
	sessionId bool
)

func init() {
	cmdActive.Flag.BoolVar(&tojson, "j", false, "output to json")
	cmdActive.Flag.BoolVar(&tojson, "json", false, "output to json")
	cmdActive.Flag.StringVar(&account, "a", "", "deprecated: set active account")
	cmdActive.Flag.StringVar(&account, "account", "", "deprecated: set active account")
	cmdActive.Flag.BoolVar(&local, "l", false, "set active account for current directory")
	cmdActive.Flag.BoolVar(&local, "local", false, "set active account for current directory")
	cmdActive.Flag.BoolVar(&sessionId, "session", false, "display session id")
	cmdActive.Flag.BoolVar(&sessionId, "s", false, "display session id")
	cmdActive.Run = runActive
}

func runActive(cmd *Command, args []string) {
	if account == "" && len(args) == 0 {
		creds, err := ActiveCredentials(true)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		if sessionId {
			fmt.Println(creds.AccessToken)
		} else if tojson {
			fmt.Printf(fmt.Sprintf("{ \"login\": \"%s\", \"instanceUrl\": \"%s\", \"namespace\":\"%s\" }", creds.SessionName(), creds.InstanceUrl, creds.UserInfo.OrgNamespace))
		} else {
			fmt.Println(fmt.Sprintf("%s - %s - ns:%s", creds.SessionName(), creds.InstanceUrl, creds.UserInfo.OrgNamespace))
		}
	} else {
		if account == "" {
			account = args[0]
		}
		accounts, _ := Config.List("accounts")
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
			if local {
				Config.SaveLocal("current", "account", account)
			} else {
				Config.SaveGlobal("current", "account", account)
			}
		} else {
			ErrorAndExit(fmt.Sprintf("no such account %s\n", account))
		}
	}
}
