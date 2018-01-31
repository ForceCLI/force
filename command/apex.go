package command

import (
	"fmt"
	"io/ioutil"
	"os"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdApex = &Command{
	Run:   runApex,
	Usage: "apex [file]",
	Short: "Execute anonymous Apex code",
	Long: `
Execute anonymous Apex code

Apex Options
  -test                      Run in test context

Examples:

  force apex ~/test.apex

  force apex
  >> Start typing Apex code; press CTRL-D(for Mac/Linux) / Ctrl-Z (for Windows) when finished

`,
}

func init() {
	cmdApex.Flag.BoolVar(&testContext, "test", false, "run apex from in a test context")
}

var (
	testContext bool
)

func runApex(cmd *Command, args []string) {
	var code []byte
	var err error
	if len(args) == 1 {
		code, err = ioutil.ReadFile(args[0])
	} else if len(args) > 1 {
		fmt.Println("Got test indication.")
	} else {
		fmt.Println(">> Start typing Apex code; press CTRL-D(for Mac/Linux) / Ctrl-Z (for Windows) when finished")
		code, err = ioutil.ReadAll(os.Stdin)
		fmt.Println("\n\n>> Executing code...")
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
	force, _ := ActiveForce()
	if testContext {
		output, err := force.Partner.ExecuteAnonymousTest(string(code))
		if err != nil {
			ErrorAndExit(err.Error())
		}
		fmt.Println(output)
	} else if len(args) <= 1 {
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
