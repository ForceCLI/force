package main

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/heroku/force/salesforce"
)

var cmdLogout = &Command{
	Usage: "logout",
	Short: "Log out from force.com",
	Long: `
  force logout -u=username

  Example:

    force logout -u=user@example.org
`,
}

func init() {
	cmdLogout.Run = runLogout
}

var (
	userName1 = cmdLogout.Flag.String("u", "", "Username for Soap Login")
)

func runLogout(cmd *Command, args []string) {
	if *userName1 == "" {
		fmt.Println("Missing required argument...")
		cmd.Flag.Usage()
		return
	}
	salesforce.Config.Delete("accounts", *userName1)
	if active, _ := salesforce.Config.Load("current", "account"); active == *userName1 {
		salesforce.Config.Delete("current", "account")
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
