package command

import (
	"encoding/xml"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	describeMetadataCmd.Flags().StringP("name", "n", "", "name of metadata")
	describeMetadataCmd.Flags().BoolP("json", "j", false, "json output")
	describeSobjectCmd.Flags().StringP("name", "n", "", "name of sobject")
	describeSobjectCmd.Flags().BoolP("json", "j", false, "json output")

	describeCmd.AddCommand(describeMetadataCmd)
	describeCmd.AddCommand(describeSobjectCmd)
	RootCmd.AddCommand(describeCmd)
}

var describeMetadataCmd = &cobra.Command{
	Use:   "metadata",
	Short: "Describe metadata",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		item, _ := cmd.Flags().GetString("name")
		json, _ := cmd.Flags().GetBool("json")
		describeMetadata(item, json)
	},
}

var describeSobjectCmd = &cobra.Command{
	Use:   "sobject",
	Short: "Describe sobject",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		item, _ := cmd.Flags().GetString("name")
		json, _ := cmd.Flags().GetBool("json")
		describeSObject(item, json)
	},
}

var describeCmd = &cobra.Command{
	Use:   "describe (metadata|sobject) [flags]",
	Short: "Describe the object or list of available objects",
	Example: `
  force describe metadata -n=CustomObject
  force describe sobject -n=Account
  `,
	Args: cobra.ExactArgs(0),
}

func describeMetadata(item string, json bool) {
	if len(item) == 0 {
		// List all metadata
		describe, err := force.Metadata.DescribeMetadata()
		if err != nil {
			ErrorAndExit(err.Error())
		}
		if json {
			DisplayMetadataListJson(describe.MetadataObjects)
		} else {
			DisplayMetadataList(describe.MetadataObjects)
		}
	} else {
		// List all metdata object of metaItem type
		body, err := force.Metadata.ListMetadata(item)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		var res struct {
			Response ListMetadataResponse `xml:"Body>listMetadataResponse"`
		}
		if err = xml.Unmarshal(body, &res); err != nil {
			ErrorAndExit(err.Error())
		}
		if json {
			DisplayListMetadataResponseJson(res.Response)
		} else {
			DisplayListMetadataResponse(res.Response)
		}
	}
}
func describeSObject(item string, json bool) {
	if len(item) == 0 {
		// list all sobject
		if json {
			l := getSobjectList("")
			DisplayForceSobjectsJson(l)
		} else {
			runSobjectList("")
		}
	} else {
		// describe sobject
		desc, err := force.DescribeSObject(item)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		DisplayForceSobjectDescribe(desc)
	}
}
