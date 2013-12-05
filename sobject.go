package main

import (
	"fmt"
	"strings"
)

var cmdSobject = &Command{
	Run:   runSobject,
	Usage: "sobject",
	Short: "Manage sobjects",
	Long: `
Manage sobjects

Usage:

  force sobject list

  force sobject create <object> [<field>:<type>]...

  force sobject delete <object>

Examples:

  force sobject list

  force sobject create Todo Description:string

  force sobject delete Todo
`,
}

func runSobject(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
	} else {
		switch args[0] {
		case "list":
			runSobjectList(args[1:])
		case "create", "add":
			runSobjectCreate(args[1:])
		case "delete", "remove":
			runSobjectDelete(args[1:])
		default:
			ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func runSobjectList(args []string) {
	force, _ := ActiveForce()
	sobjects, err := force.ListSobjects()
	if err != nil {
		ErrorAndExit(fmt.Sprintf("ERROR: %s\n", err))
	} else {
		DisplayForceSobjects(sobjects)
	}
}

func runSobjectCreate(args []string) {
	if len(args) < 2 {
		ErrorAndExit("must specify object and at least one field")
	}
	force, _ := ActiveForce()
	if err := force.Metadata.CreateCustomObject(args[0]); err != nil {
		ErrorAndExit(err.Error())
	}
	for _, field := range args[1:] {
		parts := strings.Split(field, ":")
		if len(parts) != 2 {
			ErrorAndExit("must specify name:type for fields")
		}
		if err := force.Metadata.CreateCustomField(fmt.Sprintf("%s__c", args[0]), parts[0], parts[1], nil); err != nil {
			ErrorAndExit(err.Error())
		}
	}
	fmt.Println("Custom object created")
}

func runSobjectDelete(args []string) {
	if len(args) < 1 {
		ErrorAndExit("must specify object")
	}
	force, _ := ActiveForce()
	if err := force.Metadata.DeleteCustomObject(args[0]); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Custom object deleted")
}
