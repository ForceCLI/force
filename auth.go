package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

var cmdAuth = &Command{
	Run:   runAuth,
	Usage: "auth [dx-username]",
	Short: "Authenticate with SFDX Scratch Org User",
	Long: `
Authenticate with SFDX Scratch Org User

Examples:

  force auth test-d1df0gyckgpr@dcarroll_company.net

`,
}

type UserAuth struct {
    AccessToken    string
    Alias          string
    ClientId       string
    CreatedBy      string
    DevHubId       string
    Edition        string
    Id             string
    InstanceUrl    string
    OrgName        string
    Password       string
    Status         string
    Username       string
}


func init() {
}

func runAuth(cmd *Command, args []string) {
	if len(args) != 1 {
		ErrorAndExit("Must provide either an alias or a username")
	}
	user := args[0]
	auth, err := getOrgListItem(user)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	mauth := auth.(map[string]interface{})
	connStatus := fmt.Sprintf("%s", mauth["connectedStatus"])
	fmt.Println("Connected Status:", connStatus)
	if connStatus == "Connected" || connStatus == "Unknown" {
		authData := getSFDXAuth(fmt.Sprintf("%s", mauth["username"]))
		authData.Alias =  fmt.Sprintf("%s", mauth["alias"])
		setActiveCreds(authData);
		fmt.Println("Now using credentials for", fmt.Sprintf("%s", mauth["username"]))
	} else {
		ErrorAndExit("Could not determine connection status for %s", user)
	}	
}

func setActiveCreds(authData UserAuth) {
	creds := ForceCredentials{}
	creds.AccessToken = authData.AccessToken
	creds.InstanceUrl = authData.InstanceUrl
	creds.IsCustomEP = true
	creds.ApiVersion = "40.0"
	creds.ForceEndpoint = 4
	creds.IsHourly = false
	creds.HourlyCheck = false

	body, err := json.Marshal(creds)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	configName := authData.Alias
	if len(configName) == 0 {
		configName = authData.Username
	}
	
	Config.Save("accounts", configName, string(body))
	Config.Save("current", "account", configName)
}

func getOrgListItem(user string)(data interface{}, err error) {
	cmd := exec.Command("sfdx", "force:org:list", "--json")
	stdout, err := cmd.StdoutPipe()

	if err != nil {
		ErrorAndExit(err.Error())
	}
	if err := cmd.Start(); err != nil {
		ErrorAndExit(err.Error())
	}
	type Orgs struct {
		NonScratchOrgs  map[string]interface{}
		ScratchOrgs 	map[string]interface{}
	}

	var md map[string]interface{}

	if err := json.NewDecoder(stdout).Decode(&md); err != nil {
		ErrorAndExit(err.Error())
	}
	if err := cmd.Wait(); err != nil {
		ErrorAndExit(err.Error())
	}
	for k, v := range md {
	    switch vv := v.(type) {
	    case float64:
	    case interface{}:
	    	for _, u := range vv.(map[string]interface{}) {
	    		for _, y := range u.([]interface{}) {
	    			auth := y.(map[string]interface{})
	    			//check if user matches alias or username
	    			if auth["username"] == user || auth["alias"] == user {
	    				fmt.Printf("Getting auth for %s\n", auth["username"])	    				
	    				data = auth
	    				err = nil
	    				return
	    			}
	    		}
	    	}
	    default:
	        fmt.Println(k, "is of a type I don't know how to handle")
	    }
	}
	err = fmt.Errorf("Could not find and alias or username that matches %s", user)
	return
}

func getSFDXAuth(user string)(auth UserAuth) {
	cmd := exec.Command("sfdx", "force:org:display", "-u" + user, "--json")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if err := cmd.Start(); err != nil {
		ErrorAndExit(err.Error())
	}

	type authData struct {
		Result 		UserAuth
	}
	var aData authData
	if err := json.NewDecoder(stdout).Decode(&aData); err != nil {
		ErrorAndExit(err.Error())
	}
	if err := cmd.Wait(); err != nil {
		ErrorAndExit(err.Error())
	}
	return aData.Result
}	
