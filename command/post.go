package command

import (
	"fmt"
	"io/ioutil"
	"os"

	. "github.com/heroku/force/error"
	. "github.com/heroku/force/lib"
)

var cmdPost = &Command{
	Run:   runPost,
	Usage: "post",
	Short: "Issue POST request against REST API",
	Long: `
Send POST request to REST API sendpoint

Examples:

force post /tooling/runSynchronousTests/ '{"tests": [{"classId": "01p540000005LrkAAE", "testMethods": ["firstTest"]}]}'
`,
}

func runPost(cmd *Command, args []string) {
	force, _ := ActiveForce()
	var url, body string
	if len(args) == 0 {
		cmd.PrintUsage()
		return
	}
	url = args[0]
	if len(args) == 2 {
		body = args[1]
	} else {
		stdin, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		body = string(stdin)
	}
	result, err := force.PostREST(url, body)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println(string(result))
}
