package command

import (
	"fmt"
	"strings"
	"io/ioutil"

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
	if len(args) != 3 {
		cmd.PrintUsage()
	} else {
		// TODO parse args looking for get, post etc
		// and handle other than get
		var (
			data = ""
			msg = ""
		)
		var err error
		if strings.ToLower(args[0]) == "get" {
			data, err = force.GetREST(args[1])
			if err != nil {
				ErrorAndExit(err.Error())
			}
			msg = strings.Replace(data, "null", "\"null\"", -1)
		} else if strings.ToLower(args[0]) == "post" ||
			strings.ToLower(args[0]) == "patch" {
			url := args[1]
			datafile, err := ioutil.ReadFile(args[2])
			data, err = force.PostPatchREST(url, string(datafile), strings.ToUpper(args[0]))
			if err != nil {
				ErrorAndExit(err.Error())
			}
			data = string(data)
			data = strings.Replace(data, "null", "\"null\"", -1)
			msg = fmt.Sprintf("%s %s\n%s", strings.ToUpper(args[0]), url, data)
		}
		fmt.Println(msg)

	}
}
