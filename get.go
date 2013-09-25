package main

import (
	"fmt"
	"os"
)

var cmdGet = &Command{
	Run:   runGet,
	Usage: "get <type> <id>",
	Short: "Get a force.com object",
	Long: `
Get a force.com object

Examples:

  force get User 00Ei000000000000
`,
}

func runGet(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) != 2 {
		fmt.Println("ERROR: must specify type and id")
		os.Exit(1)
	}
	object, err := force.Get(args[0], args[1])
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
	} else {
		DisplayForceObject(object)
	}
}
