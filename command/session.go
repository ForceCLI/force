package command

import (
	"fmt"

	. "github.com/heroku/force/error"
	. "github.com/heroku/force/lib"
)

var cmdSession = &Command{
	Run:   runSession,
	Usage: "session",
	Short: "Print session id",
	Long: `
Print the current session id

Examples:

  force session
`,
}

func runSession(cmd *Command, args []string) {
	force, _ := ActiveForce()
	_, err := force.Whoami()
	if err != nil {
		ErrorAndExit(err.Error())
	} else if len(args) == 0 {
		fmt.Println(force.Credentials.AccessToken)
	}
}
