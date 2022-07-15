package command

import (
	"fmt"
	"os/exec"
	"runtime"

	. "github.com/ForceCLI/force/config"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(logoutCmd)
}

var logoutCmd = &cobra.Command{
	Use:                   "logout",
	Short:                 "Log out from Force.com",
	Args:                  cobra.MaximumNArgs(0),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		runLogout()
	},
}

func runLogout() {
	accounts, _ := Config.List("accounts")
	if len(accounts) == 0 {
		ErrorAndExit("No logins, so a username cannot be assumed.")
	}
	username := force.Credentials.UserInfo.UserName
	Config.Delete("accounts", username)
	if active, _ := Config.Load("current", "account"); active == username {
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
