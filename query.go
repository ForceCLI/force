package main

import (
	"strings"
)

var cmdQuery = &Command{
	Run:   runQuery,
	Usage: "query <soql>",
	Short: "Execute a SOQL query",
	Long: `
Execute a SOQL query

Examples:

  force query select id, name from user
`,
}

func runQuery(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) < 1 {
		ErrorAndExit("must specify query")
	}
	query := strings.Join(args, " ")
	records, err := force.Query(query)
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		DisplayForceRecords(records)
	}
}
