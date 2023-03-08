package command

import (
	"fmt"
	"strings"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(searchCmd)
}

var searchCmd = &cobra.Command{
	Use:   "search [flags] <sosl statement>",
	Short: "Execute a SOSL statement",
	Example: `
  force search "FIND {Jane Doe} IN ALL FIELDS RETURNING Account (Id, Name)"
`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		format, _ := cmd.Flags().GetString("format")
		query := strings.Join(args, " ")
		runSearch(query, format)
	},
}

func runSearch(query string, format string) {
	records, err := force.Search(query)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Print(RenderForceRecords(records))
}
