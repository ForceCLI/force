package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var cmdImport = &Command{
	Usage: "import [deployment options] [dir]",
	Short: "Import metadata from a local directory",
	Long: `
Import metadata from a local directory

Deployment Options
  -rollbackonerror    Indicates whether any failure causes a complete rollback
  -runalltests        If set all Apex tests defined in the organization are run
  -checkonly          Indicates whether classes and triggers are saved during deployment
  -purgeondelete      If set the deleted components are not stored in recycle bin
  -allowmissingfiles  Specifies whether a deploy succeeds even if files missing
  -autoupdatepackage  Auto add files to the package if missing
  -ignorewarnings     Indicates if warnings should fail deployment or not
  -verbose
Examples:

  force import

  force import org/schema

  force import -checkonly -runalltests
`,
}

var (
	rollBackOnErrorFlag   = cmdImport.Flag.Bool("rollbackonerror", false, "set roll back on error")
	runAllTestsFlag       = cmdImport.Flag.Bool("runalltests", false, "set run all tests")
	checkOnlyFlag         = cmdImport.Flag.Bool("checkonly", false, "set check only")
	purgeOnDeleteFlag     = cmdImport.Flag.Bool("purgeondelete", false, "set purge on delete")
	allowMissingFilesFlag = cmdImport.Flag.Bool("allowmissingfiles", false, "set allow missing files")
	autoUpdatePackageFlag = cmdImport.Flag.Bool("autoupdatepackage", false, "set auto update package")
	ignoreWarningsFlag    = cmdImport.Flag.Bool("ignorewarnings", false, "set ignore warnings")
	verbose				  = cmdImport.Flag.Bool("verbose", false, "give more verbose output")
)

func init() {
	cmdImport.Run = runImport
	cmdImport.Flag.BoolVar(verbose, "v", false, "give more verbose output")
	cmdImport.Flag.BoolVar(rollBackOnErrorFlag, "r", false, "set roll back on error")
	cmdImport.Flag.BoolVar(runAllTestsFlag, "t", false, "set run all tests")
	cmdImport.Flag.BoolVar(checkOnlyFlag, "c", false, "set check only")
	cmdImport.Flag.BoolVar(purgeOnDeleteFlag, "p", false, "set purge on delete")
	cmdImport.Flag.BoolVar(allowMissingFilesFlag, "m", false, "set allow missing files")
	cmdImport.Flag.BoolVar(autoUpdatePackageFlag, "u", false, "set auto update package")
	cmdImport.Flag.BoolVar(ignoreWarningsFlag, "i", false, "set ignore warnings")
}

func runImport(cmd *Command, args []string) {
	fmt.Println(args)
	if len(args) > 0 {
		if err := cmd.Flag.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
	}

	wd, _ := os.Getwd()
	root := filepath.Join(wd, "metadata")
	if len(args) >= 1 {
		root, _ = filepath.Abs(args[0])
	}

	force, _ := ActiveForce()
	files := make(ForceMetadataFiles)
	if _, err := os.Stat(filepath.Join(root, "package.xml")); os.IsNotExist(err) {
		ErrorAndExit("No 'metadata' directory found and other directory specified")
	}
	err := filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
		if f.Mode().IsRegular() {
			if f.Name() != ".DS_Store" {
				data, err := ioutil.ReadFile(path)
				if err != nil {
					ErrorAndExit(err.Error())
				}
				files[strings.Replace(path, fmt.Sprintf("%s/", root), "", -1)] = data
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
	DeploymentOptions.RunAllTests = *runAllTestsFlag
	successes, problems, err := force.Metadata.Deploy(files, DeploymentOptions)
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
