package main

import (
	"os"
)

var commands = []*Command{
	cmdLogin,
	cmdLogin1,
	cmdLogout,
	cmdAccounts,
	cmdActive,
	cmdWhoami,
	cmdSobject,
	cmdField,
	cmdRecord,
	cmdExport,
	cmdFetch,
	cmdImport,
	cmdSoql,
	cmdQuery,
	cmdApex,
	cmdOauth,
	cmdVersion,
	cmdUpdate,
	cmdHelp,
	cmdPush,
	cmdPassword,
}

func main() {
	args := os.Args[1:]
	if len(args) < 1 {
		usage()
	}

	for _, cmd := range commands {
		if cmd.Name() == args[0] && cmd.Run != nil {
			cmd.Flag.Usage = func() {
				cmd.printUsage()
			}
			if err := cmd.Flag.Parse(args[1:]); err != nil {
				os.Exit(2)
			}
			cmd.Run(cmd, cmd.Flag.Args())
			return
		}
	}
	usage()
}
