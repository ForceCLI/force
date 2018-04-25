package command

import (
	"fmt"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdPackage = &Command{
	Run:   runPackage,
	Usage: "package",
	Short: "Manage installed packages",
	Long: `
Manage installed packages

Usage:

  force package install <namespace> <version> [password]
`,
}

func runPackage(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.PrintUsage()
	} else {
		switch args[0] {
		case "install":
			runInstallPackage(args[1:])
		default:
			ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func runInstallPackage(args []string) {
	if len(args) < 2 {
		ErrorAndExit("must specify package namespace and version")
	}
	force, _ := ActiveForce()
	packageNamespace := args[0]
	version := args[1]
	password := ""
	if len(args) > 2 {
		password = args[2]
	}
	if err := force.Metadata.InstallPackage(packageNamespace, version, password); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Package instaled")
}
