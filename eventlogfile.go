package main

var cmdEventLogFile = &Command{
	Run:   getEventLogFile,
	Usage: "eventlogfile [eventlogfileId]",
	Short: "List and fetch event log file",
	Long: `
List and fetch event log file

Examples:
  force eventlogfile
  force eventlogfile 0AT300000000XQ7GAM
`,
}

func getEventLogFile(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) == 0 {
		records, err := force.QueryEventLogFiles()
		if err != nil {
			ErrorAndExit(err.Error())
		}
		DisplayForceRecords(records)
	} else {
		logId := args[0]
		log, err := force.RetrieveEventLogFile(logId)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		ConsolePrintln(log)
	}
}
