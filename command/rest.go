package command

import (
	"fmt"
	"io"
	"os"
	"strings"

	. "github.com/ForceCLI/force/error"
	"github.com/spf13/cobra"
)

func init() {
	restCmd.AddCommand(restGetCmd)
	restCmd.AddCommand(restPostCmd)
	restCmd.AddCommand(restPatchCmd)
	restCmd.AddCommand(restPutCmd)
	restCmd.PersistentFlags().BoolP("absolute", "A", false, "use URL as-is (do not prepend /services/data/vXX.0)")
	RootCmd.AddCommand(restCmd)
}

var restCmd = &cobra.Command{
	Use:   "rest <method> <url>",
	Short: "Execute a REST request",
	Example: `
  force rest get "/tooling/query?q=Select id From Account"
  force rest get /appMenu/AppSwitcher
  force rest get -a /services/data/
  force rest post "/tooling/sobjects/CustomField/00D9A0000000TgcUAE" path/to/definition.json
  force rest put "/tooling/sobjects/CustomField/00D9A0000000TgcUAE" path/to/definition.json
`,
}

var restGetCmd = &cobra.Command{
	Use:   "get <url>",
	Short: "Execute a REST GET request",
	Args:  cobra.ExactArgs(1),
	Example: `
  force rest get "/tooling/query?q=Select id From Account"
  force rest get /appMenu/AppSwitcher
  force rest get -a /services/data/
`,
	Run: func(cmd *cobra.Command, args []string) {
		absolute, _ := cmd.Flags().GetBool("absolute")
		runGet(args[0], absolute)
	},
}

var restPostCmd = &cobra.Command{
	Use:   "post <url> [file]",
	Short: "Execute a REST POST request",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		fn := makePostPatchPut("post")
		runPostPatchPut(fn, cmd, args)
	},
}

var restPatchCmd = &cobra.Command{
	Use:   "patch <url> [file]",
	Short: "Execute a REST PATCH request",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		fn := makePostPatchPut("patch")
		runPostPatchPut(fn, cmd, args)
	},
}

var restPutCmd = &cobra.Command{
	Use:   "put <url> [file]",
	Short: "Execute a REST PUT request",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		fn := makePostPatchPut("put")
		runPostPatchPut(fn, cmd, args)
	},
}

type postPatchPutFn func(url string, input []byte, absolute bool)

func runPostPatchPut(fn postPatchPutFn, cmd *cobra.Command, args []string) {
	var data []byte
	var err error
	absolute, _ := cmd.Flags().GetBool("absolute")
	if len(args) == 2 {
		data, err = os.ReadFile(args[1])
	} else {
		data, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fn(args[0], data, absolute)
}

func runGet(url string, absolute bool) {
	var err error
	var data string
	if absolute {
		data, err = force.GetAbsolute(url)
	} else {
		data, err = force.GetREST(url)
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println(data)
}

func makePostPatchPut(action string) postPatchPutFn {
	action = strings.ToUpper(action)
	return func(url string, input []byte, absolute bool) {
		var (
			data string
			err  error
		)
		if absolute {
			data, err = force.PostPatchAbsolute(url, string(input), action)
		} else {
			data, err = force.PostPatchREST(url, string(input), action)
		}
		if err != nil {
			ErrorAndExit(err.Error())
		}
		msg := fmt.Sprintf("%s %s\n%s", action, url, data)
		fmt.Println(msg)
	}
}
