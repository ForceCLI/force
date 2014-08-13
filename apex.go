package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

var cmdApex = &Command{
	Usage: "apex [-q] [file]",
	Short: "Execute anonymous Apex code",
	Long: `
Execute anonymous Apex code

The -q flag enters quiet mode

Examples:

  force apex ~/test.apex

  force apex
  >> Start typing Apex code; press CTRL-D when finished

`,
}

func init() {
	cmdApex.Run = runApex
}

var (
	qApexFlag = cmdApex.Flag.Bool("q", false, "enters quiet mode")
)

func runApex(cmd *Command, args []string) {
	var code []byte
	var err error
	if len(args) == 1 {
		code, err = ioutil.ReadFile(args[0])
	} else if len(args) > 1 {
		fmt.Println("Got test indication.")
	} else {
		if !*qApexFlag {
			fmt.Println(">> Start typing Apex code; press CTRL-D when finished\n")
		}
		code, err = ioutil.ReadAll(os.Stdin)
		if !*qApexFlag {
			fmt.Println("\n\n>> Executing code...\n")
		}
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
