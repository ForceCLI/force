package main

var cmdLog = &Command{
	Run:   getLog,
	Usage: "log",
	Short: "Fetch debug logs",
	Long: `
Fetch debug logs

Examples:

  force log [list]

  force log 07Le000000sKUylEAG
`,
}

func init() {
}

func getLog(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) == 0 || args[0] == "list" {
		records, err := force.QueryLogs()
		if err != nil {
			ErrorAndExit(err.Error())
		}
		DisplayForceRecords(records)
	} else {
		logId := args[0]
		log, err := force.RetrieveLog(logId)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		ConsolePrintln(log)
	}
}
