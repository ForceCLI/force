package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

var cmdImport = &Command{
	Usage: "import [deployment options]",
	Short: "Import metadata from a local directory",
	Long: `
Import metadata from a local directory

Deployment Options
  -rollbackonerror, -r    Indicates whether any failure causes a complete rollback
  -runalltests, -t        If set all Apex tests defined in the organization are run (equivalent to -l RunAllTestsInOrg)
  -checkonly, -c          Indicates whether classes and triggers are saved during deployment
  -purgeondelete, -p      If set the deleted components are not stored in recycle bin
  -allowmissingfiles, -m  Specifies whether a deploy succeeds even if files missing
  -autoupdatepackage, -u  Auto add files to the package if missing
  -test                   Run tests in class (implies -l RunSpecifiedTests)
  -testlevel, -l          Set test level (NoTestRun, RunSpecifiedTests, RunLocalTests, RunAllTestsInOrg)
  -ignorewarnings, -i     Indicates if warnings should fail deployment or not
  -directory, -d 		  Path to the package.xml file to import
  -verbose, -v 			  Provide detailed feedback on operation

Examples:

  force import

  force import -directory=my_metadata -c -r -v

  force import -checkonly -runalltests
`,
}

var (
	testsToRun            metaName
	rollBackOnErrorFlag   = cmdImport.Flag.Bool("rollbackonerror", false, "set roll back on error")
	runAllTestsFlag       = cmdImport.Flag.Bool("runalltests", false, "set run all tests")
	testLevelFlag         = cmdImport.Flag.String("testLevel", "NoTestRun", "set test level")
	checkOnlyFlag         = cmdImport.Flag.Bool("checkonly", false, "set check only")
	purgeOnDeleteFlag     = cmdImport.Flag.Bool("purgeondelete", false, "set purge on delete")
	allowMissingFilesFlag = cmdImport.Flag.Bool("allowmissingfiles", false, "set allow missing files")
	autoUpdatePackageFlag = cmdImport.Flag.Bool("autoupdatepackage", false, "set auto update package")
	ignoreWarningsFlag    = cmdImport.Flag.Bool("ignorewarnings", false, "set ignore warnings")
	directory             = cmdImport.Flag.String("directory", "metadata", "relative path to package.xml")
	verbose               = cmdImport.Flag.Bool("verbose", false, "give more verbose output")
)

func init() {
	cmdImport.Run = runImport
	cmdImport.Flag.BoolVar(verbose, "v", false, "give more verbose output")
	cmdImport.Flag.BoolVar(rollBackOnErrorFlag, "r", false, "set roll back on error")
	cmdImport.Flag.BoolVar(runAllTestsFlag, "t", false, "set run all tests")
	cmdImport.Flag.StringVar(testLevelFlag, "l", "NoTestRun", "set test level")
	cmdImport.Flag.BoolVar(checkOnlyFlag, "c", false, "set check only")
	cmdImport.Flag.BoolVar(purgeOnDeleteFlag, "p", false, "set purge on delete")
	cmdImport.Flag.BoolVar(allowMissingFilesFlag, "m", false, "set allow missing files")
	cmdImport.Flag.BoolVar(autoUpdatePackageFlag, "u", false, "set auto update package")
	cmdImport.Flag.BoolVar(ignoreWarningsFlag, "i", false, "set ignore warnings")
	cmdImport.Flag.StringVar(directory, "d", "metadata", "relative path to package.xml")
	cmdImport.Flag.Var(&testsToRun, "test", "Test(s) to run")
}

func runImport(cmd *Command, args []string) {
	if len(args) > 0 {
		ErrorAndExit("Unrecognized argument: " + args[0])
	}

	wd, _ := os.Getwd()
	usr, err := user.Current()
	var dir string

	//Manually handle shell expansion short cut
	if err != nil {
		if strings.HasPrefix(*directory, "~") {
			ErrorAndExit("Cannot determine tilde expansion, please use relative or absolute path to directory.")
		} else {
			dir = *directory
		}
	} else {
		if strings.HasPrefix(*directory, "~") {
			dir = strings.Replace(*directory, "~", usr.HomeDir, 1)
		} else {
			dir = *directory
		}
	}

	root := filepath.Join(wd, dir)

	// Check for absolute path
	if filepath.IsAbs(dir) {
		root = dir
	}

	force, _ := ActiveForce()
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
	DeploymentOptions.AllowMissingFiles = *allowMissingFilesFlag
	DeploymentOptions.AutoUpdatePackage = *autoUpdatePackageFlag
	DeploymentOptions.CheckOnly = *checkOnlyFlag
	DeploymentOptions.IgnoreWarnings = *ignoreWarningsFlag
	DeploymentOptions.PurgeOnDelete = *purgeOnDeleteFlag
	DeploymentOptions.RollbackOnError = *rollBackOnErrorFlag
	DeploymentOptions.TestLevel = *testLevelFlag
	if *runAllTestsFlag {
		DeploymentOptions.TestLevel = "RunAllTestsInOrg"
	}
	DeploymentOptions.RunTests = testsToRun

	result, err := force.Metadata.Deploy(files, DeploymentOptions)
	problems := result.Details.ComponentFailures
	successes := result.Details.ComponentSuccesses
	if err != nil {
		ErrorAndExit(err.Error())
	}

	fmt.Printf("\nFailures - %d\n", len(problems))
	if *verbose {
		for _, problem := range problems {
			if problem.FullName == "" {
				fmt.Println(problem.Problem)
			} else {
				fmt.Printf("%s: %s\n", problem.FullName, problem.Problem)
			}
		}
	}

	fmt.Printf("\nSuccesses - %d\n", len(successes))
	if *verbose {
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
	fmt.Printf("Imported from %s\n", root)
}
