package main

import (
	"os"
)

var commands = []*Command{
	cmdLogin,
	cmdLogout,
	cmdLogins,
	cmdActive,
	cmdWhoami,
	cmdDescribe,
	cmdSobject,
	cmdBigObject,
	cmdField,
	cmdRecord,
	cmdBulk,
	cmdFetch,
	cmdImport,
	cmdExport,
	cmdQuery,
	cmdApex,
	cmdTrace,
	cmdLog,
	cmdEventLogFile,
	cmdOauth,
	cmdTest,
	cmdSecurity,
	cmdVersion,
	cmdUpdate,
	cmdPush,
	cmdAura,
	cmdPassword,
	cmdNotifySet,
	cmdLimits,
	cmdHelp,
	cmdDataPipe,
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
