package main

import ()

var cmdWhoami = &Command{
	Run:   runWhoami,
	Usage: "whoami",
	Short: "Show information about the active account",
	Long: `
Show information about the active account

Examples:

  force whoami
`,
}

func runWhoami(cmd *Command, args []string) {
	force, _ := ActiveForce()
	me, err := force.Whoami()
	if err != nil {
		ErrorAndExit(err.Error())
	} else if len(args) == 0 {
		DisplayForceRecord(me)
	}
}
