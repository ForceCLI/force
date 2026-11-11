package command

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	. "github.com/ForceCLI/force/error"
	"github.com/spf13/cobra"
)

func init() {
	apexCmd.Flags().BoolP("test", "t", false, "run in test context")
	apexCmd.Flags().Bool("debug", false, "replay the execution in the debugger (see force log debug)")
	RootCmd.AddCommand(apexCmd)
}

var apexCmd = &cobra.Command{
	Use:   "apex [file]",
	Short: "Execute anonymous Apex code",
	Long: `Execute anonymous Apex code

With --debug, the code runs with APEX_CODE logging at FINEST and the debug log
returned by Salesforce opens in the replay debugger described in
"force log debug". While the code runs, your DEVELOPER_LOG trace flag uses the
Force_CLI_Replay debug level, because an active trace flag overrides the
levels requested for the returned log. The trace flag is restored afterwards.
`,
	Example: `
  force apex ~/test.apex

  force apex
  >> Start typing Apex code; press CTRL-D(for Mac/Linux) / Ctrl-Z (for Windows) when finished

  force apex --debug ~/test.apex
  `,
	Args: func(cmd *cobra.Command, args []string) error {
		testContext, _ := cmd.Flags().GetBool("test")
		debug, _ := cmd.Flags().GetBool("debug")
		if testContext && debug {
			return errors.New("--debug cannot be used with --test")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		testContext, _ := cmd.Flags().GetBool("test")
		debug, _ := cmd.Flags().GetBool("debug")
		switch len(args) {
		case 1:
			runApex(readApexFile(args[0]), testContext, debug)
		case 0:
			runApex(readApexStdin(), testContext, debug)
		default:
			fmt.Println("Got test indication.  DEPRECATED.")
			getTestCoverage(args[1])
		}
	},
}

func readApexStdin() []byte {
	fmt.Println(">> Start typing Apex code; press CTRL-D(for Mac/Linux) / Ctrl-Z (for Windows) when finished")
	code, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("\n\n>> Executing code...")
	return code
}

func readApexFile(filename string) []byte {
	code, err := ioutil.ReadFile(filename)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	return code
}

func runApex(code []byte, testContext bool, debug bool) {
	if debug {
		debugApex(code)
		return
	}
	var output string
	var err error
	if testContext {
		output, err = executeAsTest(code)
	} else {
		output, err = execute(code)
	}
	fmt.Println(output)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

// debugApex runs code and opens its log in the debugger. A log is returned
// when the code throws an exception, so the debugger opens and the exception
// is reported after the debugger exits. A compile error returns no log.
//
// The request's DebuggingHeader asks for APEX_CODE at FINEST, but an active
// trace flag for the user overrides the header's levels, so the user's trace
// flag is also set to FINEST while the code runs.
func debugApex(code []byte) {
	restore, err := force.EnableReplayTraceFlag()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	log, err := force.Partner.WithReplayDebugging().ExecuteAnonymous(string(code))
	restoreTraceFlag(restore)
	if err != nil && log == "" {
		ErrorAndExit(err.Error())
	}
	if replayErr := replayLog(log); replayErr != nil {
		ErrorAndExit(replayErr.Error())
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func executeAsTest(code []byte) (string, error) {
	return force.Partner.ExecuteAnonymousTest(string(code))
}

func execute(code []byte) (string, error) {
	return force.Partner.ExecuteAnonymous(string(code))
}

func getTestCoverage(apexclass string) {
	fmt.Println(apexclass)
	err := force.GetCodeCoverage("", apexclass)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}
