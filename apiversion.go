package main

import (
	"fmt"

	"github.com/heroku/force/util"
)

var cmdApiVersion = &Command{
	Run:   runApiVersion,
	Usage: "apiversion",
	Short: "Display/Set current API version",
	Long: `
Display/Set current API version

Examples:

  force apiversion
  force apiversion 36.0
`,
}

func init() {
}

func runApiVersion(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) == 1 {
		var versionNumberArgument = args[0]
		// validate that the version is in the right format
		if []rune(versionNumberArgument)[0] != 'v' {
			util.ErrorAndExit("You must specify version number in the format vMM.mm")
		}
		force.Credentials.ApiVersion = versionNumberArgument
		ForceSaveLogin(force.Credentials)
	} else if len(args) == 0 {
		fmt.Println(force.Credentials.ApiVersion)
	} else {
		util.ErrorAndExit("The apiversion command only accepts a single argument in the form of nn.0")
	}
}
