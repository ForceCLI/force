package command

import (
	"fmt"
	"net/url"
	"os"

	"github.com/bgentry/speakeasy"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag/v2"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

type ScratchFeature enumflag.Flag

const (
	PersonAccounts ScratchFeature = iota
	ContactsToMultipleAccounts
	FinancialServicesUser
	StateAndCountryPicklist
)

var ScratchFeatureIds = map[ScratchFeature][]string{
	PersonAccounts:             {"PersonAccounts"},
	ContactsToMultipleAccounts: {"ContactsToMultipleAccounts"},
	FinancialServicesUser:      {"FinancialServicesUser"},
	StateAndCountryPicklist:    {"StateAndCountryPicklist"},
}

type ScratchProduct enumflag.Flag

const (
	FSC ScratchProduct = iota
)

var ScratchProductIds = map[ScratchProduct][]string{
	FSC: {"fsc"},
}

var (
	selectedFeatures  []ScratchFeature
	selectedProducts  []ScratchProduct
	featureQuantities map[string]string
)

var featuresRequiringQuantity = map[string]bool{
	"FinancialServicesUser": true,
}

const defaultFeatureQuantity = "10"

func init() {
	loginCmd.Flags().StringP("user", "u", "", "username for SOAP login")
	loginCmd.Flags().StringP("password", "p", "", "password for SOAP login")
	loginCmd.Flags().StringP("api-version", "v", "", "API version to use")
	loginCmd.Flags().String("connected-app-client-id", "", "Client Id (aka Consumer Key) to use instead of default")
	loginCmd.Flags().StringP("key", "k", "", "JWT signing key filename")
	loginCmd.Flags().String("connected-app-client-secret", "", "Client Secret (aka Consumer Secret) for Client Credentials flow")
	loginCmd.Flags().BoolP("skip", "s", false, "skip login if already authenticated and only save token (useful with SSO)")
	loginCmd.Flags().StringP("instance", "i", "", `Defaults to 'login' or last
logged in system. non-production server to login to (values are 'pre',
'test', or full instance url`)
	loginCmd.Flags().IntP("port", "P", 3835, "port for local OAuth callback server")
	loginCmd.Flags().Bool("device-flow", false, "use OAuth Device Flow (for headless environments)")

	scratchCmd.Flags().String("username", "", "username for scratch org user")
	scratchCmd.Flags().Var(
		enumflag.NewSlice(&selectedFeatures, "feature", ScratchFeatureIds, enumflag.EnumCaseInsensitive),
		"feature",
		"feature to enable (can be specified multiple times); see command help for available features")
	scratchCmd.Flags().Var(
		enumflag.NewSlice(&selectedProducts, "product", ScratchProductIds, enumflag.EnumCaseInsensitive),
		"product",
		"product shortcut for features (can be specified multiple times); see command help for available products")
	scratchCmd.Flags().StringToString("quantity", map[string]string{}, "override default quantity for features (e.g., FinancialServicesUser=5); default quantity is 10")

	loginCmd.AddCommand(scratchCmd)
	RootCmd.AddCommand(loginCmd)
}

var scratchCmd = &cobra.Command{
	Use:   "scratch",
	Short: "Create scratch org and log in",
	Long: `Create scratch org and log in

Available Features:
  ContactsToMultipleAccounts - Allows a single Contact to be associated with multiple Accounts
  FinancialServicesUser      - Enables Financial Services Cloud user licenses (requires quantity, default: 10)
  PersonAccounts             - Enables Person Accounts (B2C account model)
  StateAndCountryPicklist    - Enables State and Country Picklists for standard address fields

Available Products:
  fsc - Financial Services Cloud (enables PersonAccounts, ContactsToMultipleAccounts, FinancialServicesUser)

Examples:
  force login scratch --product fsc
  force login scratch --feature PersonAccounts --feature StateAndCountryPicklist
  force login scratch --product fsc --quantity FinancialServicesUser=20`,
	Run: func(cmd *cobra.Command, args []string) {
		scratchUser, _ := cmd.Flags().GetString("username")
		quantities, _ := cmd.Flags().GetStringToString("quantity")
		allFeatures := expandProductsToFeatures(selectedProducts, selectedFeatures, quantities)
		scratchLogin(scratchUser, allFeatures)
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
    force login --connected-app-client-id <my-consumer-key> --connected-app-client-secret <my-consumer-secret>
    force login -P 8080
    force login --device-flow
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
		clientSecret, _ := cmd.Flags().GetString("connected-app-client-secret")
		switch {
		case clientSecret != "":
			clientCredentialsLogin(endpoint, ClientId, clientSecret)
		case username == "":
			deviceFlow, _ := cmd.Flags().GetBool("device-flow")
			skipLogin, _ := cmd.Flags().GetBool("skip")
			port, _ := cmd.Flags().GetInt("port")
			if deviceFlow {
				oauthDeviceFlowLogin(endpoint, skipLogin, port)
			} else {
				oauthLogin(endpoint, skipLogin, port)
			}
		case keyFile != "":
			jwtLogin(endpoint, username, keyFile)
		default:
			password, _ := cmd.Flags().GetString("password")
			passwordLogin(endpoint, username, password)
		}
	},
}

func expandProductsToFeatures(products []ScratchProduct, features []ScratchFeature, quantities map[string]string) []string {
	productFeatures := map[ScratchProduct][]ScratchFeature{
		FSC: {PersonAccounts, ContactsToMultipleAccounts, FinancialServicesUser},
	}

	featureSet := make(map[ScratchFeature]bool)

	for _, product := range products {
		if pf, ok := productFeatures[product]; ok {
			for _, f := range pf {
				featureSet[f] = true
			}
		}
	}

	for _, feature := range features {
		featureSet[feature] = true
	}

	uniqueFeatures := make([]string, 0, len(featureSet))
	for feature := range featureSet {
		featureName := ScratchFeatureIds[feature][0]
		if featuresRequiringQuantity[featureName] {
			quantity := defaultFeatureQuantity
			if q, ok := quantities[featureName]; ok {
				quantity = q
			}
			featureName = featureName + ":" + quantity
		}
		uniqueFeatures = append(uniqueFeatures, featureName)
	}

	return uniqueFeatures
}

func scratchLogin(scratchUser string, features []string) {
	_, err := ForceScratchCreateLoginAndSave(scratchUser, features, os.Stderr)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func oauthLogin(endpoint string, skipLogin bool, port int) {
	var err error
	if skipLogin {
		_, err = ForceLoginAtEndpointWithPromptAndSaveWithPort(endpoint, os.Stdout, "consent", port)
	} else {
		_, err = ForceLoginAtEndpointAndSaveWithPort(endpoint, os.Stdout, port)
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func oauthDeviceFlowLogin(endpoint string, skipLogin bool, port int) {
	var err error
	if skipLogin {
		_, err = ForceLoginAtEndpointAndSaveDeviceFlow(endpoint, os.Stdout, "consent")
	} else {
		_, err = ForceLoginAtEndpointAndSaveDeviceFlow(endpoint, os.Stdout, "login")
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

func clientCredentialsLogin(endpoint, clientId, clientSecret string) {
	_, err := ForceLoginAtEndpointAndSaveClientCredentials(endpoint, clientId, clientSecret, os.Stdout)
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
