package main

import (
	"encoding/xml"

	"github.com/heroku/force/util"
	//"fmt"
	"github.com/heroku/force/salesforce"
)

var cmdDescribe = &Command{
	Usage: "describe (metadata|sobject) [-n=<name> -json]",
	Short: "Describe the object or list of available objects",
	Long: `
  -n, -name       # name of specific metadata to retrieve
  -json           # output in JSON format

  Examples

  force describe metadata -n=CustomObject
  force describe sobject -n=Account
  force describe metata
  force describe sobject
  `,
}

var (
	jsonout  bool
	metaItem string
)

func init() {
	cmdDescribe.Flag.StringVar(&metaItem, "name", "", "name of metadata")
	cmdDescribe.Flag.StringVar(&metaItem, "n", "", "name of metadata")
	cmdDescribe.Flag.StringVar(&metadataType, "t", "", "Type of metadata to describe")
	cmdDescribe.Flag.StringVar(&metadataType, "type", "", "Type of metadata to describe")
	cmdDescribe.Flag.BoolVar(&jsonout, "j", false, "Unpage any static resources")
	cmdDescribe.Flag.BoolVar(&jsonout, "json", false, "Unpage any static resources")
	cmdDescribe.Run = runDescribe
}

func runDescribe(cmd *Command, args []string) {
	if len(metadataType) == 0 {
		util.ErrorAndExit("You must specify metadata or sobject for description\nexample: force describe -t metadata")
	}
	if metadataType != "metadata" && metadataType != "sobject" {
		util.ErrorAndExit("Only metadata and sobject can be described")
	}

	force, _ := ActiveForce()

	if metadataType == "metadata" {
		if len(metaItem) == 0 {
			// List all metadata
			describe, err := force.Metadata.DescribeMetadata()
			if err != nil {
				util.ErrorAndExit(err.Error())
			}
			if jsonout {
				DisplayMetadataListJson(describe.MetadataObjects)
			} else {
				DisplayMetadataList(describe.MetadataObjects)
			}
		} else {
			// List all metdata object of metaItem type
			body, err := force.Metadata.ListMetadata(metaItem)
			if err != nil {
				util.ErrorAndExit(err.Error())
			}
			var res struct {
				Response salesforce.ListMetadataResponse `xml:"Body>listMetadataResponse"`
			}
			if err = xml.Unmarshal(body, &res); err != nil {
				util.ErrorAndExit(err.Error())
			}
			if jsonout {
				DisplayListMetadataResponseJson(res.Response)
			} else {
				DisplayListMetadataResponse(res.Response)
			}
		}
	} else {
		if len(metaItem) == 0 {
			// list all sobject
			if jsonout {
				l := getSobjectList(make([]string, 0))
				DisplayForceSobjectsJson(l)
			} else {
				runSobjectList(make([]string, 0))
			}
		} else {
			// describe sobject
			desc, err := force.DescribeSObject(metaItem)
			if err != nil {
				util.ErrorAndExit(err.Error())
			}
			DisplayForceSobjectDescribe(desc)
		}
	}
}
