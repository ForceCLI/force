package command

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	defaultOutputFormat := "console"
	if !terminal.IsTerminal(int(os.Stdout.Fd())) {
		defaultOutputFormat = "csv"
	}
	queryCmd.Flags().BoolP("all", "A", false, "use queryAll to include deleted and archived records in query results")
	queryCmd.Flags().BoolP("tooling", "t", false, "use Tooling API")
	queryCmd.Flags().StringP("format", "f", defaultOutputFormat, "output format: csv, json, json-pretty, console")
	RootCmd.AddCommand(queryCmd)
}

var queryCmd = &cobra.Command{
	Use:   "query [flags] <soql statement>",
	Short: "Execute a SOQL statement",
	Example: `
  force query "SELECT Id, Name, Account.Name FROM Contact"
  force query --format csv "SELECT Id, Name, Account.Name FROM Contact"
  force query --all "SELECT Id, Name FROM Account WHERE IsDeleted = true"
  force query --tooling "SELECT Id, TracedEntity.Name, ApexCode FROM TraceFlag"
  force query --user me@example.com "SELECT Id, Name, Account.Name FROM Contact"
`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		format, _ := cmd.Flags().GetString("format")
		allRows, _ := cmd.Flags().GetBool("all")
		tooling, _ := cmd.Flags().GetBool("tooling")
		query := strings.Join(args, " ")
		runQuery(query, format, allRows, tooling)
	},
}

func runQuery(query string, format string, queryAll bool, useTooling bool) {
	var queryOptions []func(*QueryOptions)
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

	if format == "console" {
		// All records have be queried before they are displayed so that
		// column widths can be calculated
		records, err := force.Query(fmt.Sprintf("%s", query), queryOptions...)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		DisplayForceRecords(records)
	} else {
		records := make(chan ForceRecord)
		done := make(chan bool)
		go DisplayForceRecordsf(records, format, done)
		err := force.QueryAndSend(fmt.Sprintf("%s", query), records, queryOptions...)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		<-done
	}
}
