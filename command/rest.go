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
	Usage: "rest [-absolute | -a] <method> <url>",
	Short: "Execute a REST request",
	Long: `
Execute a REST request

Examples:

  force rest get "/tooling/query?q=Select id From Account"

  force rest get /appMenu/AppSwitcher

  force rest get -a /services/data/

  force rest post "/tooling/sobjects/CustomField/00D9A0000000TgcUAE" path/to/definition.json

  force rest put "/tooling/sobjects/CustomField/00D9A0000000TgcUAE" path/to/definition.json

`,
	MaxExpectedArgs: -1,
}

var (
	absoluteURLFlag bool
)

func init() {
	cmdRest.Flag.BoolVar(&absoluteURLFlag, "absolute", false, "use URL as-is (do not prepend /services/data/vXX.0)")
	cmdRest.Flag.BoolVar(&absoluteURLFlag, "a", false, "use URL as-is (do not prepend /services/data/vXX.0)")
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
		return
	}
	action := strings.ToUpper(args[0])
	switch action {
	case "GET":
		url := "/"
		if len(args) > 1 {
			url = args[1]
		}
		if absoluteURLFlag {
			data, err = force.GetAbsolute(url)
		} else {
			data, err = force.GetREST(url)
		}
		if err != nil {
			ErrorAndExit(err.Error())
		}
		msg = strings.Replace(data, "null", "\"null\"", -1)
		fmt.Println(msg)
	case "POST", "PATCH", "PUT":
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
		if absoluteURLFlag {
			data, err = force.PostPatchAbsolute(url, string(input), action)
		} else {
			data, err = force.PostPatchREST(url, string(input), action)
		}
		if err != nil {
			ErrorAndExit(err.Error())
		}
		data = string(data)
		data = strings.Replace(data, "null", "\"null\"", -1)
		msg = fmt.Sprintf("%s %s\n%s", action, url, data)
		fmt.Println(msg)
	default:
		cmd.PrintUsage()
		os.Exit(1)
	}

}
