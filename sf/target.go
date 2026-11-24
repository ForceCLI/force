package sf

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

// targetOrg is an org authorization read from the sf state directory, with
// a session for making API requests.
type targetOrg struct {
	Username string
	Alias    string
	Fields   lib.SFDXAuthFields
	Order    []string
	Force    *lib.Force
}

// aliasFor returns the first alias in alias.json that refers to username.
func aliasFor(username string) string {
	aliases, _ := lib.ListSFDXAliasesOrdered()
	for _, alias := range aliases {
		if alias.Value == username {
			return alias.Alias
		}
	}
	return ""
}

// aliasesFor returns every alias that refers to username.
func aliasesFor(username string) []string {
	aliases, _ := lib.ListSFDXAliasesOrdered()
	var names []string
	for _, alias := range aliases {
		if alias.Value == username {
			names = append(names, alias.Alias)
		}
	}
	return names
}

// loadOrg reads the authorization for a username or alias and builds a
// session for it.  A refreshed access token is saved to the authorization
// file.
func loadOrg(nameOrAlias string) (*targetOrg, error) {
	username := lib.ResolveSFDXUsername(nameOrAlias)
	fields, order, err := lib.ReadSFDXAuthFieldsOrdered(username)
	if err != nil {
		return nil, err
	}
	username = fields.String("username")
	instanceURL := strings.TrimRight(fields.String("instanceUrl"), "/")
	loginURL := strings.TrimRight(fields.String("loginUrl"), "/")
	if loginURL == "" {
		loginURL = instanceURL
	}
	session := &lib.ForceSession{
		AccessToken:  fields.String("accessToken"),
		InstanceUrl:  instanceURL,
		EndpointUrl:  loginURL,
		ClientId:     fields.String("clientId"),
		RefreshToken: fields.String("refreshToken"),
		Id:           fields.String("orgId"),
		UserInfo: &lib.UserInfo{
			UserName:     username,
			OrgId:        fields.String("orgId"),
			UserId:       fields.String("userId"),
			OrgNamespace: fields.String("namespacePrefix"),
		},
		SessionOptions: &lib.SessionOptions{
			ApiVersion: fields.String("instanceApiVersion"),
			Alias:      username,
		},
	}
	session.SessionOptions.RefreshFunc = func(f *lib.Force) error {
		if f.Credentials.RefreshToken == "" {
			return fmt.Errorf("Unable to refresh session: expired access/refresh token")
		}
		if err := f.RefreshAccessToken(); err != nil {
			return err
		}
		return lib.UpdateSFDXAuthFields(username, lib.SFDXAuthFields{"accessToken": f.Credentials.AccessToken})
	}
	return &targetOrg{
		Username: username,
		Alias:    aliasFor(username),
		Fields:   fields,
		Order:    order,
		Force:    lib.NewForce(session),
	}, nil
}

// resolveTargetOrg returns the org a command acts on: the value of
// --target-org, or the target-org config value.  It also selects the API
// version from --api-version, the org-api-version config value, or the
// version recorded in the authorization file.
func resolveTargetOrg(cmd *cobra.Command) (*targetOrg, error) {
	return resolveOrg(cmd, targetOrgFlag(cmd), "target-org", lib.SFConfigTargetOrg)
}

// resolveTargetDevHub is resolveTargetOrg for --target-dev-hub.
func resolveTargetDevHub(cmd *cobra.Command) (*targetOrg, error) {
	name, _ := cmd.Flags().GetString("target-dev-hub")
	return resolveOrg(cmd, name, "target-dev-hub", lib.SFConfigTargetDevHub)
}

func resolveOrg(cmd *cobra.Command, name, flagName, configKey string) (*targetOrg, error) {
	version := ""
	if flag := cmd.Flags().Lookup("api-version"); flag != nil {
		version = flag.Value.String()
	}
	return resolveOrgNamed(name, flagName, configKey, version)
}

// resolveOrgNamed is resolveOrg with the values of the org flag and of
// --api-version given directly, for a command line that cobra has not
// parsed.
func resolveOrgNamed(name, flagName, configKey, version string) (*targetOrg, error) {
	fromFlag := name != ""
	if name == "" {
		value, found, err := lib.GetSFConfig(configKey)
		if err != nil {
			return nil, err
		}
		if !found || value.Value == "" {
			if configKey == lib.SFConfigTargetDevHub {
				return nil, newError("NoDefaultDevHubError", "No default dev hub found. Use -v or --target-dev-hub to specify an environment.")
			}
			return nil, newError("NoDefaultEnvError", "No default environment found. Use -o or --target-org to specify an environment.")
		}
		name = value.Value
	}
	org, err := loadOrg(name)
	if err != nil {
		var actions []string
		if similar := similarOrgName(name); similar != "" {
			actions = []string{fmt.Sprintf("It looks like you mistyped the username or alias. Did you mean %q?", similar)}
		}
		if fromFlag {
			return nil, &Error{
				Name:     "NamedOrgNotFoundError",
				Message:  fmt.Sprintf("Parsing --%s \n\tNo authorization information found for %s.\nSee more help with --help", flagName, name),
				ExitCode: 2,
				Actions:  actions,
			}
		}
		return nil, &Error{
			Name:     "NamedOrgNotFoundError",
			Message:  fmt.Sprintf("No authorization information found for %s.", name),
			ExitCode: 1,
			Actions:  actions,
		}
	}
	if version == "" {
		if value, found, _ := lib.GetSFConfig(lib.SFConfigApiVersion); found {
			version = value.Value
		}
	}
	if version == "" {
		version = org.Fields.String("instanceApiVersion")
	}
	if version != "" {
		if err := lib.SetApiVersion(version); err != nil {
			return nil, newError("InvalidApiVersionError", "%s", err.Error())
		}
		org.Force.Credentials.SessionOptions.ApiVersion = version
		// These clients record the API version when they are created.
		org.Force.Metadata = lib.NewForceMetadata(org.Force)
		org.Force.Partner = lib.NewForcePartner(org.Force)
	}
	return org, nil
}

// connectedStatus makes an authenticated request to the org and returns
// "Connected" or the reason the request failed.  The list of API versions
// at /services/data is answered without authentication, so the request is
// for the resources of one version, as in sf.
func connectedStatus(org *targetOrg) string {
	_, err := org.Force.GetREST("")
	if err == nil {
		return "Connected"
	}
	if errors.Is(err, lib.SessionExpiredError) {
		// The org refused the access token and a new one could not be
		// obtained.  Ask for one again to get the reason.
		var refused *lib.RefreshTokenError
		if refreshErr := org.Force.RefreshAccessToken(); errors.As(refreshErr, &refused) && refused.Description != "" {
			return "Unable to refresh session due to: Error authenticating with the refresh token due to: " + refused.Description
		}
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "connection refused"):
		return "ECONNREFUSED"
	case strings.Contains(message, "no such host"):
		// sf looks up the host of the org before it connects.
		return "DomainNotFoundError"
	case strings.Contains(message, "maintenance"):
		return "Down (Maintenance)"
	case strings.Contains(message, "<html>") || strings.Contains(message, "<!DOCTYPE HTML>"):
		return "Bad Response"
	}
	return message
}

// redactedSecret is the text sf shows in place of an access token.
func redactedSecret() string {
	return "[REDACTED] Use '" + binName + " org auth show-access-token' to view"
}

func secretsHiddenWarning(command string) string {
	return fmt.Sprintf("Secrets are now hidden from '"+binName+" %s' command output. Use the '"+binName+" org auth' commands instead. As a temporary workaround, you can set SF_TEMP_SHOW_SECRETS=true to render these secrets. This workaround will be removed in an upcoming release.", command)
}

// defaultUsernames returns the usernames that the target-org and
// target-dev-hub config values refer to.
func defaultUsernames() (org string, devHub string) {
	if value, found, _ := lib.GetSFConfig(lib.SFConfigTargetOrg); found {
		org = lib.ResolveSFDXUsername(value.Value)
	}
	if value, found, _ := lib.GetSFConfig(lib.SFConfigTargetDevHub); found {
		devHub = lib.ResolveSFDXUsername(value.Value)
	}
	return
}

// similarOrgName returns the authorized username or alias with the smallest
// edit distance from name, or "" when there are none.  sf suggests it for a
// name that has no authorization, whatever the distance.  Of two names at
// the same distance sf suggests the later one, with usernames before
// aliases.
func similarOrgName(name string) string {
	candidates := authorizedUsernames()
	if aliases, err := lib.ListSFDXAliasesOrdered(); err == nil {
		for _, alias := range aliases {
			candidates = append(candidates, alias.Alias)
		}
	}
	best, bestDistance := "", -1
	for _, candidate := range candidates {
		if distance := editDistance(name, candidate); bestDistance < 0 || distance <= bestDistance {
			best, bestDistance = candidate, distance
		}
	}
	return best
}

// editDistance is the Levenshtein distance between two strings.
func editDistance(a, b string) int {
	x, y := []rune(a), []rune(b)
	previous := make([]int, len(y)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(x); i++ {
		current := make([]int, len(y)+1)
		current[0] = i
		for j := 1; j <= len(y); j++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return previous[len(y)]
}

// An authorization file is named after a username, which has an @ and a
// domain.  sf ignores other JSON files in the state directory.
var authFileUsername = regexp.MustCompile(`^[^.][^@]*@[^.]+(\.[^.\s]+)+$`)

// authorizedUsernames returns the usernames that have an authorization
// file.
func authorizedUsernames() []string {
	var usernames []string
	for _, username := range lib.ListSFDXUsernames() {
		if authFileUsername.MatchString(username) {
			usernames = append(usernames, username)
		}
	}
	return usernames
}
