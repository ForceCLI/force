package command

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

type deployOutputOptions struct {
	verbosity                  int
	quiet                      bool
	reportFormat               string
	ignoreCodeCoverageWarnings bool
	suppressUnexpectedError    bool
}

func defaultDisplayOptions() *deployOutputOptions {
	o := deployOutputOptions{
		reportFormat:               "text",
		quiet:                      false,
		verbosity:                  0,
		ignoreCodeCoverageWarnings: false,
		suppressUnexpectedError:    false,
	}
	return &o
}

func deploy(force *Force, files ForceMetadataFiles, deployOptions *ForceDeployOptions, displayOptions *deployOutputOptions) error {
	if displayOptions.quiet {
		previousLogger := Log
		var l quietLogger
		Log = l
		defer func() {
			Log = previousLogger
		}()
	}
	startTime := time.Now()
	deployId, err := force.Metadata.StartDeploy(files, *deployOptions)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	stopDeployUponSignal(force, deployId)
	var result ForceCheckDeploymentStatusResult
	retrying := false
	for {
		result, err = force.Metadata.CheckDeployStatus(deployId)
		if err != nil {
			if retrying {
				return fmt.Errorf("Error getting deploy status: %w", err)
			} else {
				retrying = true
				Log.Info(fmt.Sprintf("Received error checking deploy status: %s.  Will retry once before aborting.", err.Error()))
			}
		} else {
			retrying = false
		}
		if result.Done {
			break
		}
		if !retrying {
			Log.Info(result)
		}
		time.Sleep(5000 * time.Millisecond)
	}
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	junitOutput := displayOptions.reportFormat == "junit"

	if displayOptions.suppressUnexpectedError {
		filteredComponentFailures := result.Details.ComponentFailures[:0]
		for _, f := range result.Details.ComponentFailures {
			if !strings.HasPrefix(f.Problem, `An unexpected error occurred. Please include this ErrorId`) {
				filteredComponentFailures = append(filteredComponentFailures, f)
			}
		}
		result.Details.ComponentFailures = filteredComponentFailures
	}

	switch {
	case displayOptions.quiet:
	case junitOutput:
		output, err := result.ToJunit(duration.Seconds())
		if err != nil {
			return fmt.Errorf("Failed to generate output: %w", err)
		}
		fmt.Println(output)
		if result.HasComponentFailures() || (result.HasTestFailures() && exitCodeOnTestFailure) || !result.Success {
			return fmt.Errorf("Deploy unsuccessful")
		}
		return nil
	default:
		output := result.ToString(duration.Seconds(), displayOptions.verbosity > 0)
		fmt.Println(output)

		codeCoverageWarnings := result.Details.RunTestResult.CodeCoverageWarnings
		if !displayOptions.ignoreCodeCoverageWarnings && len(codeCoverageWarnings) > 0 {
			fmt.Printf("\nCode Coverage Warnings - %d\n", len(codeCoverageWarnings))
			for _, warning := range codeCoverageWarnings {
				fmt.Printf("\n %s: %s\n", warning.Name, warning.Message)
			}
			if displayOptions.verbosity > 1 {
				for _, c := range result.Details.RunTestResult.CodeCoverage {
					component := c.Name
					if c.Namespace != "" {
						component = c.Namespace + "." + c.Name
					}

					for _, line := range c.LocationsNotCovered {
						fmt.Printf("%s %s: Line %d not covered\n", c.Type, component, line.Line)
					}
				}
			}
		}

		if result.HasComponentFailures() {
			err = errors.New("Some components failed deployment")
		} else if result.HasTestFailures() && exitCodeOnTestFailure {
			err = errors.New("Some tests failed")
		} else if !result.Success {
			err = errors.New(fmt.Sprintf("Status: %s, Status Code: %s, Error Message: %s", result.Status, result.ErrorStatusCode, result.ErrorMessage))
		}
		if err != nil {
			return fmt.Errorf("Deploy unsuccessful")
		}
	}
	return nil
}

func stopDeployUponSignal(force *Force, deployId string) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		interuptsReceived := 0
		for {
			<-sigs
			if interuptsReceived > 0 {
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Cancelling deploy %s\n", deployId)
			force.Metadata.CancelDeploy(deployId)
			interuptsReceived++
		}
	}()
}

func getDeploymentOutputOptions(cmd *cobra.Command) *deployOutputOptions {
	displayOptions := defaultDisplayOptions()

	displayOptions.reportFormat, _ = cmd.Flags().GetString("reporttype")
	displayOptions.quiet, _ = cmd.Flags().GetBool("quiet")
	displayOptions.verbosity, _ = cmd.Flags().GetCount("verbose")

	ignoreCoverageWarnings, _ := cmd.Flags().GetBool("ignorecoverage")
	displayOptions.ignoreCodeCoverageWarnings = ignoreCoverageWarnings

	suppressUnexpectedError, _ := cmd.Flags().GetBool("suppressunexpected")
	displayOptions.suppressUnexpectedError = suppressUnexpectedError

	return displayOptions
}

func getDeploymentOptions(cmd *cobra.Command) ForceDeployOptions {
	var deploymentOptions ForceDeployOptions
	deploymentOptions.AllowMissingFiles, _ = cmd.Flags().GetBool("allowmissingfiles")
	deploymentOptions.AutoUpdatePackage, _ = cmd.Flags().GetBool("autoupdatepackage")
	deploymentOptions.CheckOnly, _ = cmd.Flags().GetBool("checkonly")
	deploymentOptions.IgnoreWarnings, _ = cmd.Flags().GetBool("ignorewarnings")
	deploymentOptions.PurgeOnDelete, _ = cmd.Flags().GetBool("purgeondelete")
	deploymentOptions.RollbackOnError, _ = cmd.Flags().GetBool("rollbackonerror")
	deploymentOptions.TestLevel, _ = cmd.Flags().GetString("testlevel")
	deploymentOptions.RunTests, _ = cmd.Flags().GetStringSlice("test")
	deploymentOptions.SinglePackage = true
	runAllTests, _ := cmd.Flags().GetBool("runalltests")
	if runAllTests {
		deploymentOptions.TestLevel = "RunAllTestsInOrg"
	}
	if cmd.Flags().Changed("test") && len(deploymentOptions.RunTests) == 0 {
		// NoTestRun can't be used when deploying to production, but
		// RunSpecifiedTests can be used with an empty set of tests by passing
		// `--test ''`
		deploymentOptions.TestLevel = "RunSpecifiedTests"
		deploymentOptions.RunTests = []string{""}
	}
	return deploymentOptions
}
