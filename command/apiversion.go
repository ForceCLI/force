package command

import (
	"fmt"
	"regexp"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdApiVersion = &Command{
	Run:   runApiVersion,
	Usage: "apiversion",
	Short: "Display/Set current API version",
	Long: `
Display/Set current API version

Examples:

  force apiversion
  force apiversion 40.0
`,
}

func init() {
}

func runApiVersion(cmd *Command, args []string) {
	if len(args) == 1 {
		apiVersionNumber := args[0]
		matched, err := regexp.MatchString("^\\d{2}\\.0$", apiVersionNumber)
		if err != nil {
			ErrorAndExit("%v", err)
		}
		if !matched {
			ErrorAndExit("apiversion must be in the form of nn.0.")
		}
		force, err := ActiveForce()
		if err != nil {
			ErrorAndExit(err.Error())
		}
		err = force.UpdateApiVersion(apiVersionNumber)
		if err != nil {
			ErrorAndExit("%v", err)
		}
	} else if len(args) == 0 {
		fmt.Println(ApiVersion())
	} else {
		ErrorAndExit("The apiversion command only accepts a single argument in the form of nn.0")
	}
}
