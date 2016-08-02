package main

import (
	"fmt"
	"sort"

	"github.com/heroku/force/salesforce"
	"github.com/heroku/force/util"
)

var cmdLimits = &Command{
	Usage: "limits",
	Short: "Display current limits",
	Long: `
	Use the limits command to display limits information for your organization.

	 -- Max is the limit total for the organization.

	 -- Remaining is the total number of calls or events left for the organization.`,
}

func init() {
	cmdLimits.Run = runLimits
}

func runLimits(cmd *Command, args []string) {

	force, _ := ActiveForce()

	var result salesforce.ForceLimits
	result, err := force.GetLimits()

	if err != nil {
		util.ErrorAndExit(err.Error())
	} else {
		printLimits(result)
	}
}

func printLimits(result map[string]salesforce.ForceLimit) {

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
