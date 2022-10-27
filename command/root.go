package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var (
	account     string
	_apiVersion string

	force *Force
)

func init() {
	// Provide backwards compatibility for single-dash flags
	args := os.Args[1:]
	for i, arg := range args {
		if len(arg) > 2 && strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
			args[i] = fmt.Sprintf("-%s", arg)
		}
	}
	RootCmd.SetArgs(args)
	RootCmd.PersistentFlags().StringVarP(&account, "account", "a", "", "account `username` to use")
	RootCmd.PersistentFlags().StringVarP(&_apiVersion, "apiversion", "V", "", "API version to use")
}

var RootCmd = &cobra.Command{
	Use:   "force",
	Short: "force CLI",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		switch cmd.Use {
		case "force", "login":
		default:
			initializeSession()
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		os.Exit(1)
	},
	DisableFlagsInUseLine: true,
}

func initializeSession() {
	var err error
	if account != "" {
		force, err = GetForce(account)
	} else {
		force, err = ActiveForce()
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if _apiVersion != "" {
		err := SetApiVersion(_apiVersion)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type quietLogger struct{}

func (l quietLogger) Info(args ...interface{}) {
}
