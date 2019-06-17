package command

import (
	"errors"
	"fmt"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdQuickDeploy = &Command{
	Usage: "quickdeploy [deployment options] <validation id>",
	Short: "Quick deploy validation id",
	Long: `
Quick deploy validation id

Deployment Options
  -verbose, -v	  Provide detailed feedback on operation

Examples:

  force quickdeploy 0Af1200000FFbBzCAL

  force quickdeploy -v 0Af0b000000ZvXH
`,
	MaxExpectedArgs: -1,
}

var (
	verboseFlag = cmdQuickDeploy.Flag.Bool("verbose", false, "give more verbose output")
)

func init() {
	cmdQuickDeploy.Run = runQuickDeploy
	cmdQuickDeploy.Flag.BoolVar(verboseFlag, "v", false, "give more verbose output")
}

func runQuickDeploy(cmd *Command, args []string) {
	if len(args) != 1 {
		ErrorAndExit("The quickdeploy command only accepts a single validation id")
	}
	quickDeployId := args[0]

	force, err := ActiveForce()
	if err != nil {
		ErrorAndExit(err.Error())
	}

	result, err := force.Metadata.DeployRecentValidation(quickDeployId)
	problems := result.Details.ComponentFailures
	successes := result.Details.ComponentSuccesses
	if err != nil {
		ErrorAndExit(err.Error())
	}

	fmt.Printf("\nFailures - %d\n", len(problems))
	if *verboseFlag {
		for _, problem := range problems {
			if problem.FullName == "" {
				fmt.Println(problem.Problem)
			} else {
				fmt.Printf("%s: %s\n", problem.FullName, problem.Problem)
			}
		}
	}

	fmt.Printf("\nSuccesses - %d\n", len(successes))
	if *verboseFlag {
		for _, success := range successes {
			if success.FullName != "package.xml" {
				verb := "unchanged"
				if success.Changed {
					verb = "changed"
				} else if success.Deleted {
					verb = "deleted"
				} else if success.Created {
					verb = "created"
				}
				fmt.Printf("%s\n\tstatus: %s\n\tid=%s\n", success.FullName, verb, success.Id)
			}
		}
	}
	if len(problems) > 0 {
		err = errors.New("Some components failed deployment")
	} else if !result.Success {
		err = errors.New(fmt.Sprintf("Status: %s", result.Status))
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("Deploy Id %s\n", result.Id)
}
