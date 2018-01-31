package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdRest = &Command{
	Run:   runRest,
	Usage: "rest <method> <url>",
	Short: "Execute a REST request",
	Long: `
Execute a REST request

Examples:

  force rest get "/tooling/query?q=Select id From Account"

  force rest get /appMenu/AppSwitcher

  force rest post "/tooling/sobjects/CustomField/00D9A0000000TgcUAE" path/to/definition.json

`,
}

func runRest(cmd *Command, args []string) {
	var (
		data = ""
		msg  = ""
		err  error
	)
	force, _ := ActiveForce()
	if len(args) == 0 {
		cmd.PrintUsage()
	} else if strings.ToLower(args[0]) == "get" {
		url := "/"
		if len(args) > 1 {
			url = args[1]
		}
		data, err = force.GetREST(url)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		msg = strings.Replace(data, "null", "\"null\"", -1)
		fmt.Println(msg)
	} else if strings.ToLower(args[0]) == "post" || strings.ToLower(args[0]) == "patch" {
		if len(args) < 2 {
			cmd.PrintUsage()
			os.Exit(1)
		}
		url := args[1]
		var input []byte
		if len(args) > 2 {
			input, err = ioutil.ReadFile(args[2])
		} else {
			input, err = ioutil.ReadAll(os.Stdin)
		}
		if err != nil {
			ErrorAndExit(err.Error())
		}
		data, err = force.PostPatchREST(url, string(input), strings.ToUpper(args[0]))
		if err != nil {
			ErrorAndExit(err.Error())
		}
		data = string(data)
		data = strings.Replace(data, "null", "\"null\"", -1)
		msg = fmt.Sprintf("%s %s\n%s", strings.ToUpper(args[0]), url, data)
		fmt.Println(msg)
	} else {
		cmd.PrintUsage()
		os.Exit(1)
	}

}
