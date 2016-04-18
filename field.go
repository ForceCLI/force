package main

import (
	"fmt"
	"strings"

	"github.com/heroku/force/util"
)

var cmdField = &Command{
	Run:   runField,
	Usage: "field",
	Short: "Manage sobject fields",
	Long: `
Manage sobject fields

Usage:

  force field list <object>
  force field create <object> <field>:<type> [<option>:<value>]
  force field delete <object> <field>
  force field type
  force field type <fieldtype>

Examples:

  force field list Todo__c
	force field create Inspection__c "Final Outcome":picklist picklist:"Pass, Fail, Redo"
  force field create Todo__c Due:DateTime required:true
  force field delete Todo__c Due
  force field type     #displays all the supported field types
  force field type email   #displays the required and optional attributes

`,
}

func runField(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
	} else {
		switch args[0] {
		case "list":
			runFieldList(args[1:])
		case "create", "add":
			runFieldCreate(args[1:])
		case "delete", "remove":
			runFieldDelete(args[1:])
		case "type":
			if len(args) == 1 {
				DisplayFieldTypes()
			} else if len(args) == 2 {
				DisplayFieldDetails(args[1])
			}
		default:
			util.ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func runFieldList(args []string) {
	if len(args) != 1 {
		util.ErrorAndExit("must specify object")
	}
	force, _ := ActiveForce()
	sobject, err := force.GetSobject(args[0])
	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	DisplayForceSobject(sobject)
}

func runFieldCreate(args []string) {
	if len(args) < 2 {
		util.ErrorAndExit("must specify object and at least one field")
	}

	force, _ := ActiveForce()

	parts := strings.Split(args[1], ":")
	if len(parts) != 2 {
		util.ErrorAndExit("must specify name:type for fields")
	}

	var optionMap = make(map[string]string)
	if len(args) > 2 {
		for _, value := range args[2:] {
			options := strings.Split(value, ":")
			if len(options) != 2 {
				util.ErrorAndExit(fmt.Sprintf("Missing value for field attribute %s", value))
			}
			optionMap[options[0]] = options[1]
		}
	}

	// Validate the options for this field type
	newOptions, err := force.Metadata.ValidateFieldOptions(parts[1], optionMap)
	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	if err := force.Metadata.CreateCustomField(args[0], parts[0], parts[1], newOptions); err != nil {
		util.ErrorAndExit(err.Error())
	}
	fmt.Println("Custom field created")
}

func runFieldDelete(args []string) {
	if len(args) < 2 {
		util.ErrorAndExit("must specify object and at least one field")
	}
	force, _ := ActiveForce()
	if err := force.Metadata.DeleteCustomField(args[0], args[1]); err != nil {
		util.ErrorAndExit(err.Error())
	}
	fmt.Println("Custom field deleted")
}
