package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

var cmdQuery = &Command{
	Usage: "query [-format=<json>] [-q] <soql statement>",
	Short: "Execute a SOQL statement",
	Long: `
Execute a SOQL statement

The -q flag enables quiet mode
The -format flag controls output format

Examples:

  force query select Id, Name, Account.Name From Contact

  force query -format=csv select Id, FirstName, LastName From Contact
`,
}

func init() {
	cmdQuery.Run = runQuery
}

var (
	qQueryFlag      = cmdQuery.Flag.Bool("q", false, "enters quiet mode")
	formatQueryFlag = cmdQuery.Flag.String("format", "console", "control output format")
)

func runQuery(cmd *Command, args []string) {
	force, _ := ActiveForce()
	var query = ""
	if len(args) == 0 {
		if !*qQueryFlag {
			fmt.Println(">> Start typing SOQL; press CTRL-D when finished\n")
		}
		stdin, err := ioutil.ReadAll(os.Stdin)
		if !*qQueryFlag {
			fmt.Println("\n\n>> Executing SOQL...\n")
		}
		if err != nil {
			ErrorAndExit(err.Error())
		}
		query = string(stdin)
	} else {
		query = strings.Join(args, " ")
	}
	records, err := force.Query(query)
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		if *formatQueryFlag == "console" {
			DisplayForceRecords(records)
		} else {
			DisplayForceRecordsf(records.Records, format)
		}		
	}
}
