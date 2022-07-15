package command

import (
	"fmt"
	"strings"

	. "github.com/ForceCLI/force/error"
	"github.com/spf13/cobra"
)

func init() {
	createApexClassCmd.Flags().StringP("name", "n", "", "Name of Apex Class")
	createApexTriggerCmd.Flags().StringP("name", "n", "", "Name of Apex Trigger")
	createApexComponentCmd.Flags().StringP("name", "n", "", "Name of Visualforce Component")
	createApexPageCmd.Flags().StringP("name", "n", "", "Name of Visualforce Page")
	createApexTriggerCmd.Flags().StringP("sobject", "s", "", "For which sobject should the trigger be created")

	createApexClassCmd.MarkFlagRequired("name")
	createApexTriggerCmd.MarkFlagRequired("name")
	createApexTriggerCmd.MarkFlagRequired("sobject")
	createApexComponentCmd.MarkFlagRequired("name")
	createApexPageCmd.MarkFlagRequired("name")
	createCmd.AddCommand(createApexClassCmd)
	createCmd.AddCommand(createApexTriggerCmd)
	createCmd.AddCommand(createApexComponentCmd)
	createCmd.AddCommand(createApexPageCmd)
	RootCmd.AddCommand(createCmd)
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates a new, empty Apex Class, Trigger, Visualforce page, or Component.",
	Args:  cobra.MaximumNArgs(0),
}

var createApexClassCmd = &cobra.Command{
	Use:   "apexclass",
	Short: "Create an Apex Class",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		itemName, _ := cmd.Flags().GetString("name")
		runCreate("apexclass", itemName, "")
	},
}

var createApexTriggerCmd = &cobra.Command{
	Use:   "apextrigger",
	Short: "Create an Apex Trigger",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		itemName, _ := cmd.Flags().GetString("name")
		sobjectName, _ := cmd.Flags().GetString("sobject")
		runCreate("apextrigger", itemName, sobjectName)
	},
}

var createApexComponentCmd = &cobra.Command{
	Use:   "apexcomponent",
	Short: "Create a Visualforce Component",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		itemName, _ := cmd.Flags().GetString("name")
		runCreate("apexcomponent", itemName, "")
	},
}

var createApexPageCmd = &cobra.Command{
	Use:   "apexpage",
	Short: "Create a Visualforce Page",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		itemName, _ := cmd.Flags().GetString("name")
		runCreate("apexpage", itemName, "")
	},
}

func runCreate(what, itemName, sobjectName string) {
	attrs := make(map[string]string)
	switch strings.ToLower(what) {
	case "apexclass":
		attrs = getApexDefinition(itemName)
	case "apextrigger":
		attrs = getTriggerDefinition(itemName, sobjectName)
	case "apexcomponent":
		attrs = getVFComponentDefinition(itemName)
	case "apexpage":
		attrs = getVFDefinition(itemName)
	}

	_, err := force.CreateToolingRecord(what, attrs)
	if err != nil {
		ErrorAndExit(fmt.Sprintf("Failed to create %s %s: %s", itemName, what, err.Error()))
	} else {
		fmt.Printf("Created new %s named %s.\n", what, itemName)
	}
}

func getVFDefinition(itemName string) (attrs map[string]string) {
	attrs = make(map[string]string)
	attrs["markup"] = "<apex:page>\n\n</apex:page>"
	attrs["name"] = itemName
	attrs["masterlabel"] = strings.Replace(itemName, " ", "_", -1)
	return
}

func getVFComponentDefinition(itemName string) (attrs map[string]string) {
	attrs = make(map[string]string)
	attrs["markup"] = "<apex:component>\n\n</apex:component>"
	attrs["name"] = itemName
	attrs["masterlabel"] = strings.Replace(itemName, " ", "_", -1)
	return
}

func getApexDefinition(itemName string) (attrs map[string]string) {
	attrs = make(map[string]string)
	attrs["status"] = "Active"
	attrs["body"] = fmt.Sprintf("public with sharing class %s {\n\n}", itemName)
	attrs["name"] = itemName
	return
}

func getTriggerDefinition(itemName, sObjectName string) (attrs map[string]string) {
	attrs = make(map[string]string)
	attrs["status"] = "Active"
	attrs["body"] = fmt.Sprintf("trigger %s on %s (before insert, after insert, before update, after update, before delete, after delete, after undelete) { \n\n }", itemName, sObjectName)
	attrs["name"] = itemName
	attrs["TableEnumOrId"] = sObjectName
	return
}
