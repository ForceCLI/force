package main

import (
	"fmt"

	"github.com/ddollar/dist"
	. "github.com/heroku/force/salesforce"
	"github.com/heroku/force/util"
)

var cmdUpdate = &Command{
	Run:   runUpdate,
	Usage: "update",
	Short: "Update to the latest version",
	Long: `
Update to the latest version

Examples:

	force update
`,
}

func init() {
}

func runUpdate(cmd *Command, args []string) {
	d := dist.NewDist("heroku/force", Version)
	if len(args) == 1 {
		err := d.FullUpdate(args[0])
		if err != nil {
			util.ErrorAndExit(err.Error())
		} else {
			fmt.Printf("updated to %s\n", args[0])
		}
	} else {
		if Version == "dev" {
			util.ErrorAndExit("can't update dev version")
		}
		to, err := d.Update()
		if err != nil {
			util.ErrorAndExit(err.Error())
		} else {
			fmt.Printf("updated to %s\n", to)
		}
	}
}
