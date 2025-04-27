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
	accounts    []string
	configName  string
	_apiVersion string

	manager forceManager
	force   *Force
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
	RootCmd.PersistentFlags().StringArrayVarP(&accounts, "account", "a", []string{}, "account `username` to use")
	RootCmd.PersistentFlags().StringVar(&configName, "config", "", "config directory to use (default: .force)")
	RootCmd.PersistentFlags().StringVarP(&_apiVersion, "apiversion", "V", "", "API version to use")
}

var RootCmd = &cobra.Command{
	Use:   "force",
	Short: "force CLI",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initializeConfig()
		checkAccounts(cmd.Name())
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

func checkAccounts(command string) {
	//currently only import allows many accounts so a catch all error handling is simpler

	if len(accounts) > 1 && command != "import" {
		ErrorAndExit(fmt.Sprintf("Multiple accounts are not supported for %s yet", command))
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
	manager = newForceManager(accounts)

	if _apiVersion != "" {
		err := SetApiVersion(_apiVersion)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}

	force = manager.getCurrentForce()
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

// provides support for commands that can be run concurrently for many accounts
type forceManager struct {
	connections    map[string]*Force
	currentAccount string
}

func (manager forceManager) getCurrentForce() *Force {
	return manager.connections[manager.currentAccount]
}

func (manager forceManager) getAllForce() []*Force {
	fs := make([]*Force, 0, len(manager.connections))

	for _, v := range manager.connections {
		fs = append(fs, v)
	}
	return fs
}

func newForceManager(accounts []string) forceManager {
	var err error
	fm := forceManager{connections: make(map[string]*Force, 1)}

	if len(accounts) > 1 {
		for _, a := range accounts {
			if _, exists := fm.connections[a]; exists {
				ErrorAndExit("Duplicate account: " + a)
			}

			var f *Force

			f, err = GetForce(a)
			if err != nil {
				ErrorAndExit(err.Error())
			}

			fm.connections[a] = f
		}

		fm.currentAccount = accounts[0]
	} else {
		var f *Force

		if len(accounts) == 1 {
			f, err = GetForce(accounts[0])
		} else if f = envSession(); f == nil {
			f, err = ActiveForce()
		}

		if err != nil {
			ErrorAndExit(err.Error())
		}

		fm.currentAccount = f.GetCredentials().UserInfo.UserName
		fm.connections[fm.currentAccount] = f
	}

	return fm
}
