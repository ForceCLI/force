package command

import (
	"fmt"
	"github.com/bgentry/speakeasy"
	"net/url"
	"os"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdLogin = &Command{
	Usage: "login",
	Short: "force login [-i=<instance>] [<-u=username> <-p=password>]",
	Long: `
  force login [-i=<instance>] [<-u=username> <-p=password> <-v=apiversion]

  Examples:
    force login
    force login -i=test
    force login -u=un -p=pw
    force login -i=test -u=un -p=pw
    force login -i=na1-blitz01.soma.salesforce.com -u=un -p=pw -v 39.0
    force login -i my-domain.my.salesforce.com -u username -p password
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
)

func runLogin(cmd *Command, args []string) {
	var endpoint ForceEndpoint = EndpointProduction
	// If no instance specified, try to get last endpoint used
	if *instance == "" {
		currentEndpoint, customUrl, err := CurrentEndpoint()
		if err == nil && &currentEndpoint != nil {
			endpoint = currentEndpoint
			if currentEndpoint == EndpointCustom && customUrl != "" {
				*instance = customUrl
			}
		}
	}

	if *api_version != "" {
		// Todo verify format of version is 30.0
		SetApiVersion(*api_version)
	}

	if *connectedAppClientId != "" {
		ClientId = *connectedAppClientId
	}

	switch *instance {
	case "login":
		endpoint = EndpointProduction
	case "test":
		endpoint = EndpointTest
	case "pre":
		endpoint = EndpointPrerelease
	case "mobile1":
		endpoint = EndpointMobile1
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
			CustomEndpoint = uri.Scheme + "://" + uri.Host
			endpoint = EndpointCustom

			fmt.Println("Loaded Endpoint: (" + CustomEndpoint + ")")
		}
	}

	if len(*userName) != 0 { // Do SOAP login
		if len(*password) == 0 {
			var err error
			*password, err = speakeasy.Ask("Password: ")
			if err != nil {
				ErrorAndExit(err.Error())
			}
		}
		_, err := ForceLoginAndSaveSoap(endpoint, *userName, *password, os.Stdout)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	} else { // Do OAuth login
		_, err := ForceLoginAndSave(endpoint, os.Stdout)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
}

func CurrentEndpoint() (endpoint ForceEndpoint, customUrl string, err error) {
	creds, err := ActiveCredentials(false)
	if err != nil {
		return
	}
	endpoint = creds.ForceEndpoint
	customUrl = creds.InstanceUrl
	return
}
