package command

import (
	"fmt"
	"net/url"
	"os"

	"github.com/bgentry/speakeasy"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdLogin = &Command{
	Usage: "login",
	Short: "force login [-i=<instance>] [<-u=username> <-p=password>] [-scratch]",
	Long: `
  force login [-i=<instance>] [<-u=username> <-p=password> <-v=apiversion] [-scratch]

  Examples:
    force login
    force login -i=test
    force login -u=un -p=pw
    force login -i=test -u=un -p=pw
    force login -i=na1-blitz01.soma.salesforce.com -u=un -p=pw -v 39.0
    force login -i my-domain.my.salesforce.com -u username -p password
    force login --connected-app-client-id <my-consumer-key> -u username -key jwt.key
    force login -scratch
`,
}

func init() {
	cmdLogin.Run = runLogin
}

var (
	instance = cmdLogin.Flag.String("i", "", `Defaults to 'login' or last
		logged in system. non-production server to login to (values are 'pre',
		'test', or full instance url`)
	userName             = cmdLogin.Flag.String("u", "", "Username for Soap Login")
	password             = cmdLogin.Flag.String("p", "", "Password for Soap Login")
	api_version          = cmdLogin.Flag.String("v", "", "API Version to use")
	connectedAppClientId = cmdLogin.Flag.String("connected-app-client-id", "", "Client Id (aka Consumer Key) to use instead of default")
	keyFile              = cmdLogin.Flag.String("key", "", "JWT Signing Key Filename")
	scratchOrg           = cmdLogin.Flag.Bool("scratch", false, "Create new Scratch Org and Log In")
)

func runLogin(cmd *Command, args []string) {
	if *scratchOrg {
		_, err := ForceScratchLoginAndSave(os.Stderr)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		return
	}
	endpoint := "https://login.salesforce.com"
	// If no instance specified, try to get last endpoint used
	if *instance == "" {
		currentEndpointUrl, err := CurrentEndpointUrl()
		if err == nil && currentEndpointUrl != "" {
			endpoint = currentEndpointUrl
		}
	}

	if *api_version != "" {
		// Todo verify format of version is 30.0
		SetApiVersion(*api_version)
	} else {
		//override api version in case of a new login
		SetApiVersion(DefaultApiVersionNumber)
	}

	if *connectedAppClientId != "" {
		ClientId = *connectedAppClientId
	}

	switch *instance {
	case "login":
		endpoint = "https://login.salesforce.com"
	case "test":
		endpoint = "https://test.salesforce.com"
	case "pre":
		endpoint = "https://prerelna1.salesforce.com"
	case "mobile1":
		endpoint = "https://mobile1.t.pre.salesforce.com"
	default:
		if *instance != "" {
			//need to determine the form of the endpoint
			uri, err := url.Parse(*instance)
			if err != nil {
				ErrorAndExit("Unable to parse endpoint: %s", *instance)
			}
			// Could be short hand?
			if uri.Host == "" {
				uri, err = url.Parse(fmt.Sprintf("https://%s", *instance))
				//fmt.Println(uri)
				if err != nil {
					ErrorAndExit("Could not identify host: %s", *instance)
				}
			}
			endpoint = uri.Scheme + "://" + uri.Host

			fmt.Println("Loaded Endpoint: (" + endpoint + ")")
		}
	}

	if len(*userName) == 0 {
		// OAuth Login
		_, err := ForceLoginAtEndpointAndSave(endpoint, os.Stdout)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		return
	}

	if len(*keyFile) != 0 {
		// JWT Login
		assertion, err := JwtAssertionForEndpoint(endpoint, *userName, *keyFile, ClientId)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		_, err = ForceLoginAtEndpointAndSaveJWT(endpoint, assertion, os.Stdout)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		return
	}

	// Username/Password Login
	if len(*password) == 0 {
		var err error
		*password, err = speakeasy.Ask("Password: ")
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
	_, err := ForceLoginAtEndpointAndSaveSoap(endpoint, *userName, *password, os.Stdout)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func CurrentEndpoint() (endpoint ForceEndpoint, customUrl string, err error) {
	Log.Info("Deprecated call to CurrentEndpoint.  Use CurrentEndpointUrl.")
	creds, err := ActiveCredentials(false)
	if err != nil {
		return
	}
	endpoint = creds.ForceEndpoint
	customUrl = creds.InstanceUrl
	return
}

func CurrentEndpointUrl() (endpoint string, err error) {
	creds, err := ActiveCredentials(false)
	if err != nil {
		return "", err
	}
	return creds.EndpointUrl, nil
}
