package command

import (
	"fmt"
	"strings"

	. "github.com/heroku/force/lib"
	. "github.com/heroku/force/error"	
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
	if len(args) != 2 {
		cmd.PrintUsage()
	} else {
		// TODO parse args looking for get, post etc
		// and handle other than get
		data, err := force.GetREST(args[1])
		if err != nil {
			ErrorAndExit(err.Error())
		}
		data = strings.Replace(data, "null", "\"null\"", -1)
		fmt.Println(data)
	}
}
