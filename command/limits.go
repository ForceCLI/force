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
		warn, _ := cmd.Flags().GetFloat64("warn")
		runLimits(warn)
	},
}

func init() {
	limitsCmd.Flags().Float64P("warn", "w", 10, "warning percentange.  highlight if remaining is less.")
	RootCmd.AddCommand(limitsCmd)
}

func runLimits(warn float64) {
	var result ForceLimits
	result, err := force.GetLimits()

	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		printLimits(result, warn)
	}
}

func printLimits(result map[string]ForceLimit, warnPercent float64) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Limit", "Maximum", "Remaining"})
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT})
	//sort keys
	var keys []string
	for k := range result {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		remaining := result[k].Remaining
		max := result[k].Max
		row := []string{k, strconv.FormatInt(max, 10), strconv.FormatInt(remaining, 10)}
		if max > 0 && float64(remaining)/float64(max) < warnPercent/100 {
			table.Rich(row, []tablewriter.Colors{tablewriter.Colors{}, tablewriter.Colors{}, tablewriter.Colors{tablewriter.BgRedColor}})
		}

		table.Append(row)
	}
	if table.NumLines() > 0 {
		table.Render()
	}
}
