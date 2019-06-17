package command

import (
	"fmt"

	. "github.com/ForceCLI/force/lib"
)

//Dood, what
var cmdVersion = &Command{
	Run:   runVersion,
	Usage: "version",
	Short: "Display current version",
	Long: `
Display current version

Examples:

  force version
`,
	MaxExpectedArgs: 0,
}

func init() {
}

func runVersion(cmd *Command, args []string) {
	fmt.Println(Version)
}
