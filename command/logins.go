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
	loginsCmd.Flags().Bool("sfdx", false, "include SFDX logins")
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
		showSFDX, _ := cmd.Flags().GetBool("sfdx")
		runLogins(filters(cmd), showSFDX)
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

func runLogins(filters []accountFilter, includeSFDX bool) {
	active, _ := ActiveLogin()
	accounts, _ := Config.List("accounts")
	var sfdxAuths []SFDXAuth
	var err error
	if includeSFDX {
		sfdxAuths, err = ListSFDXAuths()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load SFDX logins: %v\n", err)
			includeSFDX = false
		}
	}
	if len(accounts) == 0 && (!includeSFDX || len(sfdxAuths) == 0) {
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
	if includeSFDX && len(sfdxAuths) > 0 {
		if len(accounts) > 0 {
			fmt.Fprintln(w)
		}
		for _, auth := range sfdxAuths {
			session := SFDXAuthToForceSession(auth)
			include := true
			for _, f := range filters {
				if !f(session) {
					include = false
					break
				}
			}
			if !include {
				continue
			}
			name := auth.Alias
			if strings.TrimSpace(name) == "" {
				name = auth.Username
			}
			if strings.TrimSpace(name) == "" {
				name = auth.Id
			}
			name = fmt.Sprintf("%s [sfdx]", name)
			instance := session.InstanceUrl
			if strings.TrimSpace(instance) == "" {
				instance = session.EndpointUrl
			}
			if strings.TrimSpace(instance) == "" {
				instance = auth.LoginUrl
			}
			fmt.Fprintf(w, "%s\t%s\n", name, instance)
		}
	}
	fmt.Fprintln(w)
	w.Flush()
}
