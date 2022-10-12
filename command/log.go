package command

import (
	"fmt"

	. "github.com/ForceCLI/force/error"
	"github.com/spf13/cobra"
)

func init() {
	logCmd.AddCommand(deleteLogCmd)
	RootCmd.AddCommand(logCmd)
}

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Fetch debug logs",
	Example: `
  force log [list]
  force log <id>
  force log delete <id>
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 || args[0] == "list" {
			getAllLogs()
			return
		}
		getLog(args[0])
	},
	Args:                  cobra.MaximumNArgs(1),
	DisableFlagsInUseLine: true,
}

var deleteLogCmd = &cobra.Command{
	Use:   "delete [logId]",
	Short: "Delete debug logs",
	Example: `
  force log delete <id>
`,
	Run: func(cmd *cobra.Command, args []string) {
		deleteLog(args[0])
	},
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
}

func getAllLogs() {
	records, err := force.QueryLogs()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	force.DisplayAllForceRecords(records)
}

func deleteLog(logId string) {
	err := force.DeleteToolingRecord("ApexLog", logId)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Debug log deleted")
}

func getLog(logId string) {
	log, err := force.RetrieveLog(logId)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println(log)
}
