package main

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/bgentry/speakeasy"
	"github.com/heroku/force/salesforce"
	"github.com/heroku/force/util"
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
    force login -i=na1-blitz01.soma.salesforce.com -u=un -p=pw
`,
}

func init() {
	cmdLogin.Run = runLogin
}

var (
	instance = cmdLogin.Flag.String("i", "", `Defaults to 'login' or last
		logged in system. non-production server to login to (values are 'pre',
		'test', or full instance url`)
	userName    = cmdLogin.Flag.String("u", "", "Username for Soap Login")
	password    = cmdLogin.Flag.String("p", "", "Password for Soap Login")
	api_version = cmdLogin.Flag.String("v", "", "API Version to use")
)

func runLogin(cmd *Command, args []string) {
	var endpoint salesforce.ForceEndpoint = salesforce.EndpointProduction

	// If no instance specified, try to get last endpoint used
	if *instance == "" {
		currentEndpoint, customUrl, err := CurrentEndpoint()
		if err == nil && &currentEndpoint != nil {
			endpoint = currentEndpoint
			if currentEndpoint == salesforce.EndpointCustom && customUrl != "" {
				*instance = customUrl
			}
		}
	}

	var apiVersion = salesforce.DefaultApiVersion

	if *api_version != "" {
		// Todo verify format of version is 30.0
		apiVersion = "v" + *api_version
	}

	switch *instance {
	case "login":
		endpoint = salesforce.EndpointProduction
	case "test":
		endpoint = salesforce.EndpointTest
	case "pre":
		endpoint = salesforce.EndpointPrerelease
	case "mobile1":
		endpoint = salesforce.EndpointMobile1
	default:
		if *instance != "" {
			//need to determine the form of the endpoint
			uri, err := url.Parse(*instance)
			if err != nil {
				util.ErrorAndExit("Unable to parse endpoint: %s", *instance)
			}
			// Could be short hand?
			if uri.Host == "" {
				uri, err = url.Parse(fmt.Sprintf("https://%s", *instance))
				//fmt.Println(uri)
				if err != nil {
					util.ErrorAndExit("Could not identify host: %s", *instance)
				}
			}
			// use a global side-effect to set the custom endpoint
			salesforce.CustomEndpoint = uri.Scheme + "://" + uri.Host
			endpoint = salesforce.EndpointCustom

			fmt.Println("Loaded Endpoint: (" + salesforce.CustomEndpoint + ")")
		}
	}

	if len(*userName) != 0 { // Do SOAP login
		if len(*password) == 0 {
			var err error
			*password, err = speakeasy.Ask("Password: ")
			if err != nil {
				util.ErrorAndExit(err.Error())
			}
		}
		_, err := ForceLoginAndSaveSoap(endpoint, *userName, *password, apiVersion)
		if err != nil {
			util.ErrorAndExit(err.Error())
		}
	} else { // Do OAuth login
		_, err := ForceLoginAndSave(endpoint)
		if err != nil {
			util.ErrorAndExit(err.Error())
		}
	}
}

func CurrentEndpoint() (endpoint salesforce.ForceEndpoint, customUrl string, err error) {
	creds, err := ActiveCredentials()
	if err != nil {
		return
	}
	endpoint = creds.ForceEndpoint
	customUrl = creds.InstanceUrl
	return
}

func ForceSaveLogin(creds salesforce.ForceCredentials) (username string, err error) {
	// TODO find existing creds to rescue existing apiVersion:
	creds.ApiVersion = salesforce.DefaultApiVersion

	if existingCredsJSON, err := util.Config.Load("accounts", account); err == nil {
		// there's an existing account!  Copy over its api version (and any other
		// settings we want to persist across re-logins:)
		var existingCreds salesforce.ForceCredentials
		if err = json.Unmarshal([]byte(existingCredsJSON), &existingCreds); err == nil {
			if existingCreds.ApiVersion != "" {
				fmt.Printf("We already have settings for a previous login of this account, carrying them over\n")
				creds.ApiVersion = existingCreds.ApiVersion
			}
		}
	}

	force := salesforce.NewForce(creds)
	login, err := force.Get(creds.Id)
	if err != nil {
		return
	}

	body, err := json.Marshal(creds)
	if err != nil {
		return
	}

	userId := login["user_id"].(string)
	creds.UserId = userId
	username = login["username"].(string)

	me, err := force.Whoami()
	if err != nil {
		fmt.Println("Problem getting user data, continuing...")
		//return
	}
	fmt.Printf("Logged in as '%s' (API %s)\n", me["Username"], creds.ApiVersion)
	title := fmt.Sprintf("\033];%s\007", me["Username"])
	fmt.Printf(title)

	describe, err := force.Metadata.DescribeMetadata()

	if err == nil {
		creds.Namespace = describe.NamespacePrefix
	} else {
		fmt.Printf("Your profile does not have Modify All Data enabled.  Functionality will be limited.\n")
		err = nil
	}

	body, err = json.Marshal(creds)
	if err != nil {
		return
	}
	util.Config.Save("accounts", username, string(body))
	util.Config.Save("current", "account", username)
	return
}

func ForceLoginAndSaveSoap(endpoint salesforce.ForceEndpoint, user_name string, password string, apiversion string) (username string, err error) {
	creds, err := salesforce.ForceSoapLogin(endpoint, apiversion, user_name, password)
	if err != nil {
		return
	}

	creds.ApiVersion = apiversion

	username, err = ForceSaveLogin(creds)
	//fmt.Printf("Creds %+v", creds)
	return
}

func ForceLoginAndSave(endpoint salesforce.ForceEndpoint) (username string, err error) {
	creds, err := salesforce.ForceLogin(endpoint)
	if err != nil {
		return
	}

	creds.ApiVersion = salesforce.DefaultApiVersion

	username, err = ForceSaveLogin(creds)
	return
}
