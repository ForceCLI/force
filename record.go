package main

import (
	"fmt"
)

var cmdRecord = &Command{
	Run:   runRecord,
	Usage: "record <command> [<args>]",
	Short: "Create, modify, or view records",
	Long: `
Create, modify, or view records

Usage:

  force record get <object> <id>

  force record get <object> <extid>:<value>

  force record create <object> [<fields>]

  force record create:bulk <object> <file> [<format>]

  force record update <object> <id> [<fields>]

  force record update <object> <extid>:<value> [<fields>]

  force record delete <object> <id>

Examples:

  force record get User 00Ei0000000000

  force record get User username:user@name.org

  force record create User Name:"David Dollar" Phone:0000000000

  force record update User 00Ei0000000000 State:GA

  force record update User username:user@name.org State:GA

  force record delete User 00Ei0000000000
`,
}

func runRecord(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
	} else {
		switch args[0] {
		case "get":
			runRecordGet(args[1:])
		case "create", "add":
			runRecordCreate(args[1:])
		case "create:bulk":
			if len(args) == 3 {
				createBulkInsertJob(args[2], args[1], "CSV")
			} else if len(args) == 4 {
				createBulkInsertJob(args[2], args[1], args[3])
			}
		case "update":
			runRecordUpdate(args[1:])
		case "delete", "remove":
			runRecordDelete(args[1:])
		default:
			ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func runRecordGet(args []string) {
	if len(args) != 2 {
		ErrorAndExit("must specify object and id")
	}
	force, _ := ActiveForce()
	object, err := force.GetRecord(args[0], args[1])
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		DisplayForceRecord(object)
	}
}

func runRecordCreate(args []string) {
	if len(args) < 1 {
		ErrorAndExit("must specify object")
	}
	force, _ := ActiveForce()
	attrs := ParseArgumentAttrs(args[1:])
	id, err, emessages := force.CreateRecord(args[0], attrs)
	if err != nil {
		ErrorAndExit(err.Error(), emessages[0].ErrorCode)
	}
	fmt.Printf("Record created: %s\n", id)
}

func runRecordUpdate(args []string) {
	if len(args) < 2 {
		ErrorAndExit("must specify object and id")
	}
	force, _ := ActiveForce()
	attrs := ParseArgumentAttrs(args[2:])
	err := force.UpdateRecord(args[0], args[1], attrs)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Record updated")
}

func runRecordDelete(args []string) {
	if len(args) != 2 {
		ErrorAndExit("must specify object and id")
	}
	force, _ := ActiveForce()
	err := force.DeleteRecord(args[0], args[1])
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Record deleted")
}
