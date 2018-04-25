package command

import (
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"strings"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdQuery = &Command{
	Run:   runQuery,
	Usage: "query [options] <soql statement>",
	Short: "Execute a SOQL statement",
	Long: `
Execute a SOQL statement

Examples:

  force query "SELECT Id, Name, Account.Name FROM Contact"
  force query --format csv "SELECT Id, Name, Account.Name FROM Contact"
  force query --all "SELECT Id, Name FROM Account WHERE IsDeleted = true"
  force query --tooling "SELECT Id, TracedEntity.Name, ApexCode FROM TraceFlag"

Query Options
  --all, -a      Use QueryAll to include deleted and archived records in query results
  --tooling, -t  Use Tooling API
  --format, -f   Output format: csv, json, json-pretty, console
`,
}

var (
	queryAll          bool
	useTooling        bool
	queryOutputFormat string
)

func init() {
	defaultOutputFormat := "console"
	if !terminal.IsTerminal(int(os.Stdout.Fd())) {
		defaultOutputFormat = "csv"
	}
	cmdQuery.Flag.BoolVar(&queryAll, "all", false, "use queryAll to include deleted and archived records in query results")
	cmdQuery.Flag.BoolVar(&queryAll, "a", false, "use queryAll to include deleted and archived records in query results")
	cmdQuery.Flag.BoolVar(&useTooling, "tooling", false, "use Tooling API")
	cmdQuery.Flag.BoolVar(&useTooling, "t", false, "use Tooling API")
	cmdQuery.Flag.StringVar(&queryOutputFormat, "format", defaultOutputFormat, "output format: csv, json, json-pretty, console")
	cmdQuery.Flag.StringVar(&queryOutputFormat, "f", defaultOutputFormat, "output format: csv, json, json-pretty, console")
}

func runQuery(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) < 1 {
		cmd.PrintUsage()
	} else {
		var formatArg = ""
		var formatIndex = 1
		var queryOptions []func(*QueryOptions)
		if len(args) == 2 {
			fmt.Fprintln(os.Stderr, "Deprecated use of format argument.  Use --format before query.")
			formatArg = args[len(args)-formatIndex]
		} else if len(args) == 3 {
			fmt.Fprintln(os.Stderr, "Deprecated use of tooling argument.  Use --tooling.")

			formatIndex = 2
			formatArg = args[len(args)-formatIndex]
			useTooling = true
		}
		if queryAll {
			queryOptions = append(queryOptions, func(options *QueryOptions) {
				options.QueryAll = true
			})
		}
		if useTooling {
			queryOptions = append(queryOptions, func(options *QueryOptions) {
				options.IsTooling = true
			})
		}

		if strings.Contains(formatArg, "format:") {
			args = args[:len(args)-formatIndex]
			queryOutputFormat = strings.SplitN(formatArg, ":", 2)[1]
		}

		soql := strings.Join(args, " ")
		if queryOutputFormat == "console" {
			// All records have be queried before they are displayed so that
			// column widths can be calculated
			records, err := force.Query(fmt.Sprintf("%s", soql), queryOptions...)
			if err != nil {
				ErrorAndExit(err.Error())
			}
			DisplayForceRecords(records)
		} else {
			records := make(chan ForceRecord)
			done := make(chan bool)
			go DisplayForceRecordsf(records, queryOutputFormat, done)
			err := force.QueryAndSend(fmt.Sprintf("%s", soql), records, queryOptions...)
			if err != nil {
				ErrorAndExit(err.Error())
			}
			<-done
		}
	}
}
