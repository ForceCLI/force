package main

import (
	"fmt"
	"strconv"
	"strings"
)

var cmdTest = &Command{
	Usage: "test (all | classname...)",
	Short: "Run apex tests",
	Long: `
Run apex tests

Test Options
  -namespace=<namespace>     Select namespace to run test from
  -v                         Verbose logging

Examples:

  force test all
  force test Test1 Test2 Test3
  force test -namespace=ns Test4 
  force test -v Test1
`,
}

func init() {
	cmdTest.Flag.BoolVar(&verboselogging, "v", false, "set verbose logging")
	cmdTest.Run = runTests
}

var (
	namespaceTestFlag = cmdTest.Flag.String("namespace", "", "namespace to run tests in")
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

func runTests(cmd *Command, args []string) {
	if len(args) < 1 {
		ErrorAndExit("must specify tests to run")
	}
	force, _ := ActiveForce()
	output, err := RunTests(force.Partner, args, *namespaceTestFlag)

	if err != nil {
		ErrorAndExit(err.Error())
	}
	if verboselogging {
		fmt.Println(output.Log)
		fmt.Println()
	}

	success := false

	results := GenerateResults(output)

	fmt.Print(results)

	success = len(output.FMethodNames) == 0
	// Handle notifications
	notifySuccess("test", success)

}

func GenerateResults(output TestCoverage) string {
	var results []string
	var percent int
	results = append(results, "Coverage:")
	results = append(results, "")
	for index := range output.NumberLocations {
		if output.NumberLocations[index] != 0 {
			percent = ((output.NumberLocations[index] - output.NumberLocationsNotCovered[index]) / output.NumberLocations[index]) * 100
		}

		if percent > 0 {
			results = append(results, "   "+strconv.Itoa(percent)+"%   "+output.Name[index])
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
