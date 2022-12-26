package command

import (
	"fmt"
	"net/url"
	"os"

	"github.com/bgentry/speakeasy"
	"github.com/spf13/cobra"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

func init() {
	loginCmd.Flags().StringP("user", "u", "", "username for SOAP login")
	loginCmd.Flags().StringP("password", "p", "", "password for SOAP login")
	loginCmd.Flags().StringP("api-version", "v", "", "API version to use")
	loginCmd.Flags().String("connected-app-client-id", "", "Client Id (aka Consumer Key) to use instead of default")
	loginCmd.Flags().StringP("key", "k", "", "JWT signing key filename")
	loginCmd.Flags().BoolP("skip", "s", false, "skip login if already authenticated and only save token (useful with SSO)")
	loginCmd.Flags().StringP("instance", "i", "", `Defaults to 'login' or last
logged in system. non-production server to login to (values are 'pre',
'test', or full instance url`)

	loginCmd.AddCommand(scratchCmd)
	RootCmd.AddCommand(loginCmd)
}

var scratchCmd = &cobra.Command{
	Use:   "scratch",
	Short: "Create scratch org and log in",
	Run: func(cmd *cobra.Command, args []string) {
		scratchLogin()
	},
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log into Salesforce and store a session token",
	Long: `Log into Salesforce and store a session token.  By default, OAuth is
used and a refresh token will be stored as well.  The refresh token is used
to get a new session token automatically when needed.`,
	Example: `
    force login
    force login -i test
    force login -i example--dev.sandbox.my.salesforce.com
    force login -u user@example.com -p password
    force login -i test -u user@example.com -p password
    force login -i my-domain.my.salesforce.com -u username -p password
    force login -i my-domain.my.salesforce.com -s[kipLogin]
    force login --connected-app-client-id <my-consumer-key> -u user@example.com -key jwt.key
    force login scratch
`,
	Args: cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		if connectedAppClientId, _ := cmd.Flags().GetString("connected-app-client-id"); connectedAppClientId != "" {
			ClientId = connectedAppClientId
		}
		endpoint := getEndpoint(cmd)
		selectApiVersion(cmd)
		username, _ := cmd.Flags().GetString("user")
		keyFile, _ := cmd.Flags().GetString("key")
		switch {
		case username == "":
			skipLogin, _ := cmd.Flags().GetBool("skip")
			oauthLogin(endpoint, skipLogin)
		case keyFile != "":
			jwtLogin(endpoint, username, keyFile)
		default:
			password, _ := cmd.Flags().GetString("password")
			passwordLogin(endpoint, username, password)
		}
	},
}

func scratchLogin() {
	_, err := ForceScratchLoginAndSave(os.Stderr)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func oauthLogin(endpoint string, skipLogin bool) {
	var err error
	if skipLogin {
		_, err = ForceLoginAtEndpointWithPromptAndSave(endpoint, os.Stdout, "consent")
	} else {
		_, err = ForceLoginAtEndpointAndSave(endpoint, os.Stdout)
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func jwtLogin(endpoint, username, keyfile string) {
	assertion, err := JwtAssertionForEndpoint(endpoint, username, keyfile, ClientId)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	_, err = ForceLoginAtEndpointAndSaveJWT(endpoint, assertion, os.Stdout)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func passwordLogin(endpoint, username, password string) {
	if len(password) == 0 {
		var err error
		password, err = speakeasy.Ask("Password: ")
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
	_, err := ForceLoginAtEndpointAndSaveSoap(endpoint, username, password, os.Stdout)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func getEndpoint(cmd *cobra.Command) string {
	endpoint := "https://login.salesforce.com"
	// If no instance specified, try to get last endpoint used
	var instance string
	if instance, _ = cmd.Flags().GetString("instance"); instance == "" {
		currentEndpointUrl, err := currentEndpointUrl()
		if err == nil && currentEndpointUrl != "" {
			endpoint = currentEndpointUrl
		}
	}
	switch instance {
	case "login":
		endpoint = "https://login.salesforce.com"
	case "test":
		endpoint = "https://test.salesforce.com"
	case "pre":
		endpoint = "https://prerelna1.salesforce.com"
	case "mobile1":
		endpoint = "https://mobile1.t.pre.salesforce.com"
	default:
		if instance != "" {
			//need to determine the form of the endpoint
			uri, err := url.Parse(instance)
			if err != nil {
				ErrorAndExit("Unable to parse endpoint: %s", instance)
			}
			// Could be short hand?
			if uri.Host == "" {
				uri, err = url.Parse(fmt.Sprintf("https://%s", instance))
				//fmt.Println(uri)
				if err != nil {
					ErrorAndExit("Could not identify host: %s", instance)
				}
			}
			endpoint = uri.Scheme + "://" + uri.Host

			fmt.Println("Loaded Endpoint: (" + endpoint + ")")
		}
	}
	return endpoint
}

func currentEndpoint() (endpoint ForceEndpoint, customUrl string, err error) {
	Log.Info("Deprecated call to CurrentEndpoint.  Use CurrentEndpointUrl.")
	creds, err := ActiveCredentials(false)
	if err != nil {
		return
	}
	endpoint = creds.ForceEndpoint
	customUrl = creds.InstanceUrl
	return
}

func currentEndpointUrl() (endpoint string, err error) {
	creds, err := ActiveCredentials(false)
	if err != nil {
		return "", err
	}
	return creds.EndpointUrl, nil
}

func selectApiVersion(cmd *cobra.Command) {
	if apiVersion, _ := cmd.Flags().GetString("api-version"); apiVersion != "" {
		// Todo verify format of version is 30.0
		SetApiVersion(apiVersion)
	} else {
		//override api version in case of a new login
		SetApiVersion(DefaultApiVersionNumber)
	}
}
