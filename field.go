package main

import (
	"fmt"
	"strings"
)

var cmdField = &Command{
	Run:   runField,
	Usage: "field",
	Short: "Manage custom fields",
	Long: `
Manage custom fields

Usage:

  force field create <object> <field>:<type>

  force field delete <object> <field>

Examples:

  force field create Todo Due:datetime

  force field delete Todo Due
`,
}

func runField(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
	} else {
		switch args[0] {
		case "create":
			runFieldCreate(args[1:])
		case "delete":
			runFieldDelete(args[1:])
		default:
			ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func runFieldCreate(args []string) {
	if len(args) < 2 {
		ErrorAndExit("must specify object and at least one field")
	}
	force, _ := ActiveForce()
	parts := strings.Split(args[1], ":")
	if len(parts) != 2 {
		ErrorAndExit("must specify name:type for fields")
	}
	if err := force.Metadata.CreateCustomField(args[0], parts[0], parts[1]); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Custom field created")
}

func runFieldDelete(args []string) {
	if len(args) < 2 {
		ErrorAndExit("must specify object and at least one field")
	}
	force, _ := ActiveForce()
	if err := force.Metadata.DeleteCustomField(args[0], args[1]); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Custom field deleted")
}
