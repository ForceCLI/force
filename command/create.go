package command

import (
	"fmt"
	"strings"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdCreate = &Command{
	Usage: "create --type <ApexClass, ApexPage, ApexComponent, ApexTrigger> --name <item name> [--sobject <trigger object>]",
	Short: "Creates a new, empty Apex Class, Trigger, Visualforce page, or Component.",
	Long: `
Creates a new, empty Apex Class, Trigger, Visualforce page, or Component.

Examples:

  force create -t ApexClass -n NewController

  force create -t ApexTrigger -n NewTrigger -s Account

  force create -t ApexPage -n CoolPage

  force create -t ApexComponent -n CoolComponent
`,
	MaxExpectedArgs: 0,
}
var (
	what        string
	sObjectName string
	itemName    string
)

func init() {
	cmdCreate.Flag.StringVar(&what, "type", "", "What type of thing to create (currently only Apex or Visualforce).")
	cmdCreate.Flag.StringVar(&what, "t", "", "What type of thing to create (currently only Apex or Visualforce).")
	cmdCreate.Flag.StringVar(&sObjectName, "sobject", "", "For which sobject should the trigger be created.")
	cmdCreate.Flag.StringVar(&sObjectName, "s", "", "For which sobject should the trigger be created.")
	cmdCreate.Flag.StringVar(&itemName, "n", "", "Name of thing to be created.")
	cmdCreate.Flag.StringVar(&itemName, "name", "", "Name of thing to be created.")
	cmdCreate.Flag.StringVar(&what, "what", "", "What type of thing to create [deprecated: use type]")
	cmdCreate.Flag.StringVar(&what, "w", "", "What type of thing to create [deprecated: use t]")
	cmdCreate.Run = runCreate
}

func runCreate(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(what) == 0 || len(itemName) == 0 {
		cmd.PrintUsage()
	} else {
		attrs := make(map[string]string)
		switch strings.ToLower(what) {
		case "apexclass":
			attrs = getApexDefinition()
		case "apextrigger":
			if len(sObjectName) == 0 {
				cmd.PrintUsage()
				return
			}
			attrs = getTriggerDefinition()
		case "apexcomponent":
			attrs = getVFComponentDefinition()
		case "visualforce", "apexpage":
			what = "apexpage"
			attrs = getVFDefinition()
		}

		_, err := force.CreateToolingRecord(what, attrs)
		if err != nil {
			ErrorAndExit(fmt.Sprintf("Failed to create %s %s: %s", itemName, what, err.Error()))
		} else {
			fmt.Printf("Created new %s named %s.\n", what, itemName)
		}
	}
}

func getVFDefinition() (attrs map[string]string) {
	attrs = make(map[string]string)
	attrs["markup"] = "<apex:page>\n\n</apex:page>"
	attrs["name"] = itemName
	attrs["masterlabel"] = strings.Replace(itemName, " ", "_", -1)
	return
}

func getVFComponentDefinition() (attrs map[string]string) {
	attrs = make(map[string]string)
	attrs["markup"] = "<apex:component>\n\n</apex:component>"
	attrs["name"] = itemName
	attrs["masterlabel"] = strings.Replace(itemName, " ", "_", -1)
	return
}

func getApexDefinition() (attrs map[string]string) {
	attrs = make(map[string]string)
	attrs["status"] = "Active"
	attrs["body"] = fmt.Sprintf("public with sharing class %s {\n\n}", itemName)
	attrs["name"] = itemName
	return
}

func getTriggerDefinition() (attrs map[string]string) {
	attrs = make(map[string]string)
	attrs["status"] = "Active"
	attrs["body"] = fmt.Sprintf("trigger %s on %s (before insert, after insert, before update, after update, before delete, after delete, after undelete) { \n\n }", itemName, sObjectName)
	attrs["name"] = itemName
	attrs["TableEnumOrId"] = sObjectName
	return
}
