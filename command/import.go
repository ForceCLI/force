package command

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	. "github.com/ForceCLI/force/error"
	"github.com/ForceCLI/force/lib"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	importCmd.Flags().BoolP("rollbackonerror", "r", false, "roll back deployment on error")
	importCmd.Flags().BoolP("runalltests", "t", false, "run all tests (equivalent to --testlevel RunAllTestsInOrg)")
	importCmd.Flags().StringP("testlevel", "l", "NoTestRun", "test level")
	importCmd.Flags().BoolP("checkonly", "c", false, "check only deploy")
	importCmd.Flags().BoolP("purgeondelete", "p", false, "purge metadata from org on delete")
	importCmd.Flags().BoolP("allowmissingfiles", "m", false, "set allow missing files")
	importCmd.Flags().BoolP("autoupdatepackage", "u", false, "set auto update package")
	importCmd.Flags().BoolP("ignorewarnings", "i", false, "ignore warnings")

	importCmd.Flags().StringSliceVarP(&testsToRun, "test", "", []string{}, "Test(s) to run")

	importCmd.Flags().BoolVarP(&ignoreCodeCoverageWarnings, "ignorecoverage", "w", false, "suppress code coverage warnings")
	importCmd.Flags().BoolVarP(&exitCodeOnTestFailure, "erroronfailure", "E", true, "exit with an error code if any tests fail")
	importCmd.Flags().BoolVarP(&suppressUnexpectedError, "suppressunexpected", "U", true, `suppress "An unexpected error occurred" messages`)
	importCmd.Flags().StringP("directory", "d", "src", "relative path to package.xml")

	importCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "only output failures")
	importCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "give more verbose output")

	importCmd.Flags().StringP("reporttype", "f", "text", "report type format (text or junit)")
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
		options := getDeploymentOptions(cmd)
		srcDir := sourceDir(cmd)
		reportFormat, _ := cmd.Flags().GetString("reporttype")
		runImport(srcDir, options, reportFormat)
	},
	Args: cobra.MaximumNArgs(0),
}

var (
	testsToRun                 metaName
	quiet                      bool
	verbose                    bool
	exitCodeOnTestFailure      bool
	suppressUnexpectedError    bool
	ignoreCodeCoverageWarnings bool
)

func sourceDir(cmd *cobra.Command) string {
	directory, _ := cmd.Flags().GetString("directory")

	wd, _ := os.Getwd()
	var dir string
	usr, err := user.Current()

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
	return root
}

func runImport(root string, options ForceDeployOptions, reportFormat string) {
	files := make(ForceMetadataFiles)
	if _, err := os.Stat(filepath.Join(root, "package.xml")); os.IsNotExist(err) {
		ErrorAndExit(" \n" + filepath.Join(root, "package.xml") + "\ndoes not exist")
	}

	err := filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
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

	if quiet {
		var l quietLogger
		lib.Log = l
	}

	startTime := time.Now()
	deployId, err := force.Metadata.StartDeploy(files, options)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	stopDeployUponSignal(force, deployId)
	var result ForceCheckDeploymentStatusResult
	for {
		result, err = force.Metadata.CheckDeployStatus(deployId)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		if result.Done {
			break
		}
		Log.Info(result)
		time.Sleep(5000 * time.Millisecond)
	}
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	junitOutput := reportFormat == "junit"

	if suppressUnexpectedError {
		filteredComponentFailures := result.Details.ComponentFailures[:0]
		for _, f := range result.Details.ComponentFailures {
			if !strings.HasPrefix(f.Problem, `An unexpected error occurred. Please include this ErrorId`) {
				filteredComponentFailures = append(filteredComponentFailures, f)
			}
		}
		result.Details.ComponentFailures = filteredComponentFailures
	}

	switch {
	case quiet:
	case junitOutput:
		output, err := result.ToJunit(duration.Seconds())
		if err != nil {
			ErrorAndExit(err.Error())
		}
		fmt.Println(output)
		if result.HasComponentFailures() || (result.HasTestFailures() && exitCodeOnTestFailure) || !result.Success {
			os.Exit(1)
		}
		return
	default:
		output := result.ToString(duration.Seconds(), verbose)
		fmt.Println(output)

		codeCoverageWarnings := result.Details.RunTestResult.CodeCoverageWarnings
		if !ignoreCodeCoverageWarnings && len(codeCoverageWarnings) > 0 {
			fmt.Printf("\nCode Coverage Warnings - %d\n", len(codeCoverageWarnings))
			for _, warning := range codeCoverageWarnings {
				fmt.Printf("\n %s: %s\n", warning.Name, warning.Message)
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
			ErrorAndExit(err.Error())
		}
		if !quiet {
			fmt.Printf("Imported from %s\n", root)
		}
	}
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
	runAllTests, _ := cmd.Flags().GetBool("runalltests")
	if runAllTests {
		deploymentOptions.TestLevel = "RunAllTestsInOrg"
	}
	deploymentOptions.RunTests = testsToRun
	return deploymentOptions
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

type quietLogger struct{}

func (l quietLogger) Info(args ...interface{}) {
}
