package main

import (
	"fmt"
)

var cmdSobjects = &Command{
	Run:   runSobjects,
	Usage: "sobject",
	Short: "Manage Force.com objects",
	Long: `
Manage force.com objects

Usage:

  force sobject list

  force sobject get <object>

  force sobject create <object> [<field>:<type>]...

  force sobject delete <object>

  force sobject add <object> <field>:<type>

  force sobject remove <object> <field>

Examples:

  force sobject list

  force sobject get Accounts

  force sobject create Todo Description:string

  force sobject delete Todo

  force sobject add Todo Due:datetime

  force sobject remove Todo Due
`,
}

func runSobjects(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
	} else {
		switch args[0] {
		case "list":
			runSobjectsList(args[1:])
		case "get":
			runSobjectsGet(args[1:])
		case "create":
			runSobjectsCreate(args[1:])
		case "delete":
			runSobjectsDelete(args[1:])
		case "add":
			runSobjectsAdd(args[1:])
		case "remove":
			runSobjectsRemove(args[1:])
		default:
			ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func runSobjectsList(args []string) {
	force, _ := ActiveForce()
	sobjects, err := force.ListSobjects()
	if err != nil {
		ErrorAndExit(fmt.Sprintf("ERROR: %s\n", err))
	} else {
		DisplayForceSobjects(sobjects)
	}
}

func runSobjectsGet(args []string) {
	if len(args) != 1 {
		ErrorAndExit("must specify object")
	}
	force, _ := ActiveForce()
	sobject, err := force.GetSobject(args[0])
	if err != nil {
		ErrorAndExit(err.Error())
	}
	DisplayForceSobject(sobject)
}

func runSobjectsCreate(args []string) {
	ErrorAndExit("not implemented yet")
}

func runSobjectsDelete(args []string) {
	ErrorAndExit("not implemented yet")
}

func runSobjectsAdd(args []string) {
	ErrorAndExit("not implemented yet")
}

func runSobjectsRemove(args []string) {
	ErrorAndExit("not implemented yet")
}
