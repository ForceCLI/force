package main

import (
	"fmt"
	"strings"
)

var cmdSoql = &Command{
	Run:   runSoql,
	Usage: "soql <soql statement> [output format]",
	Short: "Execute a SOQL statement",
	Long: `
Execute a SOQL statement

Examples:

  force soql select Id, Name, Account.Name From Contact

  force soql select Id, Name, Account.Name From Contact --format:csv
  
`,
}

func runSoql(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) < 1 {
		cmd.printUsage()
	} else {
		format := "console"
		formatArg := args[len(args)-1]

		if strings.Contains(formatArg, "format:") {
			args = args[:len(args) - 1]
			format = strings.SplitN(formatArg, ":", 2)[1]
		}

		soql := strings.Join(args, " ")
		records, err := force.Query(fmt.Sprintf("%s", soql))
		if err != nil {
			ErrorAndExit(err.Error())
		} else {
			if format == "console" {
				DisplayForceRecords(records)
			} else  {
				DisplayForceRecordsf(records.Records, format)
			}
		}
	}
}
