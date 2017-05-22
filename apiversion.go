package main

import (
	"fmt"
	"regexp"
)

var apiVersionNumber = "39.0"
var apiVersion = fmt.Sprintf("v%s", apiVersionNumber)

var cmdApiVersion = &Command{
	Run:   runApiVersion,
	Usage: "apiversion",
	Short: "Display/Set current API version",
	Long: `
Display/Set current API version

Examples:

  force apiversion
  force apiversion 39.0
`,
}

func init() {
}

func parseApiVersion(args []string) {
	matcher := regexp.MustCompile(`^v?(\d+\.0)$`)
	matched := matcher.FindStringSubmatch(args[0])
	if matched == nil {
		ErrorAndExit("apiversion must be in the form of nn.0.")
	}

	apiVersionNumber = matched[1]
	apiVersion = fmt.Sprintf("v%s", apiVersionNumber)
}

func runApiVersion(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) == 1 {
		parseApiVersion(args)
		force.Credentials.ApiVersion = apiVersionNumber
		ForceSaveLogin(*force.Credentials)
	} else if len(args) == 0 {
		fmt.Println(apiVersion)
	} else {
		ErrorAndExit("The apiversion command only accepts a single argument in the form of nn.0")
	}
}
