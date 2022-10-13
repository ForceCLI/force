package command

import (
	"encoding/xml"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	describeMetadataCmd.Flags().StringP("type", "t", "", "type of metadata")
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
	Long: `List the metadata in the org.  With no type specified, lists all
metadata types supported.  Specifying a type will list the individual metadata
components of that type.
`,
	Example: `
  force describe metadata
  force describe metadata -t MatchingRule -j
  `,
	Args: cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		metadataType, _ := cmd.Flags().GetString("type")
		json, _ := cmd.Flags().GetBool("json")
		describeMetadata(metadataType, json)
	},
}

var describeSobjectCmd = &cobra.Command{
	Use:   "sobject",
	Short: "List sobjects",
	Long: `With no name specified, list all SObjects in the org.  Specifying an
object name will retrieve all of the details about the object.
`,
	Args: cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		item, _ := cmd.Flags().GetString("name")
		json, _ := cmd.Flags().GetBool("json")
		describeSObject(item, json)
	},
}

var describeCmd = &cobra.Command{
	Use:   "describe (metadata|sobject) [flags]",
	Short: "Describe the types of metadata available in the org",
	Example: `
  force describe metadata
  force describe metadata -t MatchingRule -j
  force describe sobject -n Account
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
