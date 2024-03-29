package command

import (
	"container/list"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ForceCLI/force/desktop"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

////////////////////////////////////////////////////////////////////////
// Parse the permissions for a given profile and return a Profile struct
////////////////////////////////////////////////////////////////////////

func init() {
	RootCmd.AddCommand(securityCmd)
}

var securityCmd = &cobra.Command{
	Use:   "security [SObject]",
	Short: "Displays the OLS and FLS for a given SObject",
	Example: `
  force security Case
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runSecurity(args[0])
	},
}

type XLS interface {
	addProperty(name string, value string)
	addToProfile(p Profile)
}

type OLS struct {
	objectName       string
	allowCreate      string
	allowDelete      string
	allowEdit        string
	allowRead        string
	modifyAllRecords string
	viewAllRecords   string
}

func (o *OLS) addProperty(name string, value string) {
	switch name {
	case "object":
		o.objectName = value
	case "allowCreate":
		o.allowCreate = value
	case "allowDelete":
		o.allowDelete = value
	case "allowEdit":
		o.allowEdit = value
	case "allowRead":
		o.allowRead = value
	case "modifyAllRecords":
		o.modifyAllRecords = value
	case "viewAllRecords":
		o.viewAllRecords = value
	}
}
func (o *OLS) getProperty(name string) string {
	switch name {
	case "Allow Create":
		return o.allowCreate
	case "Allow Delete":
		return o.allowDelete
	case "Allow Edit":
		return o.allowEdit
	case "Allow Read":
		return o.allowRead
	case "Modify All Records":
		return o.modifyAllRecords
	case "View All Records":
		return o.viewAllRecords
	}
	return ""
}
func (o *OLS) addToProfile(p Profile) {
	p.objectPermissions[o.objectName] = *o
}

type FLS struct {
	field    string
	editable string
	readable string
}

func (f *FLS) addProperty(name string, value string) {
	switch name {
	case "field":
		f.field = value
	case "editable":
		f.editable = value
	case "readable":
		f.readable = value
	}
	//	fmt.Println("Field Property " + name + "=" + value)
}
func (f *FLS) addToProfile(p Profile) {
	p.fieldPermissions[f.field] = *f
}

type Profile struct {
	name              string
	fieldPermissions  map[string]FLS
	objectPermissions map[string]OLS
}

func parseProfileXML(profileName string, text string) Profile {
	p := new(Profile)
	p.name = profileName
	p.fieldPermissions = map[string]FLS{}
	p.objectPermissions = map[string]OLS{}
	var currentElement XLS

	r := strings.NewReader(text)
	parser := xml.NewDecoder(r)
	depth := 0

	eltType := ""
	propertyName := ""

	for {

		token, err := parser.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			elmt := xml.StartElement(t)
			name := elmt.Name.Local
			if depth == 1 {
				eltType = name
				if eltType == "objectPermissions" {
					currentElement = new(OLS)
				} else if eltType == "fieldPermissions" {
					currentElement = new(FLS)
				} else {
					currentElement = nil
				}
			}
			if depth == 2 {
				propertyName = name
			}
			depth++
		case xml.EndElement:
			if depth == 2 && currentElement != nil {
				currentElement.addToProfile(*p)
			}
			depth--
		case xml.CharData:
			bytes := xml.CharData(t)
			if currentElement != nil && depth == 3 {
				currentElement.addProperty(propertyName, string(bytes))
			}
		default:
		}
	}

	//	fmt.Println(p)

	return *p
}

//////////////////////////////////////////////////////////////////////
// Read information about an SObject and returns a CustomObject struct
//////////////////////////////////////////////////////////////////////

type CustomObject struct {
	objectName string
	fieldNames []string
	nbFields   int
}

func (co *CustomObject) addField(name string) {
	co.fieldNames[co.nbFields] = name
	co.nbFields++
}
func (co *CustomObject) getProfileFootprint(p Profile) string {
	key := "OLS:" + p.objectPermissions[co.objectName].allowCreate + "," +
		p.objectPermissions[co.objectName].allowDelete + "," +
		p.objectPermissions[co.objectName].allowEdit + "," +
		p.objectPermissions[co.objectName].allowRead + "," +
		p.objectPermissions[co.objectName].modifyAllRecords + "," +
		p.objectPermissions[co.objectName].viewAllRecords + ","

	for idx := 0; idx < co.nbFields; idx++ {
		f := co.fieldNames[idx]
		key += f + ":" + p.fieldPermissions[co.objectName+"."+f].editable + "," +
			p.fieldPermissions[co.objectName+"."+f].readable + ","
	}
	return key
}

func parseCustomObjectXML(objectName string, text string) CustomObject {
	obj := CustomObject{objectName: objectName, nbFields: 0, fieldNames: make([]string, 900, 900)}
	r := strings.NewReader(text)
	parser := xml.NewDecoder(r)
	depth := 0
	var firstLevel, secondLevel string

	for {

		token, err := parser.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			elmt := xml.StartElement(t)
			name := elmt.Name.Local
			if depth == 1 {
				firstLevel = name
			} else if depth == 2 {
				secondLevel = name
			}
			depth++
		case xml.EndElement:
			if depth == 3 {
				secondLevel = ""
			} else if depth == 2 {
				firstLevel = ""
			}
			depth--
		case xml.CharData:
			bytes := xml.CharData(t)
			if depth == 3 && firstLevel == "fields" && secondLevel == "fullName" {
				obj.addField(string(bytes))
			}
		default:
		}
	}

	//	fmt.Println(obj)
	return obj
}

/////////////////////////////////////////////////////////

func runSecurity(sobject string) {
	wd, _ := os.Getwd()
	root := filepath.Join(wd, ".")

	query := ForceMetadataQuery{
		{Name: []string{"Profile"}, Members: []string{"*"}},
		{Name: []string{"CustomObject"}, Members: []string{sobject}},
	}

	// Step 1: retrieve the desired metadata
	files, problems, err := force.Metadata.Retrieve(query)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	for _, problem := range problems {
		fmt.Fprintln(os.Stderr, problem)
	}

	// Step 2: go through the metadata and construct a list of Profile (profiles) and a CustomObject (theObject)
	var profiles list.List
	var theObject CustomObject

	for name, data := range files {
		if strings.HasPrefix(name, "profiles/") {
			profileName := strings.TrimSuffix(strings.TrimPrefix(name, "profiles/"), ".profile")
			profiles.PushBack(parseProfileXML(profileName, string(data)))
		} else if strings.HasPrefix(name, "objects/") {
			objectName := strings.TrimSuffix(strings.TrimPrefix(name, "objects/"), ".object")
			if strings.ToLower(objectName) == strings.ToLower(sobject) {
				theObject = parseCustomObjectXML(objectName, string(data))
			}
		}
	}

	// Step 3: group the profiles that have the exact same OLS and FLS
	// for the desired object together
	allProfiles := map[string]list.List{}
	var p Profile
	var profileKeys list.List

	for e := profiles.Front(); e != nil; e = e.Next() {
		p = e.Value.(Profile)
		key := theObject.getProfileFootprint(p)
		tmpList, OK := allProfiles[key]
		if !OK {
			var tmpList2 list.List
			tmpList2.PushBack(p)
			allProfiles[key] = tmpList2
			profileKeys.PushBack(key)
		} else {
			tmpList.PushBack(p)
		}
	}

	// Step 4: generate an HTML file that shows the various groups of profiles
	// as well as their OLS and FLS
	HTMLoutput := "<html><body>" +
		"<table border=\"1\" style=\"border-collapse:collapse;\">" +
		"<tr><td></td>"

	for key := profileKeys.Front(); key != nil; key = key.Next() {
		val := allProfiles[key.Value.(string)]
		profileNames := ""
		for v := val.Front(); v != nil; v = v.Next() {
			if v.Value == nil {
				continue
			}
			theProfile := v.Value.(Profile)
			profileNames += theProfile.name + "<br/>"
		}
		HTMLoutput += "<td>" + strings.Replace(profileNames, " ", "&nbsp;", -1) + "</td>"
	}
	HTMLoutput += "</tr>"

	OLSproperties := []string{"Allow Create", "Allow Delete", "Allow Edit", "Allow Read", "Modify All Records", "View All Records"}

	for _, OLSproperty := range OLSproperties {
		HTMLoutput += "<tr><td>[Object] " + OLSproperty + "</td>"

		for key := profileKeys.Front(); key != nil; key = key.Next() {
			val := allProfiles[key.Value.(string)]
			theProfile := val.Front().Value.(Profile)
			theOLS := theProfile.objectPermissions[theObject.objectName]
			HTMLoutput += "<td>" + theOLS.getProperty(OLSproperty) + "</td>"
		}
		HTMLoutput += "</tr>"
	}

	for idx := 0; idx < theObject.nbFields; idx++ {
		fieldName := theObject.fieldNames[idx]
		HTMLoutput += "<tr><td>" + fieldName + "</td>"
		for key := profileKeys.Front(); key != nil; key = key.Next() {
			val := allProfiles[key.Value.(string)]
			theProfile := val.Front().Value.(Profile)
			if theProfile.fieldPermissions[theObject.objectName+"."+fieldName].editable == "true" {
				HTMLoutput += "<td>Yes</td>"
			} else if theProfile.fieldPermissions[theObject.objectName+"."+fieldName].readable == "true" {
				HTMLoutput += "<td>Readonly</td>"
			} else {
				HTMLoutput += "<td>-</td>"
			}
		}
		HTMLoutput += "</tr>"
	}

	HTMLoutput += "</table></body></html>"

	// Last step: write the file on disk and display it inside a Web browser
	if err := ioutil.WriteFile(filepath.Join(root, "security.html"), []byte(HTMLoutput), 0644); err != nil {
		ErrorAndExit(err.Error())
	}

	desktop.Open(filepath.Join(root, "security.html"))
}
