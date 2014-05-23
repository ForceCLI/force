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
Execute a SOQL statement

Examples:

  force query select id, name from user

  force query select Id, FirstName, LastName From Contact format:json
`,
}

func runQuery(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) < 1 {
		cmd.printUsage()
	} else {
		format := "console"
		formatArg := args[len(args)-1]

		if strings.Contains(formatArg, "format:") {
			args = args[:len(args)-1]
			format = strings.SplitN(formatArg, ":", 2)[1]
		}

		soql := strings.Join(args, " ")
		records, err := force.Query(fmt.Sprintf("%s", soql))
		if err != nil {
			ErrorAndExit(err.Error())
		} else {
			if format == "console" {
				DisplayForceRecords(records)
			} else {
				DisplayForceRecordsf(records.Records, format)
			}
		}
	}
}
