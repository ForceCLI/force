package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var cmdPush = &Command{
	Run:   runPush,
	Usage: "push path name",
	Short: "Deploy single artifact from a local directory",
	Long: `
Deploy single artifact from a local directory

Examples:

  force push connectedApps name
`,
}

var pxml = `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>%s</members>
        <name>%s</name>
    </types>
    <version>29.0</version>
</Package>`

func runPush(cmd *Command, args []string) {
	wd, _ := os.Getwd()
	root := filepath.Join(wd, "metadata")
	//if len(args) == 1 {
	//	root, _ = filepath.Abs(args[0])
	//}
	if _, err := os.Stat(filepath.Join(root, "package.xml")); os.IsNotExist(err) {
		ErrorAndExit("Must specify a directory that contains metadata files")
	}

	if _, err := os.Stat(filepath.Join(root, args[1])); os.IsNotExist(err) {
		ErrorAndExit("Folder " + args[1] + " not found, must specify a metadata folder")
	}

	objType := args[1]

	switch args[1] {
	case "objects":
		objType = "CustomObject"
	case "flexipages":
		objType = "FlexiPage"
	case "tabs":
		objType = "CustomTab"
	default:
		ErrorAndExit("That folder type is not supported")
	}

	found := false
	err := filepath.Walk(filepath.Join(root, args[1]), func(path string, f os.FileInfo, err error) error {
		if f.Mode().IsRegular() {
			if strings.Contains(strings.ToLower(f.Name()), strings.ToLower(args[2])) {
				found = true
			}
		}
		return nil
	})
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if !found {
		ErrorAndExit("Could not find " + args[2] + " in " + args[1])
	}

	force, _ := ActiveForce()
	files := make(ForceMetadataFiles)

	err = os.Rename(filepath.Join(root, "package.xml"), filepath.Join(root, "package.copy.xml"))

	if err := ioutil.WriteFile(filepath.Join(root, "package.xml"), []byte(fmt.Sprintf(pxml, args[2], objType)), 0644); err != nil {
		ErrorAndExit(err.Error())
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}

	err = filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
		if f.Mode().IsRegular() {
			if strings.Contains(strings.ToLower(f.Name()), strings.ToLower(args[2])) ||
				strings.Contains(strings.ToLower(f.Name()), "package.xml") {
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

	successes, problems, err := force.Metadata.Deploy(files)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("\nFailures - %d\n", len(problems))
	for _, problem := range problems {
		if problem.FullName == "" {
			fmt.Println(problem.Problem)
		} else {
			fmt.Printf("ERROR with %s:\n %s\n", problem.FullName, problem.Problem)
		}
	}

	fmt.Printf("\nSuccesses - %d\n", len(successes))
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

	fmt.Printf("Pushed %s to Force.com\n", args[2])
}
