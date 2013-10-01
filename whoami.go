package main

import (
	"strings"
)

var cmdWhoami = &Command{
	Run:   runWhoami,
	Usage: "whoami [account]",
	Short: "Show information about the active account",
	Long: `
Show information about the active account

Examples:

  force whoami
`,
}

func runWhoami(cmd *Command, args []string) {
	force, _ := ActiveForce()
	parts := strings.Split(force.Credentials.Id, "/")
	user, err := force.GetRecord("User", parts[len(parts)-1])
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		DisplayForceRecord(user)
	}
}
