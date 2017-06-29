package main

import (
	"fmt"
	"os"
	"regexp"
)

var apiVersionNumber = "40.0"
var apiVersion = fmt.Sprintf("v%s", apiVersionNumber)

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
	force, _ := ActiveForce()
	if len(args) == 1 {
		apiVersionNumber = args[0]
		matched, err := regexp.MatchString("^\\d{2}\\.0$", apiVersionNumber)
		if err != nil {
			ErrorAndExit("%v", err)
		}
		if !matched {
			ErrorAndExit("apiversion must be in the form of nn.0.")
		}
		apiVersion = fmt.Sprintf("v%s", apiVersionNumber)
		force.Credentials.ApiVersion = apiVersionNumber
		ForceSaveLogin(*force.Credentials, os.Stdout)
	} else if len(args) == 0 {
		fmt.Println(apiVersion)
	} else {
		ErrorAndExit("The apiversion command only accepts a single argument in the form of nn.0")
	}
}
