package main

import (
	"os"
)

var commands = []*Command{
	cmdActive,
	cmdApex,
	cmdApiVersion,
	cmdAura,
	cmdBigObject,
	cmdBulk,
	cmdCreate,
	cmdDataPipe,
	cmdDescribe,
	cmdEventLogFile,
	cmdExport,
	cmdFetch,
	cmdField,
	cmdHelp,
	cmdImport,
	cmdLimits,
	cmdLog,
	cmdLogin,
	cmdLogins,
	cmdLogout,
	cmdNotifySet,
	cmdOauth,
	cmdPassword,
	cmdPush,
	cmdQuery,
	cmdRecord,
	cmdRest,
	cmdSecurity,
	cmdSobject,
	cmdTest,
	cmdTrace,
	cmdUpdate,
	cmdUseDXAuth,
	cmdVersion,
	cmdWhoami,
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
			creds, err := ActiveCredentials(false)
			if err != nil {
				ErrorAndExit(err.Error())
			}
			apiVersionNumber = creds.ApiVersion

			cmd.Run(cmd, cmd.Flag.Args())
			return
		}
	}
	usage()
}
