package command

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	. "github.com/heroku/force/config"
	. "github.com/heroku/force/lib"
)

var cmdLogins = &Command{
	Run:   runLogins,
	Usage: "logins",
	Short: "List force.com logins used",
	Long: `
List force.com accounts

Examples:

  force logins
`,
}

func runLogins(cmd *Command, args []string) {
	active, _ := ActiveLogin()
	accounts, _ := Config.List("accounts")
	if len(accounts) == 0 {
		fmt.Println("no logins")
	} else {
		w := new(tabwriter.Writer)
		w.Init(os.Stdout, 1, 0, 1, ' ', 0)

		for _, account := range accounts {
			if !strings.HasPrefix(account, ".") {
				var creds ForceSession
				data, err := Config.Load("accounts", account)
				json.Unmarshal([]byte(data), &creds)

				if err != nil {
					return
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

}
