package main

var cmdAPIVersion = &Command{
	Run:   runAPIVersion,
	Usage: "apiversion",
	Short: "Manage Force.com API versions",
	Long: `
Manage Force.com API versions

Usage:

  force apiversion list
`,
}

func runAPIVersion(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
	} else {
		switch args[0] {
		case "list":
			runAPIVersionList()
		default:
			ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func runAPIVersionList() {
	force, _ := ActiveForce()
	records, err := force.ListAPIVersions()
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
	  DisplayVersionRecords(records)	
	}
}
