package main

import (
	"fmt"
	"strings"
)

var cmdTrace = &Command{
	Run:   runTrace,
	Usage: "trace <command>",
	Short: "Manage trace flags",
	Long: `
Manage trace flags

Examples:

  force trace start [ <traceFlag>:<value> ]

  force trace list [format] [ <Field>:<value> ]

  force trace delete <id>
 
  * formats: csv, json, text
  * traceFlag: 
 	  ApexCode, ApexProfiling, Callout, Database, System, Validation, Visualforce, Workflow: Debug Level
      TracedEntityId: UserId
  * Field:

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
		var format = "json-pretty"
		if len(args) >= 2  {
			format = args[1]
		}		
		runQueryTrace(format)
	case "start":
		runStartTrace(args[1:])
	case "delete":
		if len(args) != 2 {
			ErrorAndExit("You need to provide the id of a TraceFlag to delete.")
		}
		runDeleteTrace(args[1])
	default:
		ErrorAndExit("no such command: %s", args[0])
	}
}

func runQueryTrace(format string) {
	force, _ := ActiveForce()
	result, err := force.QueryTraceFlags()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	DisplayForceRecordsf(result.Records, format)
}

func runStartTrace( args []string) {
	force, _ := ActiveForce()
	var traceFlags = new(TraceFlag)

	traceFlags.ApexCode = "Debug"
	traceFlags.ApexProfiling = "Error"
	traceFlags.Callout = "Info"
	traceFlags.Database = "Info"
	traceFlags.System = "Info"
	traceFlags.Validation = "Warn"
	traceFlags.Visualforce = "Info"
	traceFlags.Workflow = "Info"
	traceFlags.TracedEntityId = force.Credentials.UserId

	if len(args) > 0 {
		for _, value := range args {
			options := strings.Split(value, ":")
			if len(options) != 2 {
				ErrorAndExit(fmt.Sprintf("Missing value for trace flag %s", value))
			}
			switch ( strings.ToLower(options[0]) )  {
				case "apexcode": 
					traceFlags.ApexCode = options[1]
				case "apexprofiling": 
					traceFlags.ApexProfiling = options[1]
				case "callout": 
					traceFlags.Callout = options[1]
				case "database": 
					traceFlags.Database = options[1]
				case "system": 
					traceFlags.System = options[1]
				case "validation": 
					traceFlags.Validation = options[1]
				case "visualforce": 
					traceFlags.Visualforce = options[1]
				case "workflow": 
					traceFlags.Workflow = options[1]
				case "tracedentityid": 
					traceFlags.TracedEntityId = options[1]
				default:
					fmt.Printf("Format %s not supported\n\n", options[0] )
			}
		}
	}

	_, err, _ := force.StartTracet( traceFlags )
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
