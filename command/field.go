package command

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"

	fg "github.com/ForceCLI/force-md/general"
	"github.com/ForceCLI/force-md/permissionset"
	"github.com/ForceCLI/force-md/profile"
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
	Long: `Create SObject fields with various types and options.

Supported field options include:
  required:true/false    - Set field as required
  unique:true/false      - Set field as unique
  externalId:true/false  - Set field as external ID
  helpText:"text"        - Add inline help text for the field
  defaultValue:"value"   - Set default value
  picklist:"val1,val2"   - Define picklist values
  length:number          - Set text field length
  precision:number       - Set number precision
  scale:number           - Set number scale`,
	Example: `
  force field create Inspection__c "Final Outcome":picklist picklist:"Pass, Fail, Redo"
  force field create Todo__c Due:DateTime required:true
  force field create Account TestAuto:autoNumber helpText:"This field auto-generates unique numbers"
  force field create Contact Phone:phone helpText:"Primary contact phone number"
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
	if err := updateFLSOnProfile(force, args[0], parts[0]); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Custom field created")
}

func updateFLSOnProfile(force *Force, objectName string, fieldName string) (err error) {
	res, err := force.QueryProfile("Id", "Name", "FullName")
	profileFullName := fmt.Sprintf("%s", res.Records[0]["FullName"])

	soap := getFLSUpdateXML(objectName, fieldName)
	filename := fmt.Sprintf("%s.profile", profileFullName)
	// Create temp file and store the XML for the MD in the file
	wd, _ := os.Getwd()
	mpath, err := FindMetapathForFile(filename)
	if err != nil {
		return err
	}

	tempdir, err := ioutil.TempDir(wd, "md_temp")
	if err != nil {
		return err
	}

	profileDir := filepath.Join(tempdir, mpath)

	err = os.MkdirAll(profileDir, 0777)
	if err != nil {
		return fmt.Errorf("Could not create temporary directory %s: %w", profileDir, err)
	}
	defer os.RemoveAll(tempdir)
	xmlfile := filepath.Join(profileDir, filename)
	ioutil.WriteFile(xmlfile, []byte(soap), 0777)
	pb := NewPushBuilder()
	pb.Root = tempdir
	err = pb.AddFile(xmlfile)
	if err != nil {
		return fmt.Errorf("Could not add profile: %w", err)
	}
	displayOptions := defaultDeployOutputOptions()
	displayOptions.quiet = true
	return deploy(force, pb.ForceMetadataFiles(), new(ForceDeployOptions), displayOptions)
}

func getFLSUpdateXML(objectName string, fieldName string) string {
	if !strings.HasSuffix(fieldName, "__c") {
		fieldName = fieldName + "__c"
	}

	p := profile.Profile{}
	p.AddObjectPermissions(objectName)
	op := permissionset.ObjectPermissions{
		Object:           objectName,
		AllowCreate:      fg.TrueText,
		AllowDelete:      fg.TrueText,
		AllowEdit:        fg.TrueText,
		AllowRead:        fg.TrueText,
		ModifyAllRecords: fg.FalseText,
		ViewAllRecords:   fg.FalseText,
	}
	p.SetObjectPermissions(objectName, op)

	fqfn := objectName + "." + fieldName
	p.AddFieldPermissions(fqfn)
	fp := permissionset.FieldPermissions{
		Field:    fqfn,
		Editable: fg.TrueText,
		Readable: fg.TrueText,
	}
	p.SetFieldPermissions(fqfn, fp)

	const declaration = `<?xml version="1.0" encoding="UTF-8"?>`
	b, err := xml.Marshal(p)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	return fmt.Sprintf("%s\n%s", declaration, string(b))
}

func runFieldDelete(object, field string) {
	if err := force.Metadata.DeleteCustomField(object, field); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Custom field deleted")
}
