package command

import (
	"fmt"
	"strings"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	fieldCmd.AddCommand(fieldListCmd)
	fieldCmd.AddCommand(fieldCreateCmd)
	fieldCmd.AddCommand(fieldDeleteCmd)
	fieldCmd.AddCommand(fieldTypeCmd)
	RootCmd.AddCommand(fieldCmd)
}

var fieldCmd = &cobra.Command{
	Use:   "field",
	Short: "Manage SObject fields",
	Long: `
Manage SObject fields

Usage:

  force field list <object>
  force field create <object> <field>:<type> [<option>:<value>]
  force field delete <object> <field>
  force field type
  force field type <fieldtype>
  `,

	Example: `
  force field list Todo__c
  force field create Inspection__c "Final Outcome":picklist picklist:"Pass, Fail, Redo"
  force field create Todo__c Due:DateTime required:true
  force field delete Todo__c Due
  force field type     # displays all the supported field types
  force field type email   # displays the required and optional attributes
`,
	DisableFlagsInUseLine: true,
}

var fieldListCmd = &cobra.Command{
	Use:                   "list <object>",
	Short:                 "List SObject fields",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		runFieldList(args[0])
	},
}

var fieldCreateCmd = &cobra.Command{
	Use:   "create <object> <field>:<type> [<option>:<value>]",
	Short: "Create SObject fields",
	Example: `
  force field create Inspection__c "Final Outcome":picklist picklist:"Pass, Fail, Redo"
  force field create Todo__c Due:DateTime required:true
`,
	Args:                  cobra.MinimumNArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		runFieldCreate(args)
	},
}

var fieldDeleteCmd = &cobra.Command{
	Use:                   "delete <object> <field>",
	Short:                 "Delete SObject field",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		runFieldDelete(args[0], args[1])
	},
}

var fieldTypeCmd = &cobra.Command{
	Use:                   "type [field type]",
	Short:                 "Display SObject field type details",
	Args:                  cobra.MaximumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			DisplayFieldTypes()
		} else {
			DisplayFieldDetails(args[0])
		}
	},
}

func runFieldList(object string) {
	sobject, err := force.GetSobject(object)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	DisplayForceSobject(sobject)
}

func runFieldCreate(args []string) {
	parts := strings.Split(args[1], ":")
	if len(parts) != 2 {
		ErrorAndExit("must specify name:type for fields")
	}

	var optionMap = make(map[string]string)
	if len(args) > 2 {
		for _, value := range args[2:] {
			options := strings.Split(value, ":")
			if len(options) != 2 {
				ErrorAndExit(fmt.Sprintf("Missing value for field attribute %s", value))
			}
			optionMap[options[0]] = options[1]
		}
	}

	// Validate the options for this field type
	newOptions, err := force.Metadata.ValidateFieldOptions(parts[1], optionMap)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if err := force.Metadata.CreateCustomField(args[0], parts[0], parts[1], newOptions); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Custom field created")
}

func runFieldDelete(object, field string) {
	if err := force.Metadata.DeleteCustomField(object, field); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Custom field deleted")
}
