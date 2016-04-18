package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io/ioutil"
	"os"
	"strings"

	"github.com/heroku/force/salesforce"
	"github.com/heroku/force/util"
)

var cmdSobject = &Command{
	Run:   runSobject,
	Usage: "sobject",
	Short: "Manage standard & custom objects",
	Long: `
Manage sobjects

Usage:

  force sobject list

  force sobject create <object> [<field>:<type> [<option>:<value>]]

  force sobject delete <object>

  force sobject import

Examples:

  force sobject list

  force sobject create Todo Description:string

  force sobject delete Todo
`,
}

func runSobject(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
	} else {
		switch args[0] {
		case "list":
			runSobjectList(args[1:])
		case "create", "add":
			runSobjectCreate(args[1:])
		case "delete", "remove":
			runSobjectDelete(args[1:])
		case "import":
			runSobjectImport(args[1:])
		default:
			util.ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func getSobjectList(args []string) (l []salesforce.ForceSobject) {
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
func runSobjectList(args []string) {
	l := getSobjectList(args)
	DisplayForceSobjects(l)
}

func runSobjectCreate(args []string) {
	if len(args) < 1 {
		util.ErrorAndExit("must specify object name")
	}
	force, _ := ActiveForce()
	if err := force.Metadata.CreateCustomObject(args[0]); err != nil {
		util.ErrorAndExit(err.Error())
	}
	fmt.Println("Custom object created")

	if len(args) > 1 {
		args[0] = fmt.Sprintf("%s__c", args[0])
		runFieldCreate(args)
	}
}

func runSobjectDelete(args []string) {
	if len(args) < 1 {
		util.ErrorAndExit("must specify object")
	}
	force, _ := ActiveForce()
	if err := force.Metadata.DeleteCustomObject(args[0]); err != nil {
		util.ErrorAndExit(err.Error())
	}
	fmt.Println("Custom object deleted")
}

func runSobjectImport(args []string) {
	var objectDef = `
<cmd:sObjects>
	<cmd:type>%s</cmd:type>
%s</cmd:sObjects>`

	// Need to read the file into a query result structure
	data, err := ioutil.ReadAll(os.Stdin)

	var query salesforce.ForceQueryResult
	json.Unmarshal(data, &query)
	if err != nil {
		util.ErrorAndExit(err.Error())
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

	force, _ := ActiveForce()
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
