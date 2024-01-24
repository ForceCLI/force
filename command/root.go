package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/config"
	forceConfig "github.com/ForceCLI/force/config"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var (
	account     string
	configName  string
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
	RootCmd.PersistentFlags().StringVar(&configName, "config", "", "config directory to use (default: .force)")
	RootCmd.PersistentFlags().StringVarP(&_apiVersion, "apiversion", "V", "", "API version to use")
}

var RootCmd = &cobra.Command{
	Use:   "force",
	Short: "force CLI",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initializeConfig()
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

func initializeConfig() {
	if configName != "" {
		forceConfig.Config = config.NewConfig(strings.TrimPrefix(configName, "."))
		fmt.Println("Setting config to", forceConfig.Config.Base)
	}
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
