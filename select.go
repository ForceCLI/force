package main

import (
	"fmt"
	"strings"
)

var cmdSelect = &Command{
	Run:   runSelect,
	Usage: "select <soql>",
	Short: "Execute a SOQL select",
	Long: `
Execute a SOQL select

Examples:

  force select id, name from user
`,
}

func runSelect(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) < 1 {
		ErrorAndExit("must specify select")
	}
	query := strings.Join(args, " ")
	records, err := force.Query(fmt.Sprintf("select %s", query))
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		DisplayForceRecords(records)
	}
}
