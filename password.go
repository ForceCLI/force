package main

import (
	"fmt"
)

var cmdPassword = &Command{
	Run:   runPassword,
	Usage: "password <command> [user name] [new password]",
	Short: "See password status or reset password",
	Long: `
See password status or reset/change password

Examples:

  force password status joe@org.com

  force password reset joe@org.com

  force password change joe@org.com $uP3r$3cure
  
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
		case "change":
			runPasswordChange(args[1:])
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
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		object, err := force.GetPasswordStatus(records[0]["Id"].(string))
		if err != nil {
			ErrorAndExit(err.Error())
		} else {
			fmt.Printf("\nPassword is expired: %t\n\n", object.IsExpired)
		}
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
		fmt.Printf("\nNew password is: %s\n\n", object.NewPassword)
	}
}

func runPasswordChange(args []string) {
	if len(args) != 2 {
		ErrorAndExit("must specify user name")
	}
	force, _ := ActiveForce()
	records, err := force.Query(fmt.Sprintf("select Id From User Where UserName = '%s'", args[0]))
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		attrs := ParseArgumentAttrs(args[1:])
		_, err := force.ChangePassword(records[0]["Id"].(string), attrs)
		if err != nil {
			ErrorAndExit(err.Error())
		} else {
			fmt.Println("\nPassword changed\n")
		}
	}
}
