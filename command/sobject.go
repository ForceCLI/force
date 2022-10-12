package command

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io/ioutil"
	"os"
	"strings"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	sobjectCmd.AddCommand(sobjectListCmd)
	sobjectCmd.AddCommand(sobjectCreateCmd)
	sobjectCmd.AddCommand(sobjectDeleteCmd)
	sobjectCmd.AddCommand(sobjectImportCmd)

	RootCmd.AddCommand(sobjectCmd)
}

var sobjectListCmd = &cobra.Command{
	Use:                   "list [name]",
	Short:                 "List standard and custom objects",
	DisableFlagsInUseLine: true,
	Args:                  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		object := ""
		if len(args) > 0 {
			object = args[0]
		}
		runSobjectList(object)
	},
}

var sobjectCreateCmd = &cobra.Command{
	Use:                   "create <object> [<field>:<type> [<option>:<value>]]",
	Short:                 "Create custom object",
	DisableFlagsInUseLine: true,
	Args:                  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runSobjectCreate(args)
	},
}

var sobjectDeleteCmd = &cobra.Command{
	Use:                   "Delete <object>",
	Short:                 "Delete custom object",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runSobjectDelete(args[0])
	},
}

var sobjectImportCmd = &cobra.Command{
	Use:                   "import",
	Short:                 "Import custom object",
	Long:                  "Create a custom object with custom fields from a query result on stdin",
	DisableFlagsInUseLine: true,
	Args:                  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		runSobjectImport()
	},
}

var sobjectCmd = &cobra.Command{
	Use:   "sobject",
	Short: "Manage standard & custom objects",
	Long: `
Manage sobjects

Usage:

  force sobject list
  force sobject create <object> [<field>:<type> [<option>:<value>]]
  force sobject delete <object>
  force sobject import
`,
	Example: `
  force sobject list
  force sobject create Todo Description:string
  force sobject delete Todo
`,
	DisableFlagsInUseLine: true,
	Args:                  cobra.MaximumNArgs(0),
}

func getSobjectList(object string) (l []ForceSobject) {
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

func runSobjectList(object string) {
	l := getSobjectList(object)
	DisplayForceSobjects(l)
}

func runSobjectCreate(args []string) {
	if err := force.Metadata.CreateCustomObject(args[0]); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Custom object created")

	if len(args) > 1 {
		args[0] = fmt.Sprintf("%s__c", args[0])
		runFieldCreate(args)
	}
}

func runSobjectDelete(object string) {
	if err := force.Metadata.DeleteCustomObject(object); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Custom object deleted")
}

func runSobjectImport() {
	var objectDef = `
<cmd:sObjects>
	<cmd:type>%s</cmd:type>
%s</cmd:sObjects>`

	// Need to read the file into a query result structure
	data, err := ioutil.ReadAll(os.Stdin)

	var query ForceQueryResult
	json.Unmarshal(data, &query)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	var soapMsg = ""
	var objectType = ""
	for _, record := range query.Records {
		var fields = ""
		for key, _ := range record {
			if key == "Id" {
				continue
			} else if key == "attributes" {
				x := record[key].(map[string]interface{})
				val, ok := x["type"]
				if ok {
					objectType, ok = val.(string)
				}
			} else {
				if record[key] != nil {
					val, ok := record[key].(string)
					if ok {
						fields += fmt.Sprintf("\t<%s>%s</%s>\n", key, html.EscapeString(val), key)
					} else {
						valf, ok := record[key].(float64)
						if ok {
							fields += fmt.Sprintf("\t<%s>%f</%s>\n", key, valf, key)
						} else {
							fields += fmt.Sprintf("\t<%s>%s</%s>\n", key, record[key].(string), key)
						}
					}
				}
			}
		}
		soapMsg += fmt.Sprintf(objectDef, objectType, fields)
	}

	response, err := force.Partner.SoapExecuteCore("create", soapMsg)

	type errorData struct {
		Fields     string `xml:"field"`
		Message    string `xml:"message"`
		StatusCode string `xml:"statusCode"`
	}

	type result struct {
		Id      string      `xml:"id"`
		Success bool        `xml:"success"`
		Errors  []errorData `xml:"errors"`
	}

	var xmlresponse struct {
		Results []result `xml:"Body>createResponse>result"`
	}

	xml.Unmarshal(response, &xmlresponse)

	for i, res := range xmlresponse.Results {
		if res.Success {
			fmt.Printf("%s created successfully\n", res.Id)
		} else {
			for _, e := range res.Errors {
				fmt.Printf("%s\n\t%s\n%s\n", e.StatusCode, e.Message, query.Records[i])
			}
		}
	}
}
