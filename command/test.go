package command

import (
	"fmt"

	"github.com/ForceCLI/force/desktop"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"strings"
)

var cmdTest = &Command{
	Usage: "test (all | classname... | classname.method...)",
	Short: "Run apex tests",
	Long: `
Run apex tests

Test Options
  -namespace=<namespace>     Select namespace to run test from
  -class=class               Select class to run tests from
  -v                         Verbose logging

Examples:

  force test all
  force test Test1 Test2 Test3
  force test Test1.method1 Test1.method2
  force test -namespace=ns Test4
  force test -class=Test1 method1 method2
  force test -v Test1
`,
}

func init() {
	cmdTest.Flag.BoolVar(&verboselogging, "v", false, "set verbose logging")
	cmdTest.Run = runTests
}

var (
	namespaceTestFlag = cmdTest.Flag.String("namespace", "", "namespace to run tests in")
	classFlag         = cmdTest.Flag.String("class", "", "class to run tests from")
	verboselogging    bool
)

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

func runTests(cmd *Command, args []string) {
	if len(args) < 1 && *classFlag == "" {
		ErrorAndExit("must specify tests to run")
	}
	force, _ := ActiveForce()
	if *classFlag != "" {
		args = QualifyMethods(*classFlag, args)
	}
	output, err := RunTests(force.Partner, args, *namespaceTestFlag)

	if err != nil {
		ErrorAndExit(err.Error())
	}
	if verboselogging {
		fmt.Println(output.Log)
		fmt.Println()
	}

	results := GenerateResults(output)
	fmt.Print(results)

	success := len(output.FMethodNames) == 0
	// Handle notifications
	desktop.NotifySuccess("test", success)
	if !success {
		ErrorAndExit("Tests Failed")
	}
}
