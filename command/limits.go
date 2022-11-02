package command

import (
	"os"
	"sort"
	"strconv"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var limitsCmd = &cobra.Command{
	Use:   "limits",
	Short: "Display current limits",
	Long: `
	Use the limits command to display limits information for your organization.

	 -- Max is the limit total for the organization.

	 -- Remaining is the total number of calls or events left for the organization.`,
	Args: cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		runLimits()
	},
}

func init() {
	RootCmd.AddCommand(limitsCmd)
}

func runLimits() {
	var result ForceLimits
	result, err := force.GetLimits()

	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		printLimits(result)
	}
}

func printLimits(result map[string]ForceLimit) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Limit", "Maximum", "Remaining"})
	//sort keys
	var keys []string
	for k := range result {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		table.Append([]string{k, strconv.FormatInt(result[k].Max, 10), strconv.FormatInt(result[k].Remaining, 10)})
	}
	if table.NumLines() > 0 {
		table.Render()
	}
}
