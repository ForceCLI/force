package command

import (
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"strings"

	. "github.com/heroku/force/error"
	. "github.com/heroku/force/lib"
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
		cmd.PrintUsage()
	} else {
		format := "console"
		if !terminal.IsTerminal(int(os.Stdout.Fd())) {
			format = "csv"
		}
		var formatArg = ""
		var formatIndex = 1
		var queryOptions []func(*QueryOptions)
		if len(args) == 2 {
			formatArg = args[len(args)-formatIndex]
		} else if len(args) == 3 {
			formatIndex = 2
			formatArg = args[len(args)-formatIndex]
			tooling := func(options *QueryOptions) {
				options.IsTooling = true
			}
			queryOptions = append(queryOptions, tooling)
		}

		if strings.Contains(formatArg, "format:") {
			args = args[:len(args)-formatIndex]
			format = strings.SplitN(formatArg, ":", 2)[1]
		}

		soql := strings.Join(args, " ")

		records, err := force.Query(fmt.Sprintf("%s", soql), queryOptions...)

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
