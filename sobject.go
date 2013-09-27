package main

import (
	"fmt"
)

var cmdSobjects = &Command{
	Run:   runSobjects,
	Usage: "sobjects",
	Short: "List force.com objects",
	Long: `
List force.com objects

Examples:

  force sobjects
`,
}

func runSobjects(cmd *Command, args []string) {
	force, _ := ActiveForce()
	sobjects, err := force.ListSobjects()
	if err != nil {
		ErrorAndExit(fmt.Sprintf("ERROR: %s\n", err))
	} else {
		DisplayForceSobjects(sobjects)
	}
}
