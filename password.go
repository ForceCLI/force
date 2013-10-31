package main

import (
	"fmt"
)

var cmdPassword = &Command{
	Run:   runPassword,
	Usage: "password <command> [user name]",
	Short: "See password status or reset password",
	Long: `
See password status or reset password

Examples:

  force password status joe@org.com

  force password reset joe@org.com

`,
}

func init() {
}

func runPassword(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
	} else {
		switch args[0] {
		case "status":
			runPasswordStatus(args[1:])
		case "reset":
			runPasswordReset(args[1:])
		default:
			ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func runPasswordStatus(args []string) {
	if len(args) != 1 {
		ErrorAndExit("must specify user name")
	}
	force, _ := ActiveForce()
	records, err := force.Query(fmt.Sprintf("select Id From User Where UserName = '%s'", args[0]))
	object, err := force.GetPasswordStatus(records[0]["Id"].(string))
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		fmt.Printf("Password is expired: %t\n", object.IsExpired)
	}
}

func runPasswordReset(args []string) {
	if len(args) != 1 {
		ErrorAndExit("must specify user name")
	}
	force, _ := ActiveForce()
	records, err := force.Query(fmt.Sprintf("select Id From User Where UserName = '%s'", args[0]))
	object, err := force.ResetPassword(records[0]["Id"].(string))
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		fmt.Printf("New password is: %s\n", object.NewPassword)
	}
}

