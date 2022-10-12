package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ForceCLI/inflect"
	"github.com/spf13/cobra"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

func init() {
	bigObjectCreateCmd.Flags().StringSliceP("field", "f", []string{}, "field definition")
	bigObjectCreateCmd.Flags().StringP("label", "l", "", "big object label")
	bigObjectCreateCmd.Flags().StringP("plural", "p", "", "big object plural label")
	bigObjectCreateCmd.MarkFlagRequired("label")

	bigObjectCmd.AddCommand(bigObjectListCmd)
	bigObjectCmd.AddCommand(bigObjectCreateCmd)
	RootCmd.AddCommand(bigObjectCmd)
}

var bigObjectListCmd = &cobra.Command{
	Use:                   "list [object]",
	Short:                 "List big objects",
	Args:                  cobra.MaximumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		object := ""
		if len(args) > 0 {
			object = args[0]
		}
		getBigObjectList(object)
	},
}

var bigObjectCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create big object",
	Args:  cobra.MaximumNArgs(0),
	Example: `
  force bigobject create -n=MyObject -l="My Object" -p="My Objects" \
  -f=name:Field1+label:"Field 1"+type:Text+length:120 \
  -f=name:MyDate+type=dateTime
`,
	Run: func(cmd *cobra.Command, args []string) {
		fields, _ := cmd.Flags().GetStringSlice("field")
		label, _ := cmd.Flags().GetString("label")
		plural, _ := cmd.Flags().GetString("plural")
		runBigObjectCreate(fields, label, plural)
	},
}

var bigObjectCmd = &cobra.Command{
	Use:   "bigobject",
	Short: "Manage big objects",
	Long: `
Manage big objects

Usage:

  force bigobject list

  force bigobject create -n=<name> [-f=<field> ...]
  		A field is defined as a "+" separated list of attributes
  		Attributes depend on the type of the field.

  		Type = text: name, label, length
  		Type = datetime: name, label
  		Type = lookup: name, label, referenceTo, relationshipName
`,

	Example: `
  force bigobject list

  force bigobject create -n=MyObject -l="My Object" -p="My Objects" \
  -f=name:Field1+label:"Field 1"+type:Text+length:120 \
  -f=name:MyDate+type=dateTime

`,
	Args: cobra.MaximumNArgs(0),
}

func parseField(fielddata string) (result BigObjectField) {
	attrs := strings.Split(fielddata, "+")
	for _, data := range attrs {
		pair := strings.Split(data, ":")
		switch strings.ToLower(pair[0]) {
		case "fullname", "name":
			result.FullName = pair[1]
		case "label":
			result.Label = pair[1]
		case "type":
			result.Type = pair[1]
		case "referenceto":
			result.ReferenceTo = pair[1]
		case "relationshipname":
			result.RelationshipName = pair[1]
		case "length":
			var lval int64
			lval, err := strconv.ParseInt(pair[1], 10, 0)
			if err != nil {
				ErrorAndExit(err.Error())
			}
			result.Length = int(lval)
		}
	}
	result = validateField(result)
	return
}

func validateField(originField BigObjectField) (field BigObjectField) {
	field = originField
	if len(field.Type) == 0 {
		ErrorAndExit("You need to indicate the type for field %s", field.FullName)
	}
	if len(field.Label) == 0 {
		field.Label = field.FullName
	}
	switch strings.ToLower(field.Type) {
	case "text":
		if field.Length == 0 {
			ErrorAndExit("The text field %s is missing the length attribute.", field.FullName)
		}
		field.ReferenceTo = ""
		field.RelationshipName = ""
	case "lookup":
		if len(field.ReferenceTo) == 0 {
			ErrorAndExit("The lookup field %s is missing the referenceTo attribute.", field.FullName)
		}
		if len(field.RelationshipName) == 0 {
			ErrorAndExit("The lookup field %s is missing the relationshipName attribute.")
		}
	case "datetime":
		field.ReferenceTo = ""
		field.RelationshipName = ""
		field.Length = 0
	default:
		ErrorAndExit("%s is not a valid field type.\nValid field types are 'text', 'dateTime' and 'lookup'", field.Type)
	}
	return
}

func getBigObjectList(object string) (l []ForceSobject) {
	sobjects, err := force.ListSobjects()
	if err != nil {
		ErrorAndExit(fmt.Sprintf("ERROR: %s\n", err))
	}

	for _, sobject := range sobjects {
		if len(object) > 0 {
			if strings.Contains(strings.ToLower(sobject["name"].(string)), strings.ToLower(object)) {
				l = append(l, sobject)
			}
		} else {
			l = append(l, sobject)
		}
	}
	return
}

func runBigObjectCreate(fields []string, label, plural string) {
	var fieldObjects = make([]BigObjectField, len(fields))
	for i, field := range fields {
		fieldObjects[i] = parseField(field)
	}

	var object = BigObject{
		DeploymentStatus: "Deployed",
		Label:            label,
		PluralLabel:      plural,
		Fields:           fieldObjects,
	}
	if len(object.PluralLabel) == 0 {
		object.PluralLabel = inflect.Pluralize(object.Label)
	}
	if err := force.Metadata.CreateBigObject(object); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Big object created")
}
