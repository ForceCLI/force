package command

import (
	"fmt"

	desktop "github.com/ForceCLI/force/desktop"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
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
		runOpen()
	},
}

func runOpen() {
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
