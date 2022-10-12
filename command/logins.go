package command

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	. "github.com/ForceCLI/force/config"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	loginsCmd.Flags().StringP("org-id", "o", "", "filter by org id")
	loginsCmd.Flags().StringP("user-id", "i", "", "filter by user id")
	RootCmd.AddCommand(loginsCmd)
}

type accountFilter func(ForceSession) bool

var loginsCmd = &cobra.Command{
	Use:   "logins",
	Short: "List force.com logins used",
	Example: `
  force logins
`,
	Run: func(cmd *cobra.Command, args []string) {
		runLogins(filters(cmd))
	},
}

func filters(cmd *cobra.Command) []accountFilter {
	var filters []accountFilter
	orgId, _ := cmd.Flags().GetString("org-id")
	if orgId != "" {
		filters = append(filters, func(s ForceSession) bool {
			if s.UserInfo == nil {
				return false
			}
			if len(orgId) == 15 {
				return s.UserInfo.OrgId[0:15] == orgId
			}
			return strings.ToLower(s.UserInfo.OrgId) == strings.ToLower(orgId)
		})
	}

	userId, _ := cmd.Flags().GetString("user-id")
	if userId != "" {
		filters = append(filters, func(s ForceSession) bool {
			if s.UserInfo == nil {
				return false
			}
			if len(userId) == 15 {
				return s.UserInfo.UserId[0:15] == userId
			}
			return strings.ToLower(s.UserInfo.UserId) == strings.ToLower(userId)
		})
	}
	return filters
}

func runLogins(filters []accountFilter) {
	active, _ := ActiveLogin()
	accounts, _ := Config.List("accounts")
	if len(accounts) == 0 {
		fmt.Println("no logins")
		return
	}
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 1, 0, 1, ' ', 0)

ACCOUNTS:
	for _, account := range accounts {
		if !strings.HasPrefix(account, ".") {
			var creds ForceSession
			data, err := Config.Load("accounts", account)
			json.Unmarshal([]byte(data), &creds)
			if err != nil {
				return
			}
			for _, f := range filters {
				if !f(creds) {
					continue ACCOUNTS
				}
			}

			var banner = fmt.Sprintf("\t%s", creds.InstanceUrl)
			if account == active {
				account = fmt.Sprintf("\x1b[31;1m%s (active)\x1b[0m", account)
			} else {
				account = fmt.Sprintf("%s \x1b[31;1m\x1b[0m", account)
			}
			fmt.Fprintln(w, fmt.Sprintf("%s%s", account, banner))
		}
	}
	fmt.Fprintln(w)
	w.Flush()
}
