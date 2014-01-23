package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

var cmdQuery = &Command{
	Run:   runQuery,
	Usage: "query <soql> [format:<json, csv>]",
	Short: "Execute a SOQL query, optionally specify output format",
	Long: `
Execute a SOQL query

Examples:

  force query select id, name from user

  force query select Id, FirstName, LastName From Contact format:json
`,
}

func runQuery(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) < 1 {
		ErrorAndExit("must specify a query")
	}
	var query = ""
	if args[len(args)-1] == "format:json" {
		query = strings.Join(args[:len(args)-1], " ")
	} else {
		query = strings.Join(args, " ")
	}
	records, err := force.Query(query)
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		if args[len(args)-1] == "format:json" {
			d, err := json.MarshalIndent(records, "", " ")
			if err != nil {
				ErrorAndExit(err.Error())
			}
			fmt.Println(string(d))
		} else {
			DisplayForceRecords(records)
		}
	}
}
