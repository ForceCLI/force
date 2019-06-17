package main

import (
	"os"

	"github.com/ForceCLI/force/command"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

func main() {
	args := os.Args[1:]
	if len(args) < 1 {
		command.Usage()
	}

	for _, cmd := range command.Commands {
		if cmd.Name() == args[0] && cmd.Run != nil {
			cmd.Flag.Usage = func() {
				cmd.PrintUsage()
			}
			if err := cmd.Flag.Parse(args[1:]); err != nil {
				os.Exit(2)
			}
			if cmd.MaxExpectedArgs >= 0 && len(cmd.Flag.Args()) > cmd.MaxExpectedArgs {
				cmd.InvalidInvokation(args)
				os.Exit(2)
			}

			_, err := ActiveCredentials(false)
			if err != nil {
				ErrorAndExit(err.Error())
			}

			cmd.Run(cmd, cmd.Flag.Args())
			return
		}
	}
	command.Usage()
}
