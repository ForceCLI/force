package main

import (
	"encoding/json"
	"fmt"
	"github.com/bgentry/speakeasy"
	"net/url"
)

var cmdLogin = &Command{
	Usage: "login",
	Short: "force login [-i=<instance>] [<-u=username> <-p=password>]",
	Long: `
  force login [-i=<instance>] [<-u=username> <-p=password>]

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
	userName = cmdLogin.Flag.String("u", "", "Username for Soap Login")
	password = cmdLogin.Flag.String("p", "", "Password for Soap Login")
)

func runLogin(cmd *Command, args []string) {
	var endpoint ForceEndpoint = EndpointProduction

	currentEndpoint, customUrl, err := CurrentEndpoint()
	if err == nil && &currentEndpoint != nil {
		endpoint = currentEndpoint
		if currentEndpoint == EndpointCustom && customUrl != "" {
			*instance = customUrl
		}
	}

	switch *instance {
	case "login":
		endpoint = EndpointProduction
	case "test":
		endpoint = EndpointTest
	case "pre":
		endpoint = EndpointPrerelease
	default:
		if *instance != "" {
			//need to determine the form of the endpoint
			uri, err := url.Parse(*instance)
			if err != nil {
				ErrorAndExit("no such endpoint: %s", *instance)
			}
			// Could be short hand?
			if uri.Host == "" {
				uri, err = url.Parse(fmt.Sprintf("https://%s", *instance))
				//fmt.Println(uri)
				if err != nil {
					ErrorAndExit("no such endpoint: %s", *instance)
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
		_, err := ForceLoginAndSaveSoap(endpoint, *userName, *password)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	} else { // Do OAuth login
		_, err := ForceLoginAndSave(endpoint)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
}

func CurrentEndpoint() (endpoint ForceEndpoint, customUrl string, err error) {
	creds, err := ActiveCredentials()
	if err != nil {
		return
	}
	endpoint = creds.ForceEndpoint
	customUrl = creds.InstanceUrl
	return
}

func ForceSaveLogin(creds ForceCredentials) (username string, err error) {
	force := NewForce(creds)
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
	fmt.Printf("Logged in as '%s'\n", me["Username"])
	title := fmt.Sprintf("\033];%s\007", me["Username"])
	fmt.Printf(title)

	describe, err := force.Metadata.DescribeMetadata()

	if err == nil {
		creds.Namespace = describe.NamespacePrefix
	} else {
		fmt.Printf("Your profile does not have Modify All Data enabled. Functionallity will be limited.\n")
		err = nil
	}

	body, err = json.Marshal(creds)
	if err != nil {
		return
	}
	Config.Save("accounts", username, string(body))
	Config.Save("current", "account", username)
	return
}

func ForceLoginAndSaveSoap(endpoint ForceEndpoint, user_name string, password string) (username string, err error) {
	creds, err := ForceSoapLogin(endpoint, user_name, password)
	if err != nil {
		return
	}

	username, err = ForceSaveLogin(creds)
	//fmt.Printf("Creds %+v", creds)
	return
}

func ForceLoginAndSave(endpoint ForceEndpoint) (username string, err error) {
	creds, err := ForceLogin(endpoint)
	if err != nil {
		return
	}
	username, err = ForceSaveLogin(creds)
	return
}
