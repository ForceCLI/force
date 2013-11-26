package main

import (
	"strconv"
	"strings"
)

var cmdTest = &Command{
	Run:   runTests,
	Usage: "tests",
	Short: "Run apex tests",
	Long: `
Execute apex tests 

Examples:

  force tests Test1 Test2 Test3
  force tests all 
`,
}

func runTests(cmd *Command, args []string) {
	if len(args) < 1 {
		ErrorAndExit("must specify tests to run")
	}
	tests := strings.Join(args, " ")
	force, _ := ActiveForce()
	output, err := force.Partner.RunTests(tests)
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		//working on a better way to do this - catches when no class are found and ran
		if output.NumberRun == "0" {
			println("test classes specified not found")
		}else{
			var percent string
			println(output.Log)
			println()
			println()
			println("Coverage:")
			println()
			//println()
			for index := range output.NumberLocations {
				if output.NumberLocations[index] != 0 {
					percent = strconv.Itoa(((output.NumberLocations[index]-output.NumberLocationsNotCovered[index])/output.NumberLocations[index])*100) + "%"
				} else {
					percent = "0%"
				} 
				println(percent + "   " + output.Name[index])
			}
			println()
			println()
			println("Results:")
			println()
			//println()
			for index := range output.SMethodNames{
				println("[PASS]    " + output.SClassNames[index] + "::" + output.SMethodNames[index])
			}

			for index := range output.FMethodNames{
				println("[FAIL]    " + output.FClassNames[index] + "::" + output.FMethodNames[index])
			}
			println()
			println()
		}
	}
}
