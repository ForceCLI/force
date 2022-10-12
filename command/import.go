package command

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	importCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "give more verbose output")
	importCmd.Flags().BoolVarP(&rollBackOnErrorFlag, "rollbackonerror", "r", false, "set roll back on error")
	importCmd.Flags().BoolVarP(&runAllTestsFlag, "runalltests", "t", false, "set run all tests")
	importCmd.Flags().StringVarP(&testLevelFlag, "testLevel", "l", "NoTestRun", "set test level")
	importCmd.Flags().BoolVarP(&checkOnlyFlag, "checkonly", "c", false, "set check only")
	importCmd.Flags().BoolVarP(&purgeOnDeleteFlag, "purgeondelete", "p", false, "set purge on delete")
	importCmd.Flags().BoolVarP(&allowMissingFilesFlag, "allowmissingfiles", "m", false, "set allow missing files")
	importCmd.Flags().BoolVarP(&autoUpdatePackageFlag, "autoupdatepackage", "u", false, "set auto update package")
	importCmd.Flags().BoolVarP(&ignoreWarningsFlag, "ignorewarnings", "i", false, "set ignore warnings")
	importCmd.Flags().StringVarP(&directory, "directory", "d", "metadata", "relative path to package.xml")
	importCmd.Flags().StringSliceVarP(&testsToRun, "test", "", []string{}, "Test(s) to run")
	RootCmd.AddCommand(importCmd)
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import metadata from a local directory",
	Example: `
  force import
  force import -directory=my_metadata -c -r -v
  force import -checkonly -runalltests
`,
	Run: func(cmd *cobra.Command, args []string) {
		runImport()
	},
	Args: cobra.MaximumNArgs(0),
}

var (
	testsToRun            metaName
	rollBackOnErrorFlag   bool
	runAllTestsFlag       bool
	testLevelFlag         string
	checkOnlyFlag         bool
	purgeOnDeleteFlag     bool
	allowMissingFilesFlag bool
	autoUpdatePackageFlag bool
	ignoreWarningsFlag    bool
	directory             string
	verbose               bool
)

func runImport() {
	wd, _ := os.Getwd()
	usr, err := user.Current()
	var dir string

	//Manually handle shell expansion short cut
	if err != nil {
		if strings.HasPrefix(directory, "~") {
			ErrorAndExit("Cannot determine tilde expansion, please use relative or absolute path to directory.")
		} else {
			dir = directory
		}
	} else {
		if strings.HasPrefix(directory, "~") {
			dir = strings.Replace(directory, "~", usr.HomeDir, 1)
		} else {
			dir = directory
		}
	}

	root := filepath.Join(wd, dir)

	// Check for absolute path
	if filepath.IsAbs(dir) {
		root = dir
	}

	files := make(ForceMetadataFiles)
	if _, err := os.Stat(filepath.Join(root, "package.xml")); os.IsNotExist(err) {
		ErrorAndExit(" \n" + filepath.Join(root, "package.xml") + "\ndoes not exist")
	}

	err = filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
		if f.Mode().IsRegular() {
			if f.Name() != ".DS_Store" {
				data, err := ioutil.ReadFile(path)
				if err != nil {
					ErrorAndExit(err.Error())
				}
				files[strings.Replace(path, fmt.Sprintf("%s%s", root, string(os.PathSeparator)), "", -1)] = data
			}
		}
		return nil
	})
	if err != nil {
		ErrorAndExit(err.Error())
	}
	var DeploymentOptions ForceDeployOptions
	DeploymentOptions.AllowMissingFiles = allowMissingFilesFlag
	DeploymentOptions.AutoUpdatePackage = autoUpdatePackageFlag
	DeploymentOptions.CheckOnly = checkOnlyFlag
	DeploymentOptions.IgnoreWarnings = ignoreWarningsFlag
	DeploymentOptions.PurgeOnDelete = purgeOnDeleteFlag
	DeploymentOptions.RollbackOnError = rollBackOnErrorFlag
	DeploymentOptions.TestLevel = testLevelFlag
	if runAllTestsFlag {
		DeploymentOptions.TestLevel = "RunAllTestsInOrg"
	}
	DeploymentOptions.RunTests = testsToRun

	result, err := force.Metadata.Deploy(files, DeploymentOptions)
	problems := result.Details.ComponentFailures
	successes := result.Details.ComponentSuccesses
	testFailures := result.Details.RunTestResult.TestFailures
	testSuccesses := result.Details.RunTestResult.TestSuccesses
	codeCoverageWarnings := result.Details.RunTestResult.CodeCoverageWarnings
	if err != nil {
		ErrorAndExit(err.Error())
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

	fmt.Printf("\nTest Successes - %d\n", len(testSuccesses))
	for _, failure := range testSuccesses {
		fmt.Printf("  [PASS]  %s::%s\n", failure.Name, failure.MethodName)
	}

	fmt.Printf("\nFailures - %d\n", len(problems))
	for _, problem := range problems {
		if problem.FullName == "" {
			fmt.Println(problem.Problem)
		} else {
			fmt.Printf("\"%s\", line %d: %s %s\n", problem.FullName, problem.LineNumber, problem.ProblemType, problem.Problem)
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
	}

	if len(problems) > 0 {
		err = errors.New("Some components failed deployment")
	} else if len(testFailures) > 0 {
		err = errors.New("Some tests failed")
	} else if !result.Success {
		err = errors.New(fmt.Sprintf("Status: %s, Status Code: %s, Error Message: %s", result.Status, result.ErrorStatusCode, result.ErrorMessage))
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("Imported from %s\n", root)
}
