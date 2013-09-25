package main

import (
	"fmt"
)

var cmdObjects = &Command{
	Run:   runObjects,
	Usage: "objects",
	Short: "List force.com objects",
	Long: `
List force.com objects

Examples:

  force objects
`,
}

func runObjects(cmd *Command, args []string) {
	force, _ := ActiveForce()
	objects, err := force.Objects()
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
	} else {
		DisplayStringSlice(objects)
	}
}
