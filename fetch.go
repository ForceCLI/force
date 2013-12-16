package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

var cmdFetch = &Command{
	Run:   runFetch,
	Usage: "fetch <type> [<artifact name>]",
	Short: "Export specified artifact(s) to a local directory",
	Long: `
Export specified artifact(s) to a local directory

Examples

  force fetch CustomObject Book__c Author__c
`,
}

func runFetch(cmd *Command, args []string) {
	wd, _ := os.Getwd()
	root := filepath.Join(wd, "metadata")
	if len(args) < 1 {
		ErrorAndExit("must specify object type and/or object name")
	}

	artifactType := args[0]
	query := ForceMetadataQuery{}
	if len(args) >= 2 {
		newargs := args[1:]
		for artifactNames := range newargs {
			mq := ForceMetadataQueryElement{artifactType, newargs[artifactNames]}
			query = append(query, mq)
		}
	} else {
		mq := ForceMetadataQueryElement{artifactType, "*"}
		query = append(query, mq)
	}

	force, _ := ActiveForce()
	files, err := force.Metadata.Retrieve(query)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	for name, data := range files {
		file := filepath.Join(root, name)
		dir := filepath.Dir(file)
		if err := os.MkdirAll(dir, 0755); err != nil {
			ErrorAndExit(err.Error())
		}
		if err := ioutil.WriteFile(filepath.Join(root, name), data, 0644); err != nil {
			ErrorAndExit(err.Error())
		}
	}
	fmt.Printf("Exported to %s\n", root)
}
