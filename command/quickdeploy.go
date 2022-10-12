package command

import (
	"errors"
	"fmt"

	. "github.com/ForceCLI/force/error"
	"github.com/spf13/cobra"
)

func init() {
	quickDeployCmd.Flags().BoolP("verbose", "v", false, "give more verbose output")
	RootCmd.AddCommand(quickDeployCmd)
}

var quickDeployCmd = &cobra.Command{
	Use:   "quickdeploy <validation id>",
	Short: "Quick deploy validation id",
	Example: `
  force quickdeploy 0Af1200000FFbBzCAL
  force quickdeploy -v 0Af0b000000ZvXH
`,
	Args:                  cobra.ExactValidArgs(1),
	DisableFlagsInUseLine: false,
	Run: func(cmd *cobra.Command, args []string) {
		verbose, _ := cmd.Flags().GetBool("verbose")
		runQuickDeploy(args[0], verbose)
	},
}

func runQuickDeploy(quickDeployId string, verbose bool) {
	result, err := force.Metadata.DeployRecentValidation(quickDeployId)
	problems := result.Details.ComponentFailures
	successes := result.Details.ComponentSuccesses
	if err != nil {
		ErrorAndExit(err.Error())
	}

	fmt.Printf("\nFailures - %d\n", len(problems))
	if verbose {
		for _, problem := range problems {
			if problem.FullName == "" {
				fmt.Println(problem.Problem)
			} else {
				fmt.Printf("%s: %s\n", problem.FullName, problem.Problem)
			}
		}
	}

	fmt.Printf("\nSuccesses - %d\n", len(successes))
	if verbose {
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
