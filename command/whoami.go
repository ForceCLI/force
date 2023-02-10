package command

import (
	"fmt"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	whoamiCmd.Flags().BoolP("username", "u", false, "Get username of the active account")
	whoamiCmd.Flags().BoolP("id", "i", false, "Get user id of the active account")
	whoamiCmd.MarkFlagsMutuallyExclusive("username", "id")
	RootCmd.AddCommand(whoamiCmd)
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show information about the active account",
	Example: `
  force whoami
`,
	Args: cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		displayUsername, _ := cmd.Flags().GetBool("username")
		displayUserId, _ := cmd.Flags().GetBool("id")
		me, err := force.Whoami()
		if err != nil {
			ErrorAndExit(err.Error())
		}
		if displayUsername {
			fmt.Println(me["Username"])
		} else if displayUserId {
			fmt.Println(me["Id"])
		} else {
			DisplayForceRecord(me)
		}
	},
}
