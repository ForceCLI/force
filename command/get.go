package command

import (
	"fmt"

	. "github.com/heroku/force/error"
	. "github.com/heroku/force/lib"
)

var cmdGet = &Command{
	Run:   runGet,
	Usage: "get",
	Short: "Issue GET request against REST API",
	Long: `
Send GET request to REST API sendpoint

Examples:

  force get /recent
`,
}

func runGet(cmd *Command, args []string) {
	force, _ := ActiveForce()
	var url string
	if len(args) == 0 {
		url = "/"
	} else {
		url = args[0]
	}
	result, err := force.GetREST(url)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println(result)
}
