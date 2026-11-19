package sf

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

func newOrgLoginCmd() *cobra.Command {
	return newGroup("login", "Authorize an org for use with Salesforce CLI.",
		newOrgLoginAccessTokenCmd(),
		newOrgLoginJwtCmd(),
		newOrgLoginSfdxURLCmd(),
		newOrgLoginWebCmd(),
	)
}

// authorization is the outcome of a login flow, before the org details are
// read and the authorization file is written.
type authorization struct {
	session    lib.ForceSession
	loginURL   string
	privateKey string
	// order is the order sf writes the fields in for the login flow.
	order []string
}

var orgDetailOrder = []string{
	"isDevHub", "instanceApiVersion", "instanceApiVersionLastRetrieved", "name", "instanceName",
	"namespacePrefix", "isSandbox", "isScratch", "trailExpirationDate", "orgEdition",
}

func addLoginFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("instance-url", "r", "", "URL of the instance that the org lives on.")
	cmd.Flags().BoolP("set-default-dev-hub", "d", false, "Set the authenticated org as the default Dev Hub.")
	cmd.Flags().BoolP("set-default", "s", false, "Set the authenticated org as the default that all org-related commands run against.")
	cmd.Flags().StringP("alias", "a", "", "Alias for the org.")
}

// loginInstanceURL returns --instance-url, the org-instance-url config
// value, or https://login.salesforce.com.
func loginInstanceURL(cmd *cobra.Command) string {
	if value, _ := cmd.Flags().GetString("instance-url"); value != "" {
		return value
	}
	if value, found, _ := lib.GetSFConfig("org-instance-url"); found && value.Value != "" {
		return value.Value
	}
	return "https://login.salesforce.com"
}

func warnIfInsecure(cmd *cobra.Command, instanceURL string) {
	if strings.HasPrefix(instanceURL, "http:") {
		warn(cmd, fmt.Sprintf("Using insecure protocol: http: on url: %s", strings.TrimRight(instanceURL, "/")))
	}
}

// maxAPIVersion returns the newest API version the instance offers.
func maxAPIVersion(force *lib.Force) (string, error) {
	body, err := force.GetAbsolute("/services/data")
	if err != nil {
		return "", err
	}
	var versions []struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(body), &versions); err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("the instance lists no API versions")
	}
	return versions[len(versions)-1].Version, nil
}

// saveAuthorization reads the org details sf records, writes the
// authorization file, and applies --alias, --set-default and
// --set-default-dev-hub.  It returns the fields written, in file order.
func saveAuthorization(cmd *cobra.Command, auth authorization) (lib.SFDXAuthFields, []string, error) {
	session := auth.session
	session.InstanceUrl = strings.TrimRight(session.InstanceUrl, "/")
	if session.SessionOptions == nil {
		session.SessionOptions = &lib.SessionOptions{}
	}
	force := lib.NewForce(&session)

	version, err := maxAPIVersion(force)
	if err != nil {
		return nil, nil, err
	}
	if err := lib.SetApiVersion(version); err != nil {
		return nil, nil, err
	}
	userInfo, err := force.UserInfo()
	if err != nil {
		return nil, nil, err
	}
	if userInfo.UserName == "" {
		return nil, nil, fmt.Errorf("the org did not return a username for the access token")
	}

	// sf stores instance and login URLs from an access token login with a
	// trailing slash.
	instanceURL := session.InstanceUrl
	loginURL := strings.TrimRight(auth.loginURL, "/")
	if session.RefreshToken == "" && auth.privateKey == "" {
		instanceURL += "/"
		loginURL += "/"
	}
	fields := lib.SFDXAuthFields{
		"accessToken":                     session.AccessToken,
		"instanceUrl":                     instanceURL,
		"orgId":                           userInfo.OrgId,
		"username":                        userInfo.UserName,
		"loginUrl":                        loginURL,
		"instanceApiVersion":              version,
		"instanceApiVersionLastRetrieved": time.Now().Format("1/2/2006, 3:04:05 PM"),
		"isScratch":                       false,
	}
	if session.RefreshToken != "" {
		fields["refreshToken"] = session.RefreshToken
	}
	if session.ClientId != "" {
		fields["clientId"] = session.ClientId
	}
	if auth.privateKey != "" {
		fields["privateKey"] = auth.privateKey
	}
	_, devHubErr := force.Query("SELECT Id FROM ScratchOrgInfo LIMIT 1")
	fields["isDevHub"] = devHubErr == nil
	if result, err := force.Query("SELECT Name, InstanceName, IsSandbox, TrialExpirationDate, NamespacePrefix, OrganizationType FROM Organization"); err == nil && len(result.Records) > 0 {
		record := result.Records[0]
		fields["name"] = record["Name"]
		fields["instanceName"] = record["InstanceName"]
		fields["namespacePrefix"] = record["NamespacePrefix"]
		fields["isSandbox"] = record["IsSandbox"] == true
		fields["trailExpirationDate"] = record["TrialExpirationDate"]
		fields["orgEdition"] = record["OrganizationType"]
	}

	order := append(append([]string{}, auth.order...), orgDetailOrder...)
	if err := lib.WriteSFDXAuthFields(fields, order...); err != nil {
		return nil, nil, err
	}
	_, order, err = lib.ReadSFDXAuthFieldsOrdered(userInfo.UserName)
	if err != nil {
		return nil, nil, err
	}

	target := userInfo.UserName
	if alias, _ := cmd.Flags().GetString("alias"); alias != "" {
		if err := lib.SetSFDXAlias(alias, userInfo.UserName); err != nil {
			return nil, nil, err
		}
		target = alias
	}
	_, inProject := lib.SFLocalConfigPath()
	if setDefault, _ := cmd.Flags().GetBool("set-default"); setDefault {
		if _, err := lib.SetSFConfig(lib.SFConfigTargetOrg, target, !inProject); err != nil {
			return nil, nil, err
		}
	}
	if setDevHub, _ := cmd.Flags().GetBool("set-default-dev-hub"); setDevHub {
		if _, err := lib.SetSFConfig(lib.SFConfigTargetDevHub, target, !inProject); err != nil {
			return nil, nil, err
		}
	}
	return fields, order, nil
}

// emitAuthorization prints the result of a login command.
func emitAuthorization(cmd *cobra.Command, command string, fields lib.SFDXAuthFields, order []string) error {
	result := newOrderedMap()
	for _, key := range order {
		if secretFields[key] {
			continue
		}
		value := fields[key]
		if key == "accessToken" {
			value = redactedSecret()
		}
		result.Set(key, value)
	}
	if jsonRequested(cmd) {
		// sf lists this warning before the others.
		state := stateOf(cmd)
		state.warnings = append([]string{secretsHiddenWarning(command)}, state.warnings...)
	}
	return emit(cmd, result, func(w io.Writer) {
		fmt.Fprintf(w, "Successfully authorized %s with org ID %s\n", fields.String("username"), fields.String("orgId"))
	})
}

func newOrgLoginAccessTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access-token",
		Short: "Authorize an org using an existing Salesforce access token.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			instanceURL, _ := cmd.Flags().GetString("instance-url")
			if instanceURL == "" {
				return missingFlagError("instance-url")
			}
			token := os.Getenv("SF_ACCESS_TOKEN")
			if token == "" {
				token = os.Getenv("SFDX_ACCESS_TOKEN")
			}
			if token == "" {
				fmt.Fprint(cmd.ErrOrStderr(), "? Access token of user to use for authentication ")
				line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				token = strings.TrimSpace(line)
			}
			if !strings.Contains(token, "!") {
				return newError("AccessTokenInvalidError", "The access token isn't in the correct format.\nIt should include an exclamation point (\"!\") between the 15-character org ID and the session ID.\nFor example, \"00D0t000000HkBf!"+"AQEAQAuG.NP7BW9i1U5II9uLyPXuVFLnQNTn3hFY6TX67dKeS.rd4HZJ1ZeyZX3cZnIhxqgi5bsCdXSuL6EFjSRIv2ptkxqQ\".")
			}
			warnIfInsecure(cmd, instanceURL)
			fields, order, err := saveAuthorization(cmd, authorization{
				session: lib.ForceSession{
					AccessToken: token,
					InstanceUrl: instanceURL,
				},
				loginURL: instanceURL,
				order:    []string{"username", "orgId", "accessToken", "instanceUrl", "loginUrl"},
			})
			if err != nil {
				return err
			}
			return emitAuthorization(cmd, "org login access-token", fields, order)
		},
	}
	addJSONFlag(cmd)
	addLoginFlags(cmd)
	cmd.Flags().BoolP("no-prompt", "p", false, "Don't prompt for confirmation.")
	return cmd
}

func newOrgLoginJwtCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jwt",
		Short: "Log in to a Salesforce org using a JSON web token (JWT).",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			username, _ := cmd.Flags().GetString("username")
			keyFile, _ := cmd.Flags().GetString("jwt-key-file")
			clientID, _ := cmd.Flags().GetString("client-id")
			for flag, value := range map[string]string{"username": username, "jwt-key-file": keyFile, "client-id": clientID} {
				if value == "" {
					return missingFlagError(flag)
				}
			}
			instanceURL := loginInstanceURL(cmd)
			warnIfInsecure(cmd, instanceURL)
			assertion, err := lib.JwtAssertionForEndpoint(instanceURL, username, keyFile, clientID)
			if err != nil {
				return newError("JwtGrantError", "We encountered a JSON web token error, which is likely not an issue with Salesforce CLI. Here’s the error: %s", err.Error())
			}
			session, err := lib.JWTLoginAtEndpoint(instanceURL, assertion)
			if err != nil {
				return newError("JwtGrantError", "We encountered a JSON web token error, which is likely not an issue with Salesforce CLI. Here’s the error: %s", err.Error())
			}
			session.ClientId = clientID
			fields, order, err := saveAuthorization(cmd, authorization{
				session:    session,
				loginURL:   instanceURL,
				privateKey: keyFile,
				order:      []string{"accessToken", "instanceUrl", "orgId", "username", "loginUrl", "privateKey", "clientId"},
			})
			if err != nil {
				return err
			}
			return emitAuthorization(cmd, "org login jwt", fields, order)
		},
	}
	addJSONFlag(cmd)
	addLoginFlags(cmd)
	cmd.Flags().StringP("username", "o", "", "Username of the user logging in.")
	cmd.Flags().StringP("jwt-key-file", "f", "", "Path to a file containing the private key.")
	cmd.Flags().StringP("client-id", "i", "", "OAuth client ID (also called consumer key) of your custom connected app.")
	cmd.Flags().BoolP("no-prompt", "p", false, "Don't prompt for confirmation.")
	cmd.Flags().MarkHidden("no-prompt")
	return cmd
}

// unsupportedFlags returns an error naming the first of the given flags
// that was set.  sf defines these flags; this mode does not implement them.
func unsupportedFlags(cmd *cobra.Command, names ...string) error {
	for _, name := range names {
		if flag := cmd.Flags().Lookup(name); flag != nil && flag.Changed {
			return newError("UnsupportedFlagError", "The --%s flag is not supported by the sf-compatible mode of force.", name)
		}
	}
	return nil
}

func newOrgLoginWebCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Log in to a Salesforce org using the web server flow.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := unsupportedFlags(cmd, "client-app", "username"); err != nil {
				return err
			}
			browser, _ := cmd.Flags().GetString("browser")
			if browser != "" && browserCommands[browser] == nil {
				return &Error{Message: fmt.Sprintf("Expected --browser=%s to be one of: chrome, edge, firefox\nSee more help with --help", browser), ExitCode: 2}
			}
			var browserErr error
			if browser != "" {
				previous := lib.OpenLoginBrowser
				lib.OpenLoginBrowser = func(uri string) error {
					if err := openInBrowser(browser, uri); err != nil {
						browserErr = &Error{
							Name:     "CannotOpenBrowserError",
							Message:  fmt.Sprintf("Unable to open the browser you specified (%s).", browser),
							ExitCode: 1,
							Actions:  []string{fmt.Sprintf("Ensure that %s is installed on your computer. Or specify a different browser using the --browser flag.", browser)},
						}
					}
					return browserErr
				}
				lib.LoginBrowserRequired = true
				defer func() { lib.OpenLoginBrowser, lib.LoginBrowserRequired = previous, false }()
			}
			if scopes, _ := cmd.Flags().GetString("scopes"); scopes != "" {
				previous := lib.LoginScope
				lib.LoginScope = scopes
				defer func() { lib.LoginScope = previous }()
			}
			if clientID, _ := cmd.Flags().GetString("client-id"); clientID != "" {
				previous := lib.ClientId
				lib.ClientId = clientID
				defer func() { lib.ClientId = previous }()
			}
			instanceURL := loginInstanceURL(cmd)
			warnIfInsecure(cmd, instanceURL)
			session, err := lib.ForceLoginAtEndpointWithPromptAndPort(instanceURL, "login", lib.DefaultOAuthCallbackPort)
			if browserErr != nil {
				return browserErr
			}
			if err != nil {
				return err
			}
			session.ClientId = lib.ClientId
			fields, order, err := saveAuthorization(cmd, authorization{
				session:  session,
				loginURL: instanceURL,
				order:    []string{"accessToken", "instanceUrl", "orgId", "username", "loginUrl", "refreshToken", "clientId"},
			})
			if err != nil {
				return err
			}
			return emitAuthorization(cmd, "org login web", fields, order)
		},
	}
	addJSONFlag(cmd)
	addLoginFlags(cmd)
	cmd.Flags().StringP("browser", "b", "", "Browser in which to open the org.")
	cmd.Flags().StringP("client-id", "i", "", "OAuth client ID (also called consumer key) of your custom connected app.")
	cmd.Flags().StringP("client-app", "c", "", "Name to give to the link between the connected app or external client and the already-authenticated user.")
	cmd.Flags().String("username", "", "Username of the already-authenticated user to link to the connected app or external client app.")
	cmd.Flags().String("scopes", "", "Authentication (OAuth) scopes to request.")
	cmd.Flags().BoolP("no-prompt", "p", false, "Don't prompt for confirmation.")
	cmd.Flags().MarkHidden("no-prompt")
	return cmd
}

var sfdxURLPattern = regexp.MustCompile(`^force://([^:]*):([^:]*):([^@]*)@(.+)$`)

// readSfdxAuthURL returns the SFDX auth URL in content: the text itself,
// or the sfdxAuthUrl property of a JSON document such as the output of
// "sf org display --verbose --json".
func readSfdxAuthURL(content []byte) string {
	text := strings.TrimSpace(string(content))
	if strings.HasPrefix(text, "{") {
		var document struct {
			SfdxAuthUrl string `json:"sfdxAuthUrl"`
			Result      struct {
				SfdxAuthUrl string `json:"sfdxAuthUrl"`
			} `json:"result"`
		}
		if json.Unmarshal(content, &document) == nil {
			if document.SfdxAuthUrl != "" {
				return document.SfdxAuthUrl
			}
			return document.Result.SfdxAuthUrl
		}
	}
	return text
}

func newOrgLoginSfdxURLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sfdx-url",
		Short:       "Authorize an org using a Salesforce DX authorization URL stored in a file or through standard input (stdin).",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "LoginSfdxUrl"},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("sfdx-url-file")
			stdin, _ := cmd.Flags().GetString("sfdx-url-stdin")
			if (file == "") == (stdin == "") {
				return exactlyOneError("sfdx-url-file", "sfdx-url-stdin")
			}
			var content []byte
			var err error
			if file != "" {
				content, err = os.ReadFile(file)
			} else {
				content, err = io.ReadAll(cmd.InOrStdin())
			}
			if err != nil {
				return err
			}
			match := sfdxURLPattern.FindStringSubmatch(readSfdxAuthURL(content))
			if match == nil {
				return newError("INVALID_SFDX_AUTH_URL", "Invalid SFDX authorization URL. Must be in the format \"force://<clientId>:<clientSecret>:<refreshToken>@<instanceUrl>\".  Note that the SFDX authorization URL uses the \"force\" protocol, and not \"http\" or \"https\".  Also note that the \"instanceUrl\" inside the SFDX authorization URL doesn't include the protocol (\"https://\").")
			}
			clientID, clientSecret, refreshToken, host := match[1], match[2], match[3], match[4]
			instanceURL := host
			if !strings.Contains(host, "://") {
				instanceURL = "https://" + host
			}
			form := url.Values{}
			form.Set("grant_type", "refresh_token")
			form.Set("refresh_token", refreshToken)
			form.Set("client_id", clientID)
			if clientSecret != "" {
				form.Set("client_secret", clientSecret)
			}
			response, err := http.PostForm(strings.TrimRight(instanceURL, "/")+"/services/oauth2/token", form)
			if err != nil {
				return err
			}
			defer response.Body.Close()
			var token struct {
				AccessToken      string `json:"access_token"`
				InstanceURL      string `json:"instance_url"`
				ErrorDescription string `json:"error_description"`
			}
			body, _ := io.ReadAll(response.Body)
			if err := json.Unmarshal(body, &token); err != nil || token.AccessToken == "" {
				message := token.ErrorDescription
				if message == "" {
					message = strings.TrimSpace(string(body))
				}
				return newError("RefreshTokenAuthError", "Error authenticating with the refresh token due to: %s", message)
			}
			if token.InstanceURL == "" {
				token.InstanceURL = instanceURL
			}
			auth := authorization{
				session: lib.ForceSession{
					AccessToken:  token.AccessToken,
					InstanceUrl:  token.InstanceURL,
					RefreshToken: refreshToken,
					ClientId:     clientID,
				},
				loginURL: instanceURL,
				order:    []string{"accessToken", "instanceUrl", "orgId", "username", "loginUrl", "refreshToken", "clientId", "clientSecret"},
			}
			fields, order, err := saveAuthorization(cmd, auth)
			if err != nil {
				return err
			}
			if clientSecret != "" {
				if err := lib.UpdateSFDXAuthFields(fields.String("username"), lib.SFDXAuthFields{"clientSecret": clientSecret}); err != nil {
					return err
				}
			}
			return emitAuthorization(cmd, "org login sfdx-url", fields, order)
		},
	}
	addJSONFlag(cmd)
	addLoginFlags(cmd)
	cmd.Flags().StringP("sfdx-url-file", "f", "", "Path to a file that contains the Salesforce DX authorization URL.")
	cmd.Flags().StringP("sfdx-url-stdin", "u", "", "Pipe the Salesforce DX authorization URL through standard input (stdin).")
	cmd.Flags().Lookup("sfdx-url-stdin").NoOptDefVal = "-"
	cmd.Flags().BoolP("no-prompt", "p", false, "Don't prompt for confirmation.")
	cmd.Flags().MarkHidden("no-prompt")
	return cmd
}
