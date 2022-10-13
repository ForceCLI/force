package command

import (
	"encoding/json"
	"fmt"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	apiVersionListCmd.Flags().BoolP("json", "j", false, "json output")
	apiVersionCmd.AddCommand(apiVersionListCmd)

	RootCmd.AddCommand(apiVersionCmd)
}

var apiVersionCmd = &cobra.Command{
	Use:   "apiversion",
	Short: "Display/Set current API version",
	Example: `
  force apiversion
  force apiversion 40.0
  force apiversion list
`,
	Args:                  cobra.MaximumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 1 {
			setApiVersion(args[0])
		} else {
			fmt.Println(ApiVersion())
		}
	},
}

var apiVersionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API versions supported by org",
	Example: `
  force apiversion list
`,
	Args:                  cobra.MaximumNArgs(0),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		json, _ := cmd.Flags().GetBool("json")
		listApiVersions(json)
	},
}

func setApiVersion(apiVersionNumber string) {
	err := force.UpdateApiVersion(apiVersionNumber)
	if err != nil {
		ErrorAndExit("%v", err)
	}
}

func listApiVersions(jsonOutput bool) {
	data, err := force.GetAbsolute("/services/data")
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if jsonOutput {
		fmt.Println(data)
		return
	}
	var versions []map[string]string
	err = json.Unmarshal([]byte(data), &versions)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	for _, v := range versions {
		fmt.Printf("%s (%s)\n", v["version"], v["label"])
	}
}
