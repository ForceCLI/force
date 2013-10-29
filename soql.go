package main

import (
	"fmt"
	"strings"
)

var cmdSoql = &Command{
	Run:   runSoql,
	Usage: "soql",
	Short: "Execute a SOQL statement",
	Long: `
Execute a SOQL statement

Examples:

  force soql select id, name from user
`,
}

func runSoql(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) < 1 {
		ErrorAndExit("must specify a SOQL statement")
	}
	soql := strings.Join(args, " ")
	records, err := force.Query(fmt.Sprintf("%s", soql))
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		DisplayForceRecords(records)
	}
}
