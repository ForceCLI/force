package main

import (
	"fmt"
	"github.com/ddollar/dist"
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
	if Version == "dev" {
		ErrorAndExit("can't update dev cersion")
	}
	d := dist.NewDist("heroku/force", Version)
	to, err := d.Update()
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		fmt.Printf("updated to %s\n", to)
	}
}
