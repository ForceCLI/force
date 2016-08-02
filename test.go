package main

import (
	"fmt"
	"strconv"

	"github.com/heroku/force/salesforce"
	"github.com/heroku/force/util"
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

func RunTests(testRunner salesforce.TestRunner, tests []string, namespace string) (output salesforce.TestCoverage, err error) {
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
		util.ErrorAndExit("must specify tests to run")
	}
	force, _ := ActiveForce()
	output, err := RunTests(force.Partner, args, *namespaceTestFlag)
	success := false
	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	if verboselogging {
		fmt.Println(output.Log)
		fmt.Println()
	}
	var percent string
	fmt.Println("Coverage:")
	fmt.Println()
	for index := range output.NumberLocations {
		if output.NumberLocations[index] != 0 {
			percent = strconv.Itoa(((output.NumberLocations[index]-output.NumberLocationsNotCovered[index])/output.NumberLocations[index])*100) + "%"
		} else {
			percent = "0%"
		}
		fmt.Println("  " + percent + "   " + output.Name[index])
	}
	fmt.Println()
	fmt.Println()
	fmt.Println("Results:")
	fmt.Println()
	for index := range output.SMethodNames {
		fmt.Println("  [PASS]  " + output.SClassNames[index] + "::" + output.SMethodNames[index])
	}

	for index := range output.FMethodNames {
		fmt.Println("  [FAIL]  " + output.FClassNames[index] + "::" + output.FMethodNames[index] + ": " + output.FMessage[index])
		fmt.Println("    " + output.FStackTrace[index])
	}
	fmt.Println()
	fmt.Println()

	success = len(output.FMethodNames) == 0

	// Handle notifications
	notifySuccess("test", success)
}
