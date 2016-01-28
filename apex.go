package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

var cmdApex = &Command{
	Run:   runApex,
	Usage: "apex [file]",
	Short: "Execute anonymous Apex code",
	Long: `
Execute anonymous Apex code

Examples:

  force apex ~/test.apex

  force apex
  >> Start typing Apex code; press CTRL-D when finished

`,
}

func init() {
}

func runApex(cmd *Command, args []string) {
	var code []byte
	var err error
	if len(args) == 1 {
		code, err = ioutil.ReadFile(args[0])
	} else if len(args) > 1 {
		fmt.Println("Got test indication.")
	} else {
		fmt.Println(">> Start typing Apex code; press CTRL-D when finished")
		code, err = ioutil.ReadAll(os.Stdin)
		fmt.Println("\n\n>> Executing code...")
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
	force, _ := ActiveForce()
	if len(args) <= 1 {
		output, err := force.Partner.ExecuteAnonymous(string(code))
		if err != nil {
			ErrorAndExit(err.Error())
		}
		fmt.Println(output)
	} else {
		apexclass := args[1]
		fmt.Println(apexclass)
		err := force.GetCodeCoverage("", apexclass)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
}
