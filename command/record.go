package command

import (
	"fmt"
	"strings"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	recordDeleteCmd.Flags().BoolP("tooling", "t", false, "delete using object record")

	recordCmd.AddCommand(recordGetCmd)
	recordCmd.AddCommand(recordCreateCmd)
	recordCmd.AddCommand(recordUpdateCmd)
	recordCmd.AddCommand(recordUpsertCmd)
	recordCmd.AddCommand(recordDeleteCmd)
	recordCmd.AddCommand(recordMergeCmd)
	recordCmd.AddCommand(recordUndeleteCmd)
	RootCmd.AddCommand(recordCmd)
}

var recordGetCmd = &cobra.Command{
	Use:   "get <object> <id>",
	Short: "Get record details",
	Long: `
View record

Usage:

  force record get <object> <id>

  force record get <object> <extid>:<value>
`,
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		runRecordGet(args[0], args[1])
	},
}

var recordCreateCmd = &cobra.Command{
	Use:                   "create <object> [<field>:<value>...]",
	Short:                 "Create new record",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		object := args[0]
		fields := args[1:]
		runRecordCreate(object, fields)
	},
}

var recordUpdateCmd = &cobra.Command{
	Use:                   "update <object> <id> [<field>:<value>...]",
	Short:                 "Update record",
	Args:                  cobra.MinimumNArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		object := args[0]
		id := args[1]
		fields := args[2:]
		runRecordUpdate(object, id, fields)
	},
}

var recordUpsertCmd = &cobra.Command{
	Use:   "upsert <object> <extid>:<value> [<field>:<value>...]",
	Short: "Upsert record using external ID",
	Long: `
Upsert (insert or update) a record using an external ID field.

If a record with the given external ID value exists, it will be updated.
Otherwise, a new record will be created.

Usage:

  force record upsert <object> <extid>:<value> [<field>:<value>...]
`,
	Example: `
  force record upsert Account External_Id__c:ABC123 Name:"Acme Corp" Industry:Technology
  force record upsert Contact Email:john@example.com FirstName:John LastName:Doe
`,
	Args:                  cobra.MinimumNArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		object := args[0]
		extIdPair := args[1]
		fields := args[2:]
		runRecordUpsert(object, extIdPair, fields)
	},
}

var recordDeleteCmd = &cobra.Command{
	Use:                   "delete <object> <id>",
	Short:                 "Delete record",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		if tooling, _ := cmd.Flags().GetBool("tooling"); tooling {
			runToolingRecordDelete(args[0], args[1])
		} else {
			runRecordDelete(args[0], args[1])
		}
	},
}

var recordMergeCmd = &cobra.Command{
	Use:                   "merge <object> <masterId> <duplicateId>",
	Short:                 "Merge records",
	Args:                  cobra.ExactArgs(3),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		object := args[0]
		masterId := args[1]
		duplicateId := args[2]
		runRecordMerge(object, masterId, duplicateId)
	},
}

var recordUndeleteCmd = &cobra.Command{
	Use:                   "undelete <id>...",
	Short:                 "Undelete records",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		runRecordUndelete(args)
	},
}

var recordCmd = &cobra.Command{
	Use:   "record <command> [<args>]",
	Short: "Create, modify, or view records",
	Long: `
Create, modify, or view records

Usage:

  force record get <object> <id>
  force record get <object> <extid>:<value>
  force record create <object> [<fields>]
  force record update <object> <id> [<fields>]
  force record update <object> <extid>:<value> [<fields>]
  force record upsert <object> <extid>:<value> [<fields>]
  force record delete <object> <id>
  force record merge <object> <masterId> <duplicateId>
  force record undelete <id>
`,
	Example: `
  force record get User 00Ei0000000000
  force record get User username:user@name.org
  force record create User Name:"David Dollar" Phone:0000000000
  force record update User 00Ei0000000000 State:GA
  force record update User username:user@name.org State:GA
  force record upsert Account External_Id__c:ABC123 Name:"Acme Corp"
  force record delete User 00Ei0000000000
  force record merge Contact 0033c00002YDNNWAA5 0033c00002YDPqkAAH
  force record undelete 0033c00002YDNNWAA5
`,
}

func runRecordGet(sobject, id string) {
	object, err := force.GetRecord(sobject, id)
	if err != nil {
		ErrorAndExit("Failed to get record: %s", err.Error())
	} else {
		DisplayForceRecord(object)
	}
}

func runRecordCreate(object string, fields []string) {
	attrs := parseArgumentAttrs(fields)
	id, err, emessages := force.CreateRecord(object, attrs)
	if err != nil {
		ErrorAndExit("Failed to create record: %s (%s)", err.Error(), emessages[0].ErrorCode)
	}
	fmt.Printf("Record created: %s\n", id)
}

func runRecordUpdate(object string, id string, fields []string) {
	attrs := parseArgumentAttrs(fields)
	err := force.UpdateRecord(object, id, attrs)
	if err != nil {
		ErrorAndExit("Failed to update record: %s", err.Error())
	}
	fmt.Println("Record updated")
}

func runRecordUpsert(object string, extIdPair string, fields []string) {
	split := strings.SplitN(extIdPair, ":", 2)
	if len(split) != 2 {
		ErrorAndExit("Invalid external ID format. Use <extid>:<value>")
	}
	extIdField := split[0]
	extIdValue := split[1]

	attrs := parseArgumentAttrs(fields)
	result, err := force.UpsertRecord(object, extIdField, extIdValue, attrs)
	if err != nil {
		ErrorAndExit("Failed to upsert record: %s", err.Error())
	}
	if result.Created {
		fmt.Printf("Record created: %s\n", result.Id)
	} else {
		fmt.Println("Record updated")
	}
}

func runRecordMerge(object, masterId, duplicateId string) {
	err := force.Partner.Merge(object, masterId, duplicateId)
	if err != nil {
		ErrorAndExit("Failed to merge records: %s", err.Error())
	}
	fmt.Println("Records merged")
}

func runRecordUndelete(args []string) {
	res, err := force.Partner.UndeleteMany(args)
	if err != nil {
		ErrorAndExit("Failed to undelete record: %s", err.Error())
	}
	errored := false
	for _, r := range res {
		if !r.Success {
			errored = true
			fmt.Printf("Undelete failed for %s: %s\n", r.Id, r.Errors[0].Message)
		}
	}
	if !errored {
		fmt.Println("Records undeleted")
	}
}

func runRecordDelete(object, id string) {
	err := force.DeleteRecord(object, id)
	if err != nil {
		ErrorAndExit("Failed to delete record: %s", err.Error())
	}
	fmt.Println("Record deleted")
}

func runToolingRecordDelete(object, id string) {
	err := force.DeleteToolingRecord(object, id)
	if err != nil {
		ErrorAndExit("Failed to delete record: %s", err.Error())
	}
	fmt.Println("Record deleted")
}

func parseArgumentAttrs(pairs []string) (parsed map[string]string) {
	parsed = make(map[string]string)
	for _, pair := range pairs {
		split := strings.SplitN(pair, ":", 2)
		parsed[split[0]] = split[1]
	}
	return
}
