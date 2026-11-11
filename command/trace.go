package command

import (
	"fmt"

	. "github.com/ForceCLI/force/error"
	"github.com/spf13/cobra"
)

func init() {
	traceStartCmd.Flags().Bool("replay", false, "log Apex at FINEST, so that logs can be opened with force log debug")
	traceCmd.AddCommand(traceStartCmd)
	traceCmd.AddCommand(traceListCmd)
	traceCmd.AddCommand(traceDeleteCmd)
	RootCmd.AddCommand(traceCmd)
}

var traceStartCmd = &cobra.Command{
	Use:   "start [user id]",
	Short: "Set trace flag",
	Long: `Set trace flag

Without a user id, the trace flag is for the current user. The trace flag
uses the Force_CLI debug level, which logs Apex at FINE. With --replay, it uses
the Force_CLI_Replay debug level, which logs Apex at FINEST, the level that
force log debug needs to step through a log.
`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		replay, _ := cmd.Flags().GetBool("replay")
		runStartTrace(replay, args...)
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
  force trace start --replay
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

func runStartTrace(replay bool, userId ...string) {
	start := force.StartTrace
	if replay {
		start = force.StartReplayTrace
	}
	_, err, _ := start(userId...)
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
