package sf

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// org delete sandbox follows the sf plugin source and was not compared
// with sf: it needs a production org with a sandbox.

// sandboxConfig is the content of ~/.sfdx/<org id>.sandbox.json, which sf
// writes when it authorizes a sandbox it created or refreshed.
type sandboxConfig struct {
	SandboxUsername  string `json:"sandboxUsername,omitempty"`
	SandboxOrgID     string `json:"sandboxOrgId,omitempty"`
	ProdOrgUsername  string `json:"prodOrgUsername,omitempty"`
	SandboxName      string `json:"sandboxName,omitempty"`
	SandboxProcessID string `json:"sandboxProcessId,omitempty"`
	SandboxInfoID    string `json:"sandboxInfoId,omitempty"`
	Timestamp        string `json:"timestamp,omitempty"`
}

func sandboxConfigPath(orgID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".sfdx", orgID+".sandbox.json"), nil
}

func readSandboxConfig(orgID string) (*sandboxConfig, error) {
	path, err := sandboxConfigPath(orgID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config := &sandboxConfig{}
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}
	return config, nil
}

// trimTo15 returns the 15 character form of a record Id.
func trimTo15(id string) string {
	if len(id) > 15 {
		return id[:15]
	}
	return id
}

// sandboxInfoIDFor asks the production org for the SandboxInfo of a
// sandbox, by its name and then by its org Id.  It returns "" when the
// production org has no sandbox process for it.
func sandboxInfoIDFor(prod *targetOrg, name, orgID string) string {
	query := func(field, value string) string {
		records, err := toolingRecords(prod, fmt.Sprintf("SELECT SandboxInfoId FROM SandboxProcess WHERE %s ='%s' AND Status NOT IN ('D', 'E')", field, value))
		if err != nil || len(records) != 1 {
			return ""
		}
		return text(records[0].values["SandboxInfoId"])
	}
	// sf 2.138.6 discards the Id it finds by name and then deletes
	// nothing in the production org.  This uses the Id.
	if name != "" {
		if id := query("SandboxName", name); id != "" {
			return id
		}
	}
	return query("SandboxOrganization", trimTo15(orgID))
}

func newOrgDeleteSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sandbox",
		Short:       "Delete a sandbox.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DeleteSandbox"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				if e, ok := err.(*Error); ok {
					switch e.Name {
					case "NoDefaultEnvError":
						return newError("MissingUsernameError", "Unable to determine the username of the org to delete. Specify the username with the --target-org | -o flag.")
					case "NamedOrgNotFoundError":
						name := targetOrgFlag(cmd)
						e.Actions = []string{
							fmt.Sprintf("Ensure the alias or username for the %s org is correct.", name),
							fmt.Sprintf("Ensure the %s org has been authenticated with the CLI.", name),
						}
					}
				}
				return err
			}
			orgID := org.Fields.String("orgId")
			config, err := readSandboxConfig(orgID)
			if err != nil {
				return &Error{
					Name:     "UnknownSandboxError",
					Message:  fmt.Sprintf("The org with username: %s is not known by the CLI to be a sandbox", org.Username),
					ExitCode: 1,
					Actions:  []string{"Re-authenticate the sandbox with the CLI and try again.", "Ensure the CLI has authenticated with the sandbox's production org."},
				}
			}
			result := orderedFields("username", org.Username, "orgId", orgID)
			if !confirm(cmd, fmt.Sprintf("Are you sure you want to delete the sandbox with name: %s?", org.Username)) {
				return emit(cmd, result, nil)
			}
			prod, err := loadOrg(config.ProdOrgUsername)
			if err != nil {
				return newError("NamedOrgNotFoundError", "No authorization information found for %s.", config.ProdOrgUsername)
			}
			infoID := config.SandboxInfoID
			if infoID == "" {
				name := config.SandboxName
				if name == "" {
					if parts := strings.SplitN(org.Username, prod.Username+".", 2); len(parts) == 2 {
						name = parts[1]
					}
				}
				infoID = sandboxInfoIDFor(prod, name, orgID)
			}
			if infoID != "" {
				if _, err := prod.Force.DeleteREST("/tooling/sobjects/SandboxInfo/" + infoID); err != nil {
					return apiError(err)
				}
			}
			if err := logout(org.Username); err != nil {
				return err
			}
			if path, err := sandboxConfigPath(orgID); err == nil {
				os.Remove(path)
			}
			return emit(cmd, result, func(w io.Writer) {
				fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Successfully marked sandbox %s for deletion.", org.Username)))
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().BoolP("no-prompt", "p", false, "Don't prompt the user to confirm the deletion.")
	addTargetOrgFlag(cmd)
	return cmd
}
