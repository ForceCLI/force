package command

import (
	"fmt"
	"strconv"

	"github.com/ForceCLI/force/desktop"
)

var cmdNotifySet = &Command{
	Run:   notifySet,
	Usage: "notify (true | false)",
	Short: "Should notifications be used",
	Long: `
Determines if notifications should be used
`,
}

func notifySet(cmd *Command, args []string) {
	var err error
	shouldNotify := true
	if len(args) == 0 {
		shouldNotify = desktop.GetShouldNotify()
		fmt.Println("Show notifications: " + strconv.FormatBool(shouldNotify))
	} else if len(args) == 1 {
		shouldNotify, err = strconv.ParseBool(args[0])
		if err != nil {
			fmt.Println("Expecting a boolean parameter.")
		}
	} else {
		fmt.Println("Expecting only one parameter. true/false")
	}

	desktop.SetShouldNotify(shouldNotify)
}
