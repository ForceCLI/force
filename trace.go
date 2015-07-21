package main

import (
	"fmt"
)

var cmdTrace = &Command{
	Run:   runTrace,
	Usage: "trace <command>",
	Short: "Manage trace flags",
	Long: `
Manage trace flags

Examples:

  force trace start

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
		runStartTrace()
	case "delete":
		if len(args) != 2 {
			ErrorAndExit("You need to provide the id of a TraceFlag to delete.")
		}
		runDeleteTrace(args[1])
	default:
		ErrorAndExit("no such command: %s", args[0])
	}
}

func runQueryTrace() {
	force, _ := ActiveForce()
	result, err := force.QueryTraceFlags()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	DisplayForceRecords(result)
}

func runStartTrace() {
	force, _ := ActiveForce()
	_, err, _ := force.StartTrace()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("Tracing Enabled\n")
}

func runDeleteTrace(id string) {
	force, _ := ActiveForce()
	err := force.DeleteToolingRecord("TraceFlag", id)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("Trace Flag deleted\n")
}
