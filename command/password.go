package command

import (
	"fmt"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
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
	MaxExpectedArgs: -1,
}

func init() {
}

func runPassword(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.PrintUsage()
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
	run(args, 1, func(force *Force, recordID string) {
		object, err := force.GetPasswordStatus(recordID)
		if err != nil {
			ErrorAndExit(err.Error())
		} else {
			fmt.Printf("\nPassword is expired: %t\n\n", object.IsExpired)
		}
	})
}

func runPasswordReset(args []string) {
	run(args, 1, func(force *Force, recordID string) {
		object, err := force.ResetPassword(recordID)
		if err != nil {
			ErrorAndExit(err.Error())
		} else {
			fmt.Printf("\nNew password is: %s\n\n", object.NewPassword)
		}
	})
}

func runPasswordChange(args []string) {
	run(args, 2, func(force *Force, recordID string) {
		newPass := make(map[string]string)
		newPass["NewPassword"] = args[1]
		_, err, emessages := force.ChangePassword(recordID, newPass)
		if err != nil {
			ErrorAndExit(err.Error(), emessages[0].ErrorCode)
		} else {
			fmt.Println("\nPassword changed\n ")
		}
	})
}

func run(args []string, expectedArgs int, runner func(force *Force, recordID string)) {
	if len(args) != expectedArgs {
		ErrorAndExit("must specify user name")
	}

	force, _ := ActiveForce()
	records, err := force.Query(fmt.Sprintf("select Id From User Where UserName = '%s'", args[0]))
	if err != nil {
		ErrorAndExit(err.Error())
	} else if len(records.Records) > 0 {
		runner(force, records.Records[0]["Id"].(string))
	} else {
		ErrorAndExit("user not found")
	}
}
