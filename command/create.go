package command

import (
	"fmt"
	"strings"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdCreate = &Command{
	Usage: "create what=<ApexClass, ApexPage> name=<item name>",
	Short: "Creates a new, empty Apex Class or Visualforce page.",
	Long: `
Creates a new, empty Apex Class or Visualforce page.

Examples (both flags are required):

  force create -w ApexClass -n NewController

  force create -w ApexTrigger -n NewTrigger -s Account

  force create -w ApexPage -n CoolPage

  force create -w ApexComponent -n CoolComponent
`,
}
var (
	what     string
	sObjectName  string
	itemName string
)

func init() {
	cmdCreate.Flag.StringVar(&what, "what", "", "What type of thing to create (currently only Apex or Visualforce).")
	cmdCreate.Flag.StringVar(&what, "w", "", "What type of thing to create (currently only Apex or Visualforce).")
	cmdCreate.Flag.StringVar(&what, "type", "", "What type of thing to create (currently only Apex or Visualforce).") // Because consistency with other commands like fetch
	cmdCreate.Flag.StringVar(&what, "t", "", "What type of thing to create (currently only Apex or Visualforce).")
	cmdCreate.Flag.StringVar(&sObjectName, "sobject", "", "For which sobject should the thing be created (only for Triggers).")
	cmdCreate.Flag.StringVar(&sObjectName, "s", "", "For which sobject should the thing be created (only for Triggers).")
	cmdCreate.Flag.StringVar(&itemName, "n", "", "Name of thing to be created.")
	cmdCreate.Flag.StringVar(&itemName, "name", "", "Name of thing to be created.")
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

		result, err := force.CreateToolingRecord(what, attrs)
		fmt.Println(result)
		if err != nil {
			ErrorAndExit(err.Error())
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
