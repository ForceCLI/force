package main

import (
	"fmt"
	"strings"
)

var cmdTrace = &Command{
	Run:   runTrace,
	Usage: "trace <command>",
	Short: "force trace <action> [<format>] [<Field>:<value>]",
	Long: `
Manage trace flags, giving the flexibility of create (start), delete and query the trace flags records

  force trace start [<traceflag>]

  force trace list [<format>] [<Filter Criteria>]

  force trace delete { <id> | <Filter Criteria> }

  <action>
  	- start: create a new trace flag that will debug depending the trace flags that receive
  	- list: query the trace flags and display in the format defined (default json-pretty). If no filter criteria is define will show all the records
  	- delete: erase all the trace records that match with the Filter Criteria

  <format>: 
      Set how the results will be shown. Values can be:
        csv, json, json-pretty or text
 
  <traceflag>: 
      Set the level of the Debug. Is a list of Log:<TraceOptions>  and/or <TraceFlag>:<value>. 
      <TraceOptions> can be one or more of the following values (by default use general).
        dml, soql, code, heap, general
      <TraceFlag> instead of have set of options can define a value for each trace field. If they are combined with Log will set the lowest level
        TracedEntityId: UserId (default value is the active force UserId)
 	  	ApexCode, ApexProfiling, Callout, Database, System, Validation, Visualforce, Workflow: none, error, warn, info, debug, fine, finer, finest
  
  <Filter Criteria>
  	  A list of <Field>:<value> that will be use do make the where condition

Examples:
  force trace start [TraceFlag]

  force trace list [Format] [Filter Criteria]
  force trace start [user id]

  force trace delete <id> 

  force trace delete [ <Field>:<value> ] 
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
		if len(args) >= 2 {
			format = args[1]
		}
		where := getWhereCondition(args[2:])
		runQueryTrace(format, where)
	case "start":
		var traceFlags TraceFlag
		if len(args) == 0 {
			traceFlags = getTraceFlags([]string{"log:general"})
		} else {
			traceFlags = getTraceFlags(args[1:])
		}
		runStartTrace(traceFlags)
	case "delete":
		if len(args) < 2 {
			ErrorAndExit("You need to provide the id of a TraceFlag to delete or the Filter Fields Criteria.")
		} else if len(args) == 2 {
			runDeleteTrace(args[1])
		} else {
			where := getWhereCondition(args[2:])
			runDeleteTraces(where)
		}

	default:
		ErrorAndExit("no such command: %s", args[0])
	}
}

func runQueryTrace(format string, where string) {
	force, _ := ActiveForce()
	result, err := force.QueryTraceFlags(where)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	DisplayForceRecordsf(result.Records, format)
}

func getTraceFlags(args []string) TraceFlag {
	force, _ := ActiveForce()

	result, err := force.Query("SELECT Id FROM DebugLevel LIMIT 1", true)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if len(result.Records) == 0 {
		ErrorAndExit("You need to create and/or define a DebugLevel")
	}
	debugLevelId := result.Records[0]["Id"].(string)

	var traceFlags = TraceFlag{TracedEntityId: force.Credentials.UserId, ApexCode: "None", ApexProfiling: "None", Callout: "None", Database: "None", System: "None", Validation: "None", Visualforce: "None", Workflow: "None", DebugLevelId: debugLevelId, LogType: "USER_DEBUG"}

	debugLevels := map[string]int{"none": 0, "error": 1, "warn": 2, "info": 3, "debug": 4, "fine": 5, "finer": 6, "finest": 7}
	debugLogs := map[string]map[string]string{
		"general": {"apexcode": "debug", "apexprofiling": "error", "callout": "info", "database": "info", "system": "info", "validation": "warn", "visualforce": "info", "workflow": "info"},
		"heap":    {"apexcode": "finer", "apexprofiling": "none", "callout": "none", "database": "none", "system": "none", "validation": "none", "visualforce": "none", "workflow": "none"},
		"soql":    {"apexcode": "debug", "apexprofiling": "none", "callout": "none", "database": "info", "system": "none", "validation": "none", "visualforce": "none", "workflow": "none"},
		"code":    {"apexcode": "debug", "apexprofiling": "none", "callout": "none", "database": "none", "system": "none", "validation": "none", "visualforce": "none", "workflow": "none"},
		"dml":     {"apexcode": "debug", "apexprofiling": "none", "callout": "none", "database": "info", "system": "none", "validation": "none", "visualforce": "none", "workflow": "none"},
	}

	for _, optionValue := range args {
		options := strings.Split(optionValue, ":")
		var key, value string
		if len(options) != 2 {
			// If is only the userId
			if options[0][:3] == "005" {
				key = "tracedentityid"
				value = options[0]
			} else {
				fmt.Printf("Missing value for trace flag %s", optionValue)
			}
		} else {
			key = strings.ToLower(options[0])
			value = options[1]
		}

		if key != "tracedentityid" {
			value = strings.ToLower(value)
		}
		var levels map[string]string
		if key == "log" {
			levels = debugLogs[value]
		} else {
			levels = map[string]string{key: value}
		}

		for key, value := range levels {
			switch key {
			case "apexcode":
				if debugLevels[value] > debugLevels[traceFlags.ApexCode] {
					traceFlags.ApexCode = value
				}
			case "apexprofiling":
				if debugLevels[value] > debugLevels[traceFlags.ApexProfiling] {
					traceFlags.ApexProfiling = value
				}
			case "callout":
				if debugLevels[value] > debugLevels[traceFlags.Callout] {
					traceFlags.Callout = value
				}
			case "database":
				if debugLevels[value] > debugLevels[traceFlags.Database] {
					traceFlags.Database = value
				}
			case "system":
				if debugLevels[value] > debugLevels[traceFlags.System] {
					traceFlags.System = value
				}
			case "validation":
				if debugLevels[value] > debugLevels[traceFlags.Validation] {
					traceFlags.Validation = value
				}
			case "visualforce":
				if debugLevels[value] > debugLevels[traceFlags.Visualforce] {
					traceFlags.Visualforce = value
				}
			case "workflow":
				if debugLevels[value] > debugLevels[traceFlags.Workflow] {
					traceFlags.Workflow = value
				}
			case "tracedentityid":
				traceFlags.TracedEntityId = value
			default:
				fmt.Printf("Format %s not supported\n\n", key)
			}
		}
	}
	return traceFlags
}

func runStartTrace(traceFlags TraceFlag) {
	force, _ := ActiveForce()
	_, err, _ := force.StartTracet(&traceFlags)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("Tracing Enabled\n")
}

func getWhereCondition(args []string) string {
	where := ""

	for _, value := range args {
		options := strings.Split(value, ":")
		if len(options) != 2 {
			fmt.Printf("Missing value for trace field filter criteria %s", options[0])
		} else {
			where += " AND " + strings.Trim(options[0], " ") + "='" + strings.Trim(options[1], " ") + "'"
		}
	}
	if where != "" {
		where = " WHERE " + where[5:]
	}
	return where
}
func runDeleteTraces(where string) {
	force, _ := ActiveForce()

	result, err := force.Query("SELECT Id FROM TraceFlag "+where, true)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	for _, record := range result.Records {
		err := force.DeleteToolingRecord("TraceFlag", record["Id"].(string))
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
	fmt.Printf("Trace Flags deleted\n")
}

func runDeleteTrace(id string) {
	force, _ := ActiveForce()

	err := force.DeleteToolingRecord("TraceFlag", id)

	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("Trace Flag deleted\n")
}
