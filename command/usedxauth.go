package command

import (
	"fmt"
	"os"
	"path"
	"strings"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(useDXAuthCmd)
}

var useDXAuthCmd = &cobra.Command{
	Use:   "usedxauth [dx-username or alias]",
	Short: "Authenticate with SFDX Scratch Org User",
	Long: `
Authenticate with SFDX Scratch Org User. If a user or alias is passed to the
command then an attempt is made to find that user authentication info.  If no
user or alias is passed an attempt is made to find the default user based on
sfdx config.
`,
	Example: `
  force usedxauth test-d1df0gyckgpr@dcarroll_company.net
  force usedxauth ScratchUserAlias
  force usedxauth
`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filter := strings.TrimSpace(account)
		if len(args) > 0 {
			arg := strings.TrimSpace(args[0])
			if filter != "" && !strings.EqualFold(filter, arg) {
				ErrorAndExit("Conflicting values provided via --account and argument: %s vs %s", filter, arg)
			}
			filter = arg
		}
		runUseDXAuth(filter)
	},
}

func runUseDXAuth(filter string) {
	auths, err := ListSFDXAuths()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if len(auths) == 0 {
		ErrorAndExit("No SFDX logins found")
	}

	aliases := ListSFDXAliases()

	var selected *SFDXAuth
	preferScratch := inProjectDir()

	if strings.TrimSpace(filter) != "" {
		fmt.Printf("Looking for %s in DX orgs...\n", filter)
		selected = findSFDXAuth(auths, filter)
		if selected == nil {
			ErrorAndExit("Could not find an alias or username that matches %s", filter)
		}
	} else {
		fmt.Println("Determining default user...")
		username := defaultSFDXUsername(aliases, preferScratch)
		if username != "" {
			selected = findSFDXAuth(auths, username)
		}
		if selected == nil {
			selected = &auths[0]
		}
	}

	alias := strings.TrimSpace(selected.Alias)
	if alias == "" {
		alias = aliasForUsername(aliases, selected.Username)
	}

	fmt.Printf("Getting auth for %s", selected.Username)
	if alias != "" {
		fmt.Printf(" (%s)", alias)
	}
	fmt.Println("...")

	session := SFDXAuthToForceSession(*selected)
	if err := CompleteSFDXSession(&session, alias); err != nil {
		ErrorAndExit(err.Error())
	}

	if _, err := ForceSaveLogin(session, os.Stderr); err != nil {
		ErrorAndExit(err.Error())
	}
	if alias != "" {
		fmt.Printf("Now using DX credentials for %s (%s)\n", selected.Username, alias)
	} else {
		fmt.Printf("Now using DX credentials for %s\n", selected.Username)
	}
}

func findSFDXAuth(auths []SFDXAuth, filter string) *SFDXAuth {
	filterLower := strings.ToLower(strings.TrimSpace(filter))
	if filterLower == "" {
		return nil
	}
	for i := range auths {
		auth := &auths[i]
		if strings.ToLower(auth.Username) == filterLower || strings.ToLower(auth.Alias) == filterLower {
			return auth
		}
	}
	return nil
}

func defaultSFDXUsername(aliases map[string]string, preferScratch bool) string {
	if preferScratch {
		if u, ok := aliases["defaultusername"]; ok {
			return u
		}
	}
	if u, ok := aliases["defaultdevhubusername"]; ok {
		return u
	}
	if !preferScratch {
		if u, ok := aliases["defaultusername"]; ok {
			return u
		}
	}
	return ""
}

func aliasForUsername(aliases map[string]string, username string) string {
	usernameLower := strings.ToLower(strings.TrimSpace(username))
	for alias, value := range aliases {
		if strings.ToLower(value) == usernameLower {
			return alias
		}
	}
	return ""
}

func inProjectDir() bool {
	dir, err := os.Getwd()
	if err != nil {
		return false
	}
	_, err = os.Stat(path.Join(dir, ".sfdx"))
	return err == nil
}
