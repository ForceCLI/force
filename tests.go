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
			println("NumberRun: " + output.NumberRun)
			println()
			println(output.Log)
			println()
			println("****Test Coverage****")
			println("__________________________________")
			var percent string
			var locs_covered, locs_not int
			for index := range output.NumberLocations {
				if output.NumberLocations[index] != 0 {
					percent = strconv.Itoa(((output.NumberLocations[index]-output.NumberLocationsNotCovered[index])/output.NumberLocations[index])*100) + "%"
					locs_covered += output.NumberLocations[index]
					locs_not += output.NumberLocationsNotCovered[index]
				} else {
					percent = "0%"
				} 
				println(output.Name[index] + "  " + output.Type[index] + "  " + percent)
			}
			println("__________________________________")
			println("Total" + "  " + strconv.Itoa(((locs_covered-locs_not)/locs_covered)*100) + "%")
			
			println()
			println()

			println("****SUCCESSES****")
			println("__________________________________")
			for index := range output.SMethodNames{
				println(output.SClassNames[index] + "    " + output.SMethodNames[index])
			}

			println()
			println()

			println("****FAILURES****")
			println("__________________________________")
			for index := range output.FMethodNames{
				println(output.FClassNames[index] + "    " + output.FMethodNames[index])
			}
		}
	}
}
