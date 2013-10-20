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
	if len(args) > 0 {
		code, err = ioutil.ReadFile(args[0])
	} else {
		fmt.Println(">> Start typing Apex code; press CTRL-D when finished\n")
		code, err = ioutil.ReadAll(os.Stdin)
		fmt.Println("\n\n>> Executing code...\n")
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
	force, _ := ActiveForce()
	output, err := force.Partner.ExecuteAnonymous(string(code))
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println(output)
}
