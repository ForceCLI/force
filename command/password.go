package command

import (
	"fmt"

	. "github.com/ForceCLI/force/error"
	"github.com/spf13/cobra"
)

var passwordCmd = &cobra.Command{
	Use:   "password <command> [user name] [new password]",
	Short: "See password status or reset password",
	Long: `
See password status or reset/change password
`,
	Example: `
  force password status joe@org.com
  force password reset joe@org.com
  force password change joe@org.com '$uP3r$3cure'
`,
	DisableFlagsInUseLine: true,
}

var passwordStatusCmd = &cobra.Command{
	Use:   "status [user name]",
	Short: "See password status",
	Example: `
  force password status joe@org.com

`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runPasswordStatus(args[0])
	},
	DisableFlagsInUseLine: true,
}

var passwordResetCmd = &cobra.Command{
	Use:   "reset [user name]",
	Short: "Reset password",
	Example: `
  force password reset joe@org.com
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runPasswordReset(args[0])
	},
	DisableFlagsInUseLine: true,
}

var passwordChangeCmd = &cobra.Command{
	Use:   "change [user name] [new password]",
	Short: "Set new password",
	Example: `
  force password change joe@org.com '$uP3r$3cure'

`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		runPasswordChange(args[0], args[1])
	},
	DisableFlagsInUseLine: true,
}

func init() {
	passwordCmd.AddCommand(passwordStatusCmd)
	passwordCmd.AddCommand(passwordResetCmd)
	passwordCmd.AddCommand(passwordChangeCmd)
	RootCmd.AddCommand(passwordCmd)
}

func runPasswordStatus(username string) {
	id := getUserId(username)
	object, err := force.GetPasswordStatus(id)
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		fmt.Printf("\nPassword is expired: %t\n\n", object.IsExpired)
	}
}

func runPasswordReset(username string) {
	id := getUserId(username)
	object, err := force.ResetPassword(id)
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		fmt.Printf("\nNew password is: %s\n\n", object.NewPassword)
	}
}

func runPasswordChange(username, pass string) {
	id := getUserId(username)
	newPass := make(map[string]string)
	newPass["NewPassword"] = pass
	_, err, emessages := force.ChangePassword(id, newPass)
	if err != nil {
		ErrorAndExit(err.Error(), emessages[0].ErrorCode)
	} else {
		fmt.Println("\nPassword changed\n ")
	}
}

func getUserId(username string) string {
	records, err := force.Query(fmt.Sprintf("SELECT Id FROM User WHERE UserName = '%s'", username))
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if len(records.Records) > 0 {
		return records.Records[0]["Id"].(string)
	}
	ErrorAndExit("user not found")
	return ""
}
