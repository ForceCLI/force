package command

import (
	"fmt"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(eventLogFileCmd)
}

var eventLogFileCmd = &cobra.Command{
	Use:   "eventlogfile [eventlogfileId]",
	Short: "List and fetch event log file",
	Example: `
  force eventlogfile
  force eventlogfile 0AT300000000XQ7GAM
`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			listEventLogFiles()
			return
		}
		getEventLogFile(args[0])
	},
}

func listEventLogFiles() {
	records, err := force.QueryEventLogFiles()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	DisplayForceRecords(records)
}

func getEventLogFile(logId string) {
	log, err := force.RetrieveEventLogFile(logId)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println(log)
}
