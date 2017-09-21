package command

import (
	"fmt"
	"strings"

	. "github.com/heroku/force/lib"
	
)

var cmdRest = &Command{
	Run:   runRest,
	Usage: "rest <method> <url>",
	Short: "Execute a REST request",
	Long: `
Execute a REST request

Examples:

  force rest get "tooling/query?q=Select id From Account"

  force rest get appMenu/AppSwitcher
`,
}

func runRest(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) != 1 {
		cmd.PrintUsage()
	} else {
		data := force.RestQuery(args[0])
		data = strings.Replace(data, "null", "\"null\"", -1)
		fmt.Println(data)
	}
}
