package main

import ()

var cmdGet = &Command{
	Run:   runGet,
	Usage: "get <type> <id>",
	Short: "Display a record",
	Long: `
Display a record

Examples:

  force get User 00Ei000000000000
`,
}

func runGet(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) != 2 {
		ErrorAndExit("must specify type and id")
	}
	object, err := force.Get(args[0], args[1])
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		DisplayForceObject(object)
	}
}
