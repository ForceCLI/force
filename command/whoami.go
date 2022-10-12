package command

import (
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
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
		me, err := force.Whoami()
		if err != nil {
			ErrorAndExit(err.Error())
		} else if len(args) == 0 {
			DisplayForceRecord(me)
		}
	},
}
