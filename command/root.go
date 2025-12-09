package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

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

	RootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		initializeConfig()
		current := cmd
		for current.Parent() != nil && current.Parent() != RootCmd {
			current = current.Parent()
		}
		isLoginScratch := current.Name() == "login" && cmd.Name() == "scratch"
		if isLoginScratch && account != "" {
			if err := SetActiveLogin(account); err != nil {
				ErrorAndExit(err.Error())
			}
		}
		switch current.Name() {
		case "force", "completion", "usedxauth", "logins":
		case "login":
			if isLoginScratch {
				initializeSession()
			}
		default:
			initializeSession()
		}
	}
}

var RootCmd = &cobra.Command{
	Use:   "force",
	Short: "force CLI",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		os.Exit(1)
	},
	DisableFlagsInUseLine: true,
}

func initializeConfig() {
	if configName != "" {
		customConfig := strings.TrimSpace(configName)
		if customConfig == "" {
			return
		}

		isPath := strings.ContainsAny(customConfig, string(os.PathSeparator))
		if !isPath && strings.ContainsAny(customConfig, "/\\") {
			isPath = true
		}
		if strings.HasPrefix(customConfig, "~") {
			isPath = true
		}
		if strings.HasPrefix(customConfig, ".") {
			if customConfig == "." || customConfig == ".." {
				isPath = true
			} else if len(customConfig) > 1 {
				next := customConfig[1]
				if next == '/' || next == '\\' {
					isPath = true
				}
			}
		}
		if !isPath {
			if vol := filepath.VolumeName(customConfig); vol != "" && len(customConfig) > len(vol) {
				isPath = true
			}
		}

		var err error
		if isPath {
			err = forceConfig.UseConfigDirectory(customConfig)
		} else {
			forceConfig.UseConfigBase(customConfig)
		}
		if err != nil {
			ErrorAndExit(err.Error())
		}
		fmt.Println("Setting config to", forceConfig.Config.GlobalRoot())
	}
}

func envSession() *Force {
	token := os.Getenv("SF_ACCESS_TOKEN")
	instance := os.Getenv("SF_INSTANCE_URL")
	if token == "" || instance == "" {
		return nil
	}
	creds := &ForceSession{
		AccessToken: token,
		InstanceUrl: instance,
	}
	f := NewForce(creds)
	return f
}

func initializeSession() {
	var err error
	if account != "" {
		force, err = GetForce(account)
	} else if force = envSession(); force == nil {
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
