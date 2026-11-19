package main

import (
	"os"

	"github.com/ForceCLI/force/command"
	"github.com/ForceCLI/force/sf"
)

func main() {
	if sf.InvokedAsSf(os.Args[0]) {
		sf.SetInvocation(os.Args[0], false)
		sf.Execute()
		return
	}
	if args, ok := sf.StripModeFlag(os.Args[1:]); ok {
		sf.SetInvocation(os.Args[0], true)
		os.Exit(sf.Run(args, os.Stdout, os.Stderr))
	}
	command.Execute()
}
