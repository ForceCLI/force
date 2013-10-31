package main

import (
	"fmt"
	"strings"
)

var cmdSobject = &Command{
	Run:   runSobject,
	Usage: "sobject",
	Short: "Manage standard & custom objects",
	Long: `
Manage custom objects

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

	l := make ([]ForceSobject, 0)
	for _, sobject := range sobjects {
		if len(args) == 1 {
			if strings.Contains(sobject["name"].(string), args[0]) {
				l = append(l, sobject)
			}
		} else {
			l = append(l, sobject)
		}
	}

	if err != nil {
		ErrorAndExit(fmt.Sprintf("ERROR: %s\n", err))
	} else {
		DisplayForceSobjects(l)
	}
}

func runSobjectCreate(args []string) {
	if len(args) < 1 {
		ErrorAndExit("must specify object name")
	}
	force, _ := ActiveForce()
	if err := force.Metadata.CreateCustomObject(args[0]); err != nil {
		ErrorAndExit(err.Error())
	}
	for _, field := range args[1:] {
		parts := strings.Split(field, ":")
		if len(parts) != 2 {
			ErrorAndExit("must specify name:type for fields")
		} else {
			if err := force.Metadata.CreateCustomField(fmt.Sprintf("%s__c", args[0]), parts[0], parts[1]); err != nil {
				ErrorAndExit(err.Error())
			}
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
