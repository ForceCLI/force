package main

import (
	"encoding/json"
	"net/url"
)

var cmdLogin = &Command{
	Run:   runLogin,
	Usage: "login",
	Short: "Log in to force.com",
	Long: `
Log in to force.com

Examples:

  force login     		 # log in to production or developer org

  force login test 		 # log in to sandbox org

  force login pre  		 # log in to prerelease org
  
  force login un pw 	 # log in using SOAP

  force login test un pw # log in using SOAP to sandbox org

  force login na1-blitz01.soma.salesforce.com un pw #internal only
`,
}

func runLogin(cmd *Command, args []string) {
	var endpoint ForceEndpoint
	var username, password string
	endpoint = EndpointProduction
	if len(args) > 0 {
		switch args[0] {
		case "test":
			endpoint = EndpointTest
		case "pre":
			endpoint = EndpointPrerelease
		default:
			// need to determine the form of the endpoint
			uri, err := url.Parse(args[0])
			if err != nil {
				ErrorAndExit("no such endpointx: %s", args[0])
			}
			// Could be short hand?
			if uri.Host == "" {
				uri, err = url.Parse("https://" + args[0])
				if err != nil {
					ErrorAndExit("no such endpoint: %s", args[0])
				}
			}
			CustomEndpoint = uri.Scheme + "://" + uri.Host
			endpoint = EndpointCustom
		}
	}
	if len(args) > 1 {
		if len(args) == 2 {
			// username and password option
			username = args[0]
			password = args[1]
		} else if len(args) == 3 {
			// endpoint, username, password
			username = args[1]
			password = args[2]
		}
		_, err := ForceLoginAndSaveSoap(endpoint, username, password)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	} else {
		_, err := ForceLoginAndSave(endpoint)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
}

var cmdLogout = &Command{
	Run:   runLogout,
	Usage: "logout <account>",
	Short: "Log out from force.com",
	Long: `
Log out from force.com

Examples:

  force logout user@example.org
`,
}

func runLogout(cmd *Command, args []string) {
	if len(args) != 1 {
		ErrorAndExit("must specify account to log out")
	}
	account := args[0]
	Config.Delete("accounts", account)
	if active, _ := Config.Load("current", "account"); active == account {
		Config.Delete("current", "account")
		SetActiveAccountDefault()
	}
}
func ForceLoginAndSaveSoap(endpoint ForceEndpoint, user_name string, password string) (username string, err error) {
	creds, err := ForceSoapLogin(endpoint, user_name, password)
	if err != nil {
		return
	}
	force := NewForce(creds)
	login, err := force.Get(creds.Id)
	if err != nil {
		return
	}
	body, err := json.Marshal(creds)
	if err != nil {
		return
	}
	username = login["username"].(string)
	Config.Save("accounts", username, string(body))
	Config.Save("current", "account", username)
	return
}

func ForceLoginAndSave(endpoint ForceEndpoint) (username string, err error) {
	creds, err := ForceLogin(endpoint)
	if err != nil {
		return
	}
	force := NewForce(creds)
	login, err := force.Get(creds.Id)
	if err != nil {
		return
	}
	body, err := json.Marshal(creds)
	if err != nil {
		return
	}
	username = login["username"].(string)
	Config.Save("accounts", username, string(body))
	Config.Save("current", "account", username)
	return
}
