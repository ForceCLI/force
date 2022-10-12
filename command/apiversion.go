package command

import (
	"fmt"
	"regexp"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

var apiVersionCmd = &cobra.Command{
	Use:   "apiversion",
	Short: "Display/Set current API version",
	Example: `
  force apiversion
  force apiversion 40.0
`,
	Args:                  cobra.MaximumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 1 {
			setApiVersion(args[0])
		} else {
			fmt.Println(ApiVersion())
		}
	},
}

func init() {
	RootCmd.AddCommand(apiVersionCmd)
}

func setApiVersion(apiVersionNumber string) {
	matched, err := regexp.MatchString("^\\d{2}\\.0$", apiVersionNumber)
	if err != nil {
		ErrorAndExit("%v", err)
	}
	if !matched {
		ErrorAndExit("apiversion must be in the form of nn.0.")
	}
	err = force.UpdateApiVersion(apiVersionNumber)
	if err != nil {
		ErrorAndExit("%v", err)
	}
}
