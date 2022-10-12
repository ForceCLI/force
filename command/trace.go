package command

import (
	"fmt"

	. "github.com/ForceCLI/force/error"
	"github.com/spf13/cobra"
)

func init() {
	traceCmd.AddCommand(traceStartCmd)
	traceCmd.AddCommand(traceListCmd)
	traceCmd.AddCommand(traceDeleteCmd)
	RootCmd.AddCommand(traceCmd)
}

var traceStartCmd = &cobra.Command{
	Use:   "start [user id]",
	Short: "Set trace flag",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runStartTrace(args...)
	},
}

var traceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List trace flags",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		runQueryTrace()
	},
}

var traceDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete trace flag",
	Args:  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runDeleteTrace(args[0])
	},
}

var traceCmd = &cobra.Command{
	Use:   "trace <command>",
	Short: "Manage trace flags",
	Example: `
  force trace start [user id]
  force trace list
  force trace delete <id>
`,
}

func runQueryTrace() {
	result, err := force.QueryTraceFlags()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	force.DisplayAllForceRecordsf(result, "json-pretty")
}

func runStartTrace(userId ...string) {
	_, err, _ := force.StartTrace(userId...)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("Tracing Enabled\n")
}

func runDeleteTrace(id string) {
	err := force.DeleteToolingRecord("TraceFlag", id)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("Trace Flag deleted\n")
}
