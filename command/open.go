package command

import (
	"fmt"
	"net/url"

	desktop "github.com/ForceCLI/force/desktop"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	openCmd.Flags().StringP("start", "s", "", "relative URL to open")
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
			force, err = GetForce(args[0])
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
	openUrl := fmt.Sprintf("%s/secur/frontdoor.jsp?sid=%s", force.Credentials.InstanceUrl, force.Credentials.AccessToken)
	if startUrl != "" {
		openUrl = fmt.Sprintf("%s&retURL=%s", openUrl, url.QueryEscape(startUrl))
	}
	err = desktop.Open(openUrl)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}
