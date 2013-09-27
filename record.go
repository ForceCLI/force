package main

import ()

var cmdRecord = &Command{
	Run:   runRecord,
	Usage: "record <command> [<args>]",
	Short: "Create, modify, or view records",
	Long: `
Create, modify, or view records

Examples:

	force record list User

  force record get User 00Ei0000000000

	force record create User Name:"David Dollar" Phone:0000000000

	force record update User 00Ei0000000000 State:GA

	force record search User Name:"David Dollar"
`,
}

func runRecord(cmd *Command, args []string) {
	switch args[0] {
	case "list":
		runRecordList(args[1:])
	case "get":
		runRecordGet(args[1:])
	case "create":
		runRecordCreate(args[1:])
	case "update":
		runRecordUpdate(args[1:])
	case "search":
		runRecordUpdate(args[1:])
	default:
		ErrorAndExit("so such subcommand for record: %s", args[0])
	}
}

func runRecordList(args []string) {
	ErrorAndExit("not implemented yet")
}

func runRecordGet(args []string) {
	force, _ := ActiveForce()
	if len(args) != 2 {
		ErrorAndExit("must specify type and id")
	}
	object, err := force.Get(args[0], args[1])
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		DisplayForceObject(object)
	}
}

func runRecordCreate(args []string) {
	ErrorAndExit("not implemented yet")
}

func runRecordUpdate(args []string) {
	ErrorAndExit("not implemented yet")
}

func runRecordSearch(args []string) {
	ErrorAndExit("not implemented yet")
}
