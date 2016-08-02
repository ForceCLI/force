package main

import (
	"github.com/heroku/force/util"
)

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
		util.ErrorAndExit(err.Error())
	} else if len(args) == 0 {
		DisplayForceRecord(me)
	}
}
