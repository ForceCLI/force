package command

import (
	"errors"
	"fmt"
	"os"
	"time"

	"strings"

	"github.com/ForceCLI/force/desktop/notify"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

var (
	namespaceTestFlag   string
	classFlag           string
	verboselogging      bool
	integrationTestFlag bool
	debugTestFlag       bool
	checkpointFlag      []string
	webTestFlag         bool
)

func init() {
	testCmd.Flags().BoolVarP(&verboselogging, "verbose", "v", false, "set verbose logging")
	testCmd.Flags().StringVarP(&namespaceTestFlag, "namespace", "n", "", "namespace to run tests in")
	testCmd.Flags().StringP("reporttype", "f", "text", "report type format (text or junit)")
	testCmd.Flags().StringVarP(&classFlag, "class", "c", "", "class to run tests from")
	testCmd.Flags().BoolVar(&integrationTestFlag, "integration", false, "run an @IntegrationTest class asynchronously via the Tooling API")
	testCmd.Flags().BoolVar(&debugTestFlag, "debug", false, "replay the test run in the debugger after showing the results (see force log debug)")
	testCmd.Flags().StringArrayVar(&checkpointFlag, "checkpoint", nil, "with --debug, "+checkpointFlagUsage)
	testCmd.Flags().BoolVar(&webTestFlag, "web", false, "with --debug, "+webFlagUsage)
	RootCmd.AddCommand(testCmd)
}

var testCmd = &cobra.Command{
	Use:   "test (all | classname... | classname.method...)",
	Short: "Run apex tests",
	Long: `
Run apex tests

Examples:

  force test all
  force test Test1 Test2 Test3
  force test Test1.method1 Test1.method2
  force test -namespace=ns Test4
  force test -class=Test1 method1 method2
  force test -v Test1
  force test --integration MyIntegrationTest
  force test --debug Test1.method1
  force test --debug --checkpoint MyClass:42 Test1.method1

With --debug, the tests run while your DEVELOPER_LOG trace flag uses the
Force_CLI_Replay debug level, which logs APEX_CODE at FINEST. After the
results are shown, the debug log of the run opens in the replay debugger
described in "force log debug". All of the tests in the run share one log.
The trace flag is restored when the tests finish.
`,
	Args: func(cmd *cobra.Command, args []string) error {
		if debugTestFlag && integrationTestFlag {
			return errors.New("--debug cannot be used with --integration")
		}
		if len(checkpointFlag) > 0 && !debugTestFlag {
			return errors.New("--checkpoint requires --debug")
		}
		if webTestFlag && !debugTestFlag {
			return errors.New("--web requires --debug")
		}
		if _, err := parseCheckpoints(checkpointFlag); err != nil {
			return err
		}
		return nil
	},

	Run: func(cmd *cobra.Command, args []string) {
		reportFormat, _ := cmd.Flags().GetString("reporttype")
		if integrationTestFlag {
			runIntegrationTest(reportFormat, args)
			return
		}
		runTests(reportFormat, args)
	},
}

func RunTests(testRunner TestRunner, tests []string, namespace string) (output TestCoverage, err error) {
	output, err = testRunner.RunTests(tests, namespace)
	if err != nil {
		return
	}
	if output.NumberRun == 0 && output.NumberFailures == 0 {
		err = fmt.Errorf("Test classes specified not found: %v", tests)
		return
	}
	return
}

func QualifyMethods(class string, methods []string) []string {
	if len(methods) == 0 {
		return []string{class}
	}
	var qualified []string
	for _, method := range methods {
		qualified = append(qualified, fmt.Sprintf("%s.%s", class, method))
	}
	return qualified
}

func GenerateResults(output TestCoverage) string {
	var results []string
	var percent int
	results = append(results, "Coverage:")
	results = append(results, "")
	for index := range output.NumberLocations {
		if output.NumberLocations[index] != 0 {
			locations := float64(output.NumberLocations[index])
			notCovered := float64(output.NumberLocationsNotCovered[index])
			percent = int((locations - notCovered) / locations * 100)
		}

		if percent > 0 {
			results = append(results, fmt.Sprintf("%6d%%  %s", percent, output.Name[index]))
		}
	}
	results = append(results, "")
	results = append(results, "")
	results = append(results, "Results:")
	results = append(results, "")
	for index := range output.SMethodNames {
		results = append(results, "  [PASS]  "+output.SClassNames[index]+"::"+output.SMethodNames[index])
	}

	for index := range output.FMethodNames {
		results = append(results, "  [FAIL]  "+output.FClassNames[index]+"::"+output.FMethodNames[index]+": "+output.FMessage[index])
		results = append(results, "    "+output.FStackTrace[index])
	}
	results = append(results, "")
	results = append(results, "")

	result := strings.Join(results, "\n")

	return result
}

func runTests(reportFormat string, args []string) {
	if len(args) < 1 && classFlag == "" {
		ErrorAndExit("must specify tests to run")
	}
	if classFlag != "" {
		args = QualifyMethods(classFlag, args)
	}
	for i, t := range args {
		args[i] = strings.Replace(t, "::", ".", 1)
	}
	var result TestCoverage
	var err error
	if debugTestFlag {
		result, err = runTestsForReplay(args)
	} else {
		result, err = RunTests(force.Partner, args, namespaceTestFlag)
	}

	if err != nil {
		ErrorAndExit(err.Error())
	}
	if verboselogging {
		fmt.Println(result.Log)
		fmt.Println()
	}

	junitOutput := reportFormat == "junit"
	switch {
	case junitOutput:
		output, err := result.ToJunit()
		if err != nil {
			ErrorAndExit(err.Error())
		}
		fmt.Println(output)
		replayTestLog(result)
		return
	default:
		results := GenerateResults(result)
		fmt.Print(results)

		success := len(result.FMethodNames) == 0
		// Handle notifications
		notify.NotifySuccess("test", success)
		replayTestLog(result)
		if !success {
			ErrorAndExit("Tests Failed")
		}
	}
}

// runTestsForReplay runs tests while the user's trace flag logs Apex at
// FINEST, and returns the results with the debug log of the run, fetched by
// the log ID in the runTests result.
func runTestsForReplay(tests []string) (TestCoverage, error) {
	checkpoints, err := parseCheckpoints(checkpointFlag)
	if err != nil {
		return TestCoverage{}, err
	}
	removeCheckpoints, err := createCheckpoints(checkpoints)
	if err != nil {
		return TestCoverage{}, err
	}
	defer removeCheckpoints()
	restore, err := force.EnableReplayTraceFlag()
	if err != nil {
		return TestCoverage{}, err
	}
	result, err := RunTests(force.Partner, tests, namespaceTestFlag)
	restoreTraceFlag(restore)
	if err != nil {
		return result, err
	}
	if result.ApexLogId == "" {
		return result, errors.New("the test run did not return a debug log ID")
	}
	result.Log, err = force.RetrieveLog(result.ApexLogId)
	return result, err
}

// restoreTraceFlag puts back the trace flag changed for the replay
// debugger. A failure is reported without stopping the command.
func restoreTraceFlag(restore func() error) {
	if err := restore(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %v\n", err)
	}
}

func replayTestLog(result TestCoverage) {
	if !debugTestFlag {
		return
	}
	if err := replayLog(result.Log, webTestFlag); err != nil {
		ErrorAndExit(err.Error())
	}
}

// runIntegrationTest drives the Tooling API runTestsAsynchronous endpoint for a
// single @IntegrationTest class. The Salesforce Tooling API only allows one
// concurrent asynchronous integration test run at a time, so we reject
// requests that target more than one class.
func runIntegrationTest(reportFormat string, args []string) {
	if classFlag != "" {
		args = append([]string{classFlag}, args...)
	}
	if len(args) != 1 {
		ErrorAndExit("--integration requires exactly one class (or class.method) argument")
	}
	target := strings.Replace(args[0], "::", ".", 1)

	result, err := force.RunIntegrationTest(target, 2*time.Second, 30*time.Minute)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	output := GenerateIntegrationTestResults(result)
	fmt.Print(output)

	success := result.Status == "Completed" && result.MethodsFailed == 0
	notify.NotifySuccess("test", success)
	if !success {
		ErrorAndExit("Integration tests Failed")
	}
}

// GenerateIntegrationTestResults formats an IntegrationTestResult for display.
func GenerateIntegrationTestResults(result IntegrationTestResult) string {
	var b strings.Builder
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Integration Test Run: %s (status: %s)\n", result.AsyncApexJobID, result.Status)
	fmt.Fprintf(&b, "  Classes:  %d/%d completed\n", result.ClassesCompleted, result.ClassesEnqueued)
	fmt.Fprintf(&b, "  Methods:  %d/%d completed, %d failed\n", result.MethodsCompleted, result.MethodsEnqueued, result.MethodsFailed)
	if result.TestTime > 0 {
		fmt.Fprintf(&b, "  TestTime: %dms\n", result.TestTime)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Results:")
	for _, r := range result.Results {
		switch r.Outcome {
		case "Pass":
			fmt.Fprintf(&b, "  [PASS]  %s::%s (%dms)\n", r.ClassName, r.MethodName, r.RunTime)
		default:
			fmt.Fprintf(&b, "  [%s]  %s::%s: %s\n", strings.ToUpper(r.Outcome), r.ClassName, r.MethodName, r.Message)
			if r.StackTrace != "" {
				fmt.Fprintf(&b, "    %s\n", r.StackTrace)
			}
		}
	}
	fmt.Fprintln(&b)
	return b.String()
}
