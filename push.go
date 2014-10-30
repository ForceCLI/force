package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var cmdPush = &Command{
	Run:   runPush,
	Usage: "push (<metadata> <name> | <file>...)",
	Short: "Deploy artifact from a local directory",
	Long: `
Deploy artifact from a local directory
<metadata>: Accepts either actual directory name or Metadata type

Examples:
  force push classes MyClass
  force push ApexClass MyClass
  force push src/classes/MyClass.cls
`,
}

var namePaths = make(map[string]string)
var byName = false

func argIsFile(fpath string) bool {
	if _, err := os.Stat(fpath); err != nil {
		return false
	}
	return true
}

func runPush(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
		return
	}

	if argIsFile(args[0]) {
		pushByPaths(args)
		return
	}

	if len(args) == 2 {
		// If arg[0] is already path or meta, the method will return arg[0]
		objPath := getPathForMeta(args[0])
		objName := args[1]
		pushByName(objPath, objName)
		return
	}

	fmt.Println("Could not find file or determine metadata")

	// If we got here, something is not valid
	cmd.printUsage()
}

func pushByName(objPath string, objName string) {
	byName = true

	root, err := GetSourceDir("")
	ExitIfNoSourceDir(err)

	if _, err := os.Stat(filepath.Join(root, objPath)); os.IsNotExist(err) {
		ErrorAndExit("Folder " + objPath + " not found, must specify valid metadata")
	}

	// Find file by walking directory and ignoring extension
	found := false
	var fpath string
	err = filepath.Walk(filepath.Join(root, objPath), func(path string, f os.FileInfo, err error) error {
		if f.Mode().IsRegular() {
			fname := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
			if strings.EqualFold(fname, objName) {
				found = true
				fpath = filepath.Join(root, objPath, f.Name())
			}
		}
		return nil
	})
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if !found {
		ErrorAndExit("Could not find " + objName + " in " + objPath)
	}

	pushByPath(fpath)
}

func pushByPath(fpath string) {
	pushByPaths([]string{fpath})
}

// Push metadata object by path to a file
func pushByPaths(fpaths []string) {
	pb := NewPushBuilder()

	var badPaths []string
	for _, fpath := range fpaths {
		name, err := pb.AddFile(fpath)
		if err != nil {
			badPaths = append(badPaths, fpath)
		} else {
			// Store paths by name for error messages
			namePaths[name] = fpath
		}
	}

	if len(badPaths) == 0 {
		deployFiles(pb.ForceMetadataFiles())
	} else {
		ErrorAndExit("Could not add the following files:\n {}", strings.Join(badPaths, "\n"))
	}
}

func deployFiles(files ForceMetadataFiles) {
	force, _ := ActiveForce()
	var DeploymentOptions ForceDeployOptions
	successes, problems, err := force.Metadata.Deploy(files, DeploymentOptions)
	if err != nil {
		ErrorAndExit(err.Error())
	}
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
			fmt.Printf("%s\n\tstatus: %s\n\tid=%s\n", success.FullName, verb, success.Id)
		}
	}

	// Handle notifications
	notifySuccess("push", len(problems) == 0)
}
