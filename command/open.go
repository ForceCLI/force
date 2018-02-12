package command

import (
	"fmt"
	desktop "github.com/ForceCLI/force/desktop"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdOpen = &Command{
	Usage: "open",
	Short: "Open a browser window, logged into the active Salesforce org",
	Long: `
Open a browser window, logged into the active Salesforce org

  force open
`,
}

func init() {
	cmdOpen.Run = runOpen
}

func runOpen(cmd *Command, args []string) {
	force, _ := ActiveForce()
	_, err := force.Whoami()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	url := fmt.Sprintf("%s/secur/frontdoor.jsp?sid=%s", force.Credentials.InstanceUrl, force.Credentials.AccessToken)
	err = desktop.Open(url)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}
