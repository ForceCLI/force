package main

import (
	"fmt"
	"strings"

	"github.com/heroku/force/util"
)

var cmdQuery = &Command{
	Run:   runQuery,
	Usage: "query <soql statement> [output format]",
	Short: "Execute a SOQL statement",
	Long: `
Execute a SOQL statement

Examples:

  force query "select Id, Name, Account.Name From Contact"

  force query "select Id, Name, Account.Name From Contact" --format:csv

  force query "select Id, Name From Account Where MailingState IN ('CA', 'NY')"
`,
}

func runQuery(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) < 1 {
		cmd.printUsage()
	} else {
		format := "console"
		var formatArg = ""
		var isTooling = false
		var formatIndex = 1
		if len(args) == 2 {
			formatArg = args[len(args)-formatIndex]
		} else if len(args) == 3 {
			formatIndex = 2
			formatArg = args[len(args)-formatIndex]
			isTooling = true
		}

		if strings.Contains(formatArg, "format:") {
			args = args[:len(args)-formatIndex]
			format = strings.SplitN(formatArg, ":", 2)[1]
		}

		soql := strings.Join(args, " ")

		records, err := force.Query(fmt.Sprintf("%s", soql), isTooling)

		if err != nil {
			util.ErrorAndExit(err.Error())
		} else {
			if format == "console" {
				DisplayForceRecords(records)
			} else {
				DisplayForceRecordsf(records.Records, format)
			}
		}
	}
}
