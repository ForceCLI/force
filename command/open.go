package command

import (
	"fmt"
	"strings"
	"os/exec"
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
  force open -b firefox
  force open -b "firefox --private-window"
  force open --browser google-chrome
`,
}
var (
	browser     string
)

func init() {
	cmdOpen.Flag.StringVar(&browser, "browser", "", "Command for the browser to open instead of the system default.")
	cmdOpen.Flag.StringVar(&browser, "b", "", "Command for the browser to open instead of the system default.")
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
	if len(browser) != 0 {
		err = openBrowser(browser, url)
	} else {
		err = desktop.Open(url)
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func openBrowser(browser string, url string) error {
	run := strings.Fields(browser)
	run = append(run, url)
	cmd := exec.Command(run[0], run[1:]...)
	return cmd.Start()
}
