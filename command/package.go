package command

import (
	"fmt"

	. "github.com/ForceCLI/force/error"
	"github.com/spf13/cobra"
)

func init() {
	packageInstallCmd.Flags().BoolP("activate", "A", false, "keep the isActive state of any Remote Site Settings (RSS) and Content Security Policies (CSP) in package")
	packageInstallCmd.Flags().StringP("password", "p", "", "password for package")

	packageCmd.AddCommand(packageInstallCmd)
	RootCmd.AddCommand(packageCmd)
}

var packageCmd = &cobra.Command{
	Use:   "package",
	Short: "Manage installed packages",
}

var packageInstallCmd = &cobra.Command{
	Use:   "install [flags] <namespace> <version>",
	Short: "Installed packages",
	Args:  cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		activateRSS, _ := cmd.Flags().GetBool("activate")
		password, _ := cmd.Flags().GetString("password")
		packageNamespace := args[0]
		version := args[1]
		if len(args) > 2 {
			fmt.Println("Warning: Deprecated use of [password] argument.  Use --password flag.")
			password = args[2]
		}
		runInstallPackage(packageNamespace, version, password, activateRSS)
	},
}

func runInstallPackage(packageNamespace string, version string, password string, activateRSS bool) {
	if err := force.Metadata.InstallPackageWithRSS(packageNamespace, version, password, activateRSS); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Package installed")
}
