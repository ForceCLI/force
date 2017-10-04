package command

import (
	"fmt"
	"os/exec"
	"runtime"

	. "github.com/heroku/force/config"
	. "github.com/heroku/force/error"
	. "github.com/heroku/force/lib"
)

var cmdLogout = &Command{
	Usage: "logout [-u=username]",
	Short: "Log out from Force.com",
	Long: `
Log out from Force.com

The username may be omitted when only one account is logged in, in which case
that single logged-in account will be logged out.

Examples:
  force logout
  force logout -u=user@example.org
`,
}

func init() {
	cmdLogout.Run = runLogout
}

var (
	logoutUserName = cmdLogout.Flag.String("u", "", "Username to log out")
)

func runLogout(cmd *Command, args []string) {
	/* If a username was specified, we will use it regardless of logins.
	 * Otherwise, if there's only one login, we'll use that. */
	if *logoutUserName == "" {
		accounts, _ := Config.List("accounts")
		if len(accounts) == 0 {
			ErrorAndExit("No logins, so a username cannot be assumed.")
		} else if len(accounts) > 1 {
			ErrorAndExit("More than one login. Please specify a username.")
		} else {
			logoutUserName = &accounts[0]
		}
	}
	Config.Delete("accounts", *logoutUserName)
	if active, _ := Config.Load("current", "account"); active == *logoutUserName {
		Config.Delete("current", "account")
		SetActiveLoginDefault()
	}
	if runtime.GOOS == "windows" {
		cmd := exec.Command("title", account)
		cmd.Run()
	} else {
		title := fmt.Sprintf("\033];%s\007", "")
		fmt.Printf(title)
	}
}
