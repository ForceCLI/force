package main

import (
	"fmt"

	"github.com/heroku/force/util"
)

var cmdTrace = &Command{
	Run:   runTrace,
	Usage: "trace <command>",
	Short: "Manage trace flags",
	Long: `
Manage trace flags

Examples:

  force trace start [user id]

  force trace list

  force trace delete <id>
`,
}

func init() {
}

func runTrace(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
		return
	}
	switch args[0] {
	case "list":
		runQueryTrace()
	case "start":
		if len(args) == 2 {
			runStartTrace(args[1])
		} else {
			runStartTrace()
		}
	case "delete":
		if len(args) != 2 {
			util.ErrorAndExit("You need to provide the id of a TraceFlag to delete.")
		}
		runDeleteTrace(args[1])
	default:
		util.ErrorAndExit("no such command: %s", args[0])
	}
}

func runQueryTrace() {
	force, _ := ActiveForce()
	result, err := force.QueryTraceFlags()
	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	DisplayForceRecordsf(result.Records, "json-pretty")
}

func runStartTrace(userId ...string) {
	force, _ := ActiveForce()
	_, err, _ := force.StartTrace(userId...)
	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	fmt.Printf("Tracing Enabled\n")
}

func runDeleteTrace(id string) {
	force, _ := ActiveForce()
	err := force.DeleteToolingRecord("TraceFlag", id)
	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	fmt.Printf("Trace Flag deleted\n")
}
