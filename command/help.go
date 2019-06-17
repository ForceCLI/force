package command

import (
	"flag"
	"fmt"
	"log"
	"os"
	"text/template"
)

var cmdHelp = &Command{
	Usage:           "help [topic]",
	Short:           "Show this help",
	Long:            `Help shows usage for a command.`,
	MaxExpectedArgs: -1,
}

func init() {
	cmdHelp.Run = runHelp // break init loop
}

func runHelp(cmd *Command, args []string) {
	if len(args) == 0 {
		PrintUsage()
		return
	}
	if len(args) != 1 {
		log.Fatal("too many arguments")
	}

	for _, cmd := range Commands {
		if cmd.Name() == args[0] {
			cmd.PrintUsage()
			return
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown help topic: %q. Run 'force help'.\n", args[0])
	os.Exit(2)
}

var usageTemplate = template.Must(template.New("usage").Parse(`
Usage: force <command> [<args>]

Available commands:{{range .Commands}}{{if .Runnable}}{{if .List}}
   {{.Name | printf "%-8s"}}  {{.Short}}{{end}}{{end}}{{end}}

Run 'force help [command]' for details.
`[1:]))

func PrintUsage() {
	usageTemplate.Execute(os.Stdout, struct {
		Commands []*Command
	}{
		Commands,
	})
}

func Usage() {
	flag.PrintDefaults()
	PrintUsage()
	os.Exit(0)
}
