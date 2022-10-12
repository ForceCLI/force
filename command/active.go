package command

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"

	. "github.com/ForceCLI/force/config"
	. "github.com/ForceCLI/force/error"
	"github.com/spf13/cobra"
)

func init() {
	activeCmd.Flags().BoolP("json", "j", false, "output in JSON format")
	activeCmd.Flags().BoolP("local", "l", false, "set active account locally, for current directory")
	activeCmd.Flags().BoolP("session", "s", false, "output session id")
	RootCmd.AddCommand(activeCmd)
}

var activeCmd = &cobra.Command{
	Use:   "active [account]",
	Short: "Show or set the active force.com account",
	Long:  "Get or set the active force.com account",
	Example: `
  force active
  force active user@example.org
  `,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			printCurrentAccount(cmd)
			return
		}
		local, _ := cmd.Flags().GetBool("local")
		setAccount(args[0], local)
	},
}

func printCurrentAccount(cmd *cobra.Command) {
	if force == nil {
		ErrorAndExit("No active session")
	}
	creds := force.Credentials
	if creds == nil || creds.UserInfo == nil {
		ErrorAndExit("No active session")
	}
	sessionId, _ := cmd.Flags().GetBool("session")
	tojson, _ := cmd.Flags().GetBool("json")
	if sessionId {
		fmt.Println(creds.AccessToken)
	} else if tojson {
		fmt.Printf(fmt.Sprintf("{ \"login\": \"%s\", \"instanceUrl\": \"%s\", \"namespace\":\"%s\" }", creds.SessionName(), creds.InstanceUrl, creds.UserInfo.OrgNamespace))
	} else {
		fmt.Println(fmt.Sprintf("%s - %s - ns:%s", creds.SessionName(), creds.InstanceUrl, creds.UserInfo.OrgNamespace))
	}
}

func setAccount(account string, local bool) {
	accounts, _ := Config.List("accounts")
	i := sort.SearchStrings(accounts, account)
	if i < len(accounts) && accounts[i] == account {
		if runtime.GOOS == "windows" {
			cmd := exec.Command("title", account)
			cmd.Run()
		} else {
			title := fmt.Sprintf("\033];%s\007", account)
			fmt.Printf(title)
		}
		fmt.Printf("%s now active\n", account)
		if local {
			Config.SaveLocal("current", "account", account)
		} else {
			Config.SaveGlobal("current", "account", account)
		}
	} else {
		ErrorAndExit(fmt.Sprintf("no such account %s\n", account))
	}
}
