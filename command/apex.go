package command

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	apexCmd.Flags().BoolP("test", "t", false, "run in test context")
	apexCmd.Flags().Bool("debug", false, "replay the execution in the debugger (see force log debug)")
	apexCmd.Flags().StringArray("checkpoint", nil, "with --debug, "+checkpointFlagUsage)
	apexCmd.Flags().Bool("web", false, "with --debug, "+webFlagUsage)
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
  force apex --debug --checkpoint MyClass:42 ~/test.apex
  `,
	Args: func(cmd *cobra.Command, args []string) error {
		testContext, _ := cmd.Flags().GetBool("test")
		debug, _ := cmd.Flags().GetBool("debug")
		checkpoints, _ := cmd.Flags().GetStringArray("checkpoint")
		if len(checkpoints) > 0 && !debug {
			return errors.New("--checkpoint requires --debug")
		}
		if web, _ := cmd.Flags().GetBool("web"); web && !debug {
			return errors.New("--web requires --debug")
		}
		if _, err := parseCheckpoints(checkpoints); err != nil {
			return err
		}
		if testContext && debug {
			// compileAndTest returns and saves a log of the compile request
			// only; the test run is not logged.
			return errors.New("--debug cannot be used with --test: Salesforce does not log the test run of code run with --test")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		testContext, _ := cmd.Flags().GetBool("test")
		debug, _ := cmd.Flags().GetBool("debug")
		values, _ := cmd.Flags().GetStringArray("checkpoint")
		checkpoints, _ := parseCheckpoints(values)
		web, _ := cmd.Flags().GetBool("web")
		options := apexDebugOptions{enabled: debug, checkpoints: checkpoints, web: web}
		switch len(args) {
		case 1:
			runApex(readApexFile(args[0]), testContext, options)
		case 0:
			runApex(readApexStdin(), testContext, options)
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

type apexDebugOptions struct {
	enabled     bool
	checkpoints []Checkpoint
	web         bool
}

func runApex(code []byte, testContext bool, debug apexDebugOptions) {
	if debug.enabled {
		debugApex(code, debug)
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
func debugApex(code []byte, options apexDebugOptions) {
	removeCheckpoints, err := createCheckpoints(options.checkpoints)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	restore, err := force.EnableReplayTraceFlag()
	if err != nil {
		removeCheckpoints()
		ErrorAndExit(err.Error())
	}
	log, err := force.Partner.WithReplayDebugging().ExecuteAnonymous(string(code))
	restoreTraceFlag(restore)
	removeCheckpoints()
	if err != nil && log == "" {
		ErrorAndExit(err.Error())
	}
	if replayErr := replayLog(log, options.web); replayErr != nil {
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
