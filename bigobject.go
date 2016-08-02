package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"bitbucket.org/pkg/inflect"
	"github.com/heroku/force/salesforce"
	"github.com/heroku/force/util"
)

var cmdBigObject = &Command{
	Usage: "bigobject",
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


Examples:

  force bigobject list

  force bigobject create -n=MyObject -l="My Object" -p="My Objects" \
  -f=name:Field1+label:"Field 1"+type:Text+length:120 \
  -f=name:MyDate+type=dateTime

`,
}

type boField []string

func (i *boField) String() string {
	return fmt.Sprint(*i)
}

func (i *boField) Set(value string) error {
	// That would permit usages such as
	//	-deltaT 10s -deltaT 15s
	for _, name := range strings.Split(value, ",") {
		*i = append(*i, name)
	}
	return nil
}

var (
	fields           boField
	deploymentStatus string
	objectLabel      string
	pluralLabel      string
)

func init() {
	cmdBigObject.Flag.Var(&fields, "field", "names of metadata")
	cmdBigObject.Flag.Var(&fields, "f", "names of metadata")
	cmdBigObject.Flag.StringVar(&deploymentStatus, "deployment", "Deployed", "deployment status")
	cmdBigObject.Flag.StringVar(&deploymentStatus, "d", "Deployed", "deployment status")
	cmdBigObject.Flag.StringVar(&objectLabel, "label", "", "big object label")
	cmdBigObject.Flag.StringVar(&objectLabel, "l", "", "big object label")
	cmdBigObject.Flag.StringVar(&pluralLabel, "plural", "", "big object plural label")
	cmdBigObject.Flag.StringVar(&pluralLabel, "p", "", "big object plural label")
	cmdBigObject.Run = runBigObject
}

func runBigObject(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
	} else {
		if err := cmd.Flag.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		switch args[0] {
		case "list":
			getBigObjectList(args[1:])
		case "create":
			runBigObjectCreate(args[1:])
		default:
			util.ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func parseField(fielddata string) (result salesforce.BigObjectField) {
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
				util.ErrorAndExit(err.Error())
			}
			result.Length = int(lval)
		}
	}
	result = validateField(result)
	return
}

func validateField(originField salesforce.BigObjectField) (field salesforce.BigObjectField) {
	field = originField
	if len(field.Type) == 0 {
		util.ErrorAndExit("You need to indicate the type for field %s", field.FullName)
	}
	if len(field.Label) == 0 {
		field.Label = field.FullName
	}
	switch strings.ToLower(field.Type) {
	case "text":
		if field.Length == 0 {
			util.ErrorAndExit("The text field %s is missing the length attribute.", field.FullName)
		}
		field.ReferenceTo = ""
		field.RelationshipName = ""
	case "lookup":
		if len(field.ReferenceTo) == 0 {
			util.ErrorAndExit("The lookup field %s is missing the referenceTo attribute.", field.FullName)
		}
		if len(field.RelationshipName) == 0 {
			util.ErrorAndExit("The lookup field %s is missing the relationshipName attribute.")
		}
	case "datetime":
		field.ReferenceTo = ""
		field.RelationshipName = ""
		field.Length = 0
	default:
		util.ErrorAndExit("%s is not a valid field type.\nValid field types are 'text', 'dateTime' and 'lookup'", field.Type)
	}
	return
}

func getBigObjectList(args []string) (l []salesforce.ForceSobject) {
	force, _ := ActiveForce()
	sobjects, err := force.ListSobjects()
	if err != nil {
		util.ErrorAndExit(fmt.Sprintf("ERROR: %s\n", err))
	}

	for _, sobject := range sobjects {
		if len(args) == 1 {
			if strings.Contains(sobject["name"].(string), args[0]) {
				l = append(l, sobject)
			}
		} else {
			l = append(l, sobject)
		}
	}
	return
}

func runBigObjectCreate(args []string) {
	var fieldObjects = make([]salesforce.BigObjectField, len(fields))
	for i, field := range fields {
		fieldObjects[i] = parseField(field)
	}

	var object = salesforce.BigObject{deploymentStatus, objectLabel, pluralLabel, fieldObjects}
	if len(object.Label) == 0 {
		util.ErrorAndExit("Please provide a label for your big object using the -l flag.")
	}
	if len(object.PluralLabel) == 0 {
		object.PluralLabel = inflect.Pluralize(object.Label)
	}
	force, _ := ActiveForce()
	if err := force.Metadata.CreateBigObject(object); err != nil {
		util.ErrorAndExit(err.Error())
	}
	fmt.Println("Big object created")

}
