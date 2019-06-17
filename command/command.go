package command

import (
	"flag"
	"fmt"
	"strings"
)

var Commands = []*Command{
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
	cmdOpen,
	cmdPackage,
	cmdPassword,
	cmdPush,
	cmdQuery,
	cmdQuickDeploy,
	cmdRecord,
	cmdRest,
	cmdSecurity,
	cmdSobject,
	cmdTest,
	cmdTrace,
	cmdUseDXAuth,
	cmdVersion,
	cmdWhoami,
}

type Command struct {
	// args does not include the command name
	Run  func(cmd *Command, args []string)
	Flag flag.FlagSet

	Usage           string // first word is the command name
	Short           string // `forego help` output
	Long            string // `forego help cmd` output
	MaxExpectedArgs int    // the maximum number of arguments this command expects, -1 if it is variadic
}

func (c *Command) PrintUsage() {
	if c.Runnable() {
		fmt.Printf("Usage: force %s\n\n", c.Usage)
	}
	fmt.Println(strings.Trim(c.Long, "\n"))
}

func (c *Command) Name() string {
	name := c.Usage
	i := strings.Index(name, " ")
	if i >= 0 {
		name = name[:i]
	}
	return name
}

func (c *Command) Runnable() bool {
	return c.Run != nil
}

func (c *Command) List() bool {
	return c.Short != ""
}

func (c *Command) InvalidInvokation(args []string) {
	fmt.Printf("Invalid invokation: force %s\n\n", strings.Join(args, " "))
	c.PrintUsage()
}
