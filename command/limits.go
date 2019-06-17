package command

import (
	"fmt"
	"sort"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdLimits = &Command{
	Usage: "limits",
	Short: "Display current limits",
	Long: `
	Use the limits command to display limits information for your organization.

	 -- Max is the limit total for the organization.

	 -- Remaining is the total number of calls or events left for the organization.`,
	MaxExpectedArgs: 0,
}

func init() {
	cmdLimits.Run = runLimits
}

func runLimits(cmd *Command, args []string) {

	force, _ := ActiveForce()

	var result ForceLimits
	result, err := force.GetLimits()

	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		printLimits(result)
	}
}

func printLimits(result map[string]ForceLimit) {

	//sort keys
	var keys []string
	for k := range result {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	//print map
	for _, k := range keys {
		fmt.Println(k, "\n ", result[k].Max, "maximum\n", result[k].Remaining, "remaining\n ")
	}

}
