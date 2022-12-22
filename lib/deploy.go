package lib

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/ForceCLI/force/desktop"
	. "github.com/ForceCLI/force/error"
)

// Creates a package that includes everything in the passed in string slice
// and then deploys the package to salesforce
func PushByPaths(force *Force, fpaths []string, byName bool, namePaths map[string]string, opts *ForceDeployOptions) {
	pb := NewPushBuilder()
	var badPaths []string
	for _, fpath := range fpaths {

		fi, err := os.Stat(fpath)
		if err != nil {
			Log.Info(err.Error())
			badPaths = append(badPaths, fpath)
			continue
		}

		mode := fi.Mode()
		//If path provided is dir we are adding all containing files to deployment
		if mode.IsDir() {
			dirNamePaths, dirBadPath, err := pb.AddDirectory(fpath)
			if err != nil {
				Log.Info(err.Error())
				badPaths = append(badPaths, dirBadPath...)
			} else {
				for dirContentName, dirContentPath := range dirNamePaths {
					namePaths[dirContentName] = dirContentPath
				}
			}
		} else if mode.IsRegular() { // single file processing
			name, err := pb.AddFile(fpath)
			if err != nil {
				Log.Info(err.Error())
				badPaths = append(badPaths, fpath)
			} else {
				// Store paths by name for error messages
				namePaths[name] = fpath
			}
		}
	}

	if len(badPaths) == 0 {
		Log.Info("Deploying now...")
		t0 := time.Now()
		deployFiles(force, pb.ForceMetadataFiles(), byName, namePaths, opts)
		t1 := time.Now()
		Log.Info(fmt.Sprintf("The deployment took %v to run.\n", t1.Sub(t0)))
	} else {
		ErrorAndExit("Could not add the following files:\n {%v}", strings.Join(badPaths, "\n"))
	}
}

func deployFiles(force *Force, files ForceMetadataFiles, byName bool, namePaths map[string]string, opts *ForceDeployOptions) {
	result, err := force.Metadata.Deploy(files, *opts)
	err = processDeployResults(result, byName, namePaths, err)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	return
}

// Process and display the result of the push operation
func processDeployResults(result ForceCheckDeploymentStatusResult, byName bool, namePaths map[string]string, deployErr error) (err error) {
	if deployErr != nil {
		ErrorAndExit(deployErr.Error())
	}

	problems := result.Details.ComponentFailures
	successes := result.Details.ComponentSuccesses
	testFailures := result.Details.RunTestResult.TestFailures
	testSuccesses := result.Details.RunTestResult.TestSuccesses
	codeCoverageWarnings := result.Details.RunTestResult.CodeCoverageWarnings

	if len(successes) > 0 {
		fmt.Printf("\nSuccesses - %d\n", len(successes)-1)
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
				fmt.Printf("\t%s: %s\n", success.FullName, verb)
			}
		}
	}

	fmt.Printf("\nTest Successes - %d\n", len(testSuccesses))
	for _, failure := range testSuccesses {
		fmt.Printf("  [PASS]  %s::%s\n", failure.Name, failure.MethodName)
	}

	if len(problems) > 0 {
		fmt.Printf("\nFailures - %d\n", len(problems))
		for _, problem := range problems {
			if problem.FullName == "" {
				fmt.Println(problem.Problem)
			} else {
				if byName {
					fmt.Printf("ERROR with %s, line %d\n %s\n", problem.FullName, problem.LineNumber, problem.Problem)
				} else {
					fname, found := namePaths[problem.FullName]
					if !found {
						fname = problem.FullName
					}
					fmt.Printf("\"%s\", line %d: %s %s\n", fname, problem.LineNumber, problem.ProblemType, problem.Problem)
				}
			}
		}
	}

	fmt.Printf("\nTest Failures - %d\n", len(testFailures))
	for _, failure := range testFailures {
		fmt.Printf("\n  [FAIL]  %s::%s: %s\n", failure.Name, failure.MethodName, failure.Message)
		fmt.Println(failure.StackTrace)
	}

	if len(codeCoverageWarnings) > 0 {
		fmt.Printf("\nCode Coverage Warnings - %d\n", len(codeCoverageWarnings))
		for _, warning := range codeCoverageWarnings {
			fmt.Printf("\n %s: %s\n", warning.Name, warning.Message)
		}
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

	// Handle notifications
	desktop.NotifySuccess("push", len(problems) == 0)
	if len(problems) > 0 {
		err = errors.New("Some components failed deployment")
	} else if len(testFailures) > 0 {
		err = errors.New("Some tests failed")
	} else if !result.Success {
		err = errors.New(fmt.Sprintf("Status: %s", result.Status))
	}
	return
}

// Deploy a previously create package. This is used for "force push package". In this case the
// --path flag should be pointing to a zip file that may or may not have come from a different
// org altogether
func DeployPackage(force *Force, resourcepaths []string, opts *ForceDeployOptions) {
	for _, name := range resourcepaths {
		zipfile, err := ioutil.ReadFile(name)
		result, err := force.Metadata.DeployZipFile(zipfile, *opts)
		byName := false
		namePaths := make(map[string]string)
		err = processDeployResults(result, byName, namePaths, err)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
	return
}
