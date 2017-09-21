package main

import (
	"os"

	"github.com/heroku/force/command"
	. "github.com/heroku/force/error"
	. "github.com/heroku/force/lib"
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
			creds, err := ActiveCredentials(false)
			if err != nil {
				ErrorAndExit(err.Error())
			}
			SetApiVersion(creds.ApiVersion)

			cmd.Run(cmd, cmd.Flag.Args())
			return
		}
	}
	command.Usage()
}
