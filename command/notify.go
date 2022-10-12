package command

import (
	"fmt"
	"strconv"

	"github.com/ForceCLI/force/desktop"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(notifyCmd)
}

var notifyCmd = &cobra.Command{
	Use:   "notify (true | false)",
	Short: "Should notifications be used",
	Long: `
Determines if notifications should be used
`,
	Args:                  cobra.MaximumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		shouldNotify := true
		if len(args) == 0 {
			shouldNotify = desktop.GetShouldNotify()
			fmt.Println("Show notifications: " + strconv.FormatBool(shouldNotify))
			return
		}

		shouldNotify, err = strconv.ParseBool(args[0])
		if err != nil {
			fmt.Println("Expecting a boolean parameter.")
		}

		desktop.SetShouldNotify(shouldNotify)
	},
}
