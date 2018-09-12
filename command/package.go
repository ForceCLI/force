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

  force package [options] install <namespace> <version> [password]

Options:
  -activate, -a     Keep the isActive state of any Remote Site Settings (RSS) and Content Security Policies (CSP) in package
`,
}

var (
	activateRSS bool
)

func init() {
	cmdPackage.Flag.BoolVar(&activateRSS, "a", false, "keep the isActive state of any Remote Site Settings (RSS) and Content Security Policies (CSP) in package")
	cmdPackage.Flag.BoolVar(&activateRSS, "activate", false, "keep the isActive state of any Remote Site Settings (RSS) and Content Security Policies (CSP) in package")
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
	if err := force.Metadata.InstallPackageWithRSS(packageNamespace, version, password, activateRSS); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Package instaled")
}
