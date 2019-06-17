package command

import (
	"fmt"

	desktop "github.com/ForceCLI/force/desktop"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdOpen = &Command{
	Usage: "open [account]",
	Short: "Open a browser window, logged into an authenticated Salesforce org",
	Long: `
Open a browser window, logged into an authenticated Salesforce org.
By default, the active account is used.

  force open [account]
`,
	MaxExpectedArgs: 1,
}

func init() {
	cmdOpen.Run = runOpen
}

func runOpen(cmd *Command, args []string) {
	var force *Force
	var err error
	if len(args) > 0 {
		force, err = GetForce(args[0])
	} else {
		force, err = ActiveForce()
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
	_, err = force.Whoami()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	url := fmt.Sprintf("%s/secur/frontdoor.jsp?sid=%s", force.Credentials.InstanceUrl, force.Credentials.AccessToken)
	err = desktop.Open(url)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}
