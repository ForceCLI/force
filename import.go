package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var cmdImport = &Command{
	Run:   runImport,
	Usage: "import [dir]",
	Short: "Import metadata from a local directory",
	Long: `
Import metadata from a local directory

Examples:

  force import

  force import org/schema
`,
}

func runImport(cmd *Command, args []string) {
	wd, _ := os.Getwd()
	root := filepath.Join(wd, "metadata")
	if len(args) == 1 {
		root, _ = filepath.Abs(args[0])
	}
	force, _ := ActiveForce()
	files := make(ForceMetadataFiles)
	if _, err := os.Stat(filepath.Join(root, "package.xml")); os.IsNotExist(err) {
		ErrorAndExit("Must specify a directory that contains metadata files")
	}
	err := filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
		if f.Mode().IsRegular() {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				ErrorAndExit(err.Error())
			}
			files[strings.Replace(path, fmt.Sprintf("%s/", root), "", -1)] = data
		}
		return nil
	})
	if err != nil {
		ErrorAndExit(err.Error())
	}
	problems, err := force.Metadata.Deploy(files)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	for _, problem := range problems {
		if problem.Name == "" {
			fmt.Println(problem.Problem)
		} else {
			fmt.Printf("%s: %s\n", problem.Name, problem.Problem)
		}
	}
	fmt.Printf("Imported from %s\n", root)
}