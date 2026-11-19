package command

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	desktop "github.com/ForceCLI/force/desktop"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

var debug bool

func init() {
	openCmd.Flags().StringP("start", "s", "", "relative URL to open")
	openCmd.Flags().BoolVarP(&debug, "debug", "d", false, "show link")
	RootCmd.AddCommand(openCmd)
}

var openCmd = &cobra.Command{
	Use:   "open [account]",
	Short: "Open a browser window, logged into an authenticated Salesforce org",
	Long: `
Open a browser window, logged into an authenticated Salesforce org.
By default, the active account is used.
`,
	Example: `
  force open user@example.com
`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		if len(args) != 0 {
			force, err = GetForceLazy(args[0])
			if err != nil {
				ErrorAndExit(err.Error())
			}
		}
		startUrl, _ := cmd.Flags().GetString("start")
		runOpen(startUrl)
	},
}

func runOpen(startUrl string) {
	_, err := force.Whoami()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	openUrl, err := frontdoorUrl(force.Credentials)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if startUrl != "" {
		openUrl = fmt.Sprintf("%s&retURL=%s", openUrl, url.QueryEscape(startUrl))
	}
	if debug {
		fmt.Fprintln(os.Stderr, openUrl)
	}
	err = desktop.Open(openUrl)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

// frontdoorUrl returns the frontdoor.jsp URL that logs the browser in with
// the session.
func frontdoorUrl(creds *ForceSession) (string, error) {
	sid := creds.UISessionId()
	if sid == "" {
		return "", errors.New("This DPoP session has no UI session ID (ui_sid) to open the browser with. Run `force login` again.")
	}
	return fmt.Sprintf("%s/secur/frontdoor.jsp?sid=%s", creds.InstanceUrl, sid), nil
}
