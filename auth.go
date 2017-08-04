package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
)

var cmdUseDXAuth = &Command{
	Run:   runUseDXAuth,
	Usage: "usedxauth [dx-username or alias]",
	Short: "Authenticate with SFDX Scratch Org User",
	Long: `
Authenticate with SFDX Scratch Org User. If a user or alias is passed to the command then an attempt is made to find that user authentication info.  If no user or alias is passed an attempt is made to find the default user based on sfdx config.

Examples:

  force usedxauth test-d1df0gyckgpr@dcarroll_company.net
  force usedxauth ScratchUserAlias
  force usedxauth
`,
}

type UserAuth struct {
	AccessToken string
	Alias       string
	ClientId    string
	CreatedBy   string
	DevHubId    string
	Edition     string
	Id          string
	InstanceUrl string
	OrgName     string
	Password    string
	Status      string
	Username    string
}

func init() {
}

func runUseDXAuth(cmd *Command, args []string) {
	var auth map[string]interface{}
	var err error
	if len(args) == 0 {
		fmt.Println("Determining default user...")
		auth, _ = getDefaultItem()
	} else {
		user := args[0]
		fmt.Printf("Looking for %s in DX orgs...\n", user)
		auth, err = getOrgListItem(user)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}

	connStatus := fmt.Sprintf("%s", auth["connectedStatus"])
	username := fmt.Sprintf("%s", auth["username"])
	alias := fmt.Sprintf("%s", auth["alias"])
	if connStatus == "Connected" || connStatus == "Unknown" {
		authData := getSFDXAuth(username)
		authData.Alias = alias
		authData.Username = username
		setActiveCreds(authData)
		if len(authData.Alias) > 0 {
			fmt.Printf("Now using DX credentials for %s (%s)\n", username, alias)
		} else {
			fmt.Printf("Now using DX credentials for %s\n", username)
		}
	} else {
		ErrorAndExit("Could not determine connection status for %s", username)
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

func getDefaultItem() (data map[string]interface{}, err error) {
	md, err := getOrgList()
	defUsers, err := findDefaultUserInOrgList(md)

	if len(defUsers) == 0 {
		ErrorAndExit("No default user logins found")
	}

	if len(defUsers) == 1 {
		data = defUsers[0]
	} else {
		var hubUser map[string]interface{}
		var scrUser map[string]interface{}
		for _, y := range defUsers {
			if y["defaultMarker"] == "(D)" {
				hubUser = y
			} else {
				scrUser = y
			}
		}
		if inProjectDir() == true {
			data = scrUser
		} else {
			data = hubUser
		}
	}
	if len(fmt.Sprintf("%s", data["alias"])) > 0 {
		fmt.Printf("Getting auth for %s (%s)...\n", data["username"], data["alias"])
	} else {
		fmt.Printf("Getting auth for %s\n...", data["username"])
	}
	return
}

func inProjectDir() bool {
	dir, err := os.Getwd()
	_, err = os.Stat(path.Join(dir, ".sfdx"))

	return err == nil
}

func getOrgListItem(user string) (data map[string]interface{}, err error) {
	md, err := getOrgList()
	data, err = findUserInOrgList(user, md)
	return
}

func getOrgList() (data map[string]interface{}, err error) {
	cmd := exec.Command("sfdx", "force:org:list", "--json")
	stdout, err := cmd.StdoutPipe()

	if err != nil {
		ErrorAndExit(err.Error())
	}
	if err := cmd.Start(); err != nil {
		ErrorAndExit(err.Error())
	}
	type Orgs struct {
		NonScratchOrgs map[string]interface{}
		ScratchOrgs    map[string]interface{}
	}

	if err := json.NewDecoder(stdout).Decode(&data); err != nil {
		ErrorAndExit(err.Error())
	}
	if err := cmd.Wait(); err != nil {
		ErrorAndExit(err.Error())
	}

	return
}

func findUserInOrgList(user string, md map[string]interface{}) (data map[string]interface{}, err error) {
	for k, v := range md {
		fmt.Println(v)
		switch vv := v.(type) {
		case float64:
		case interface{}:
			for _, u := range vv.(map[string]interface{}) {
				for _, y := range u.([]interface{}) {
					auth := y.(map[string]interface{})
					//check if user matches alias or username
					if auth["username"] == user || auth["alias"] == user {
						if len(fmt.Sprintf("%s", auth["alias"])) > 0 {
							fmt.Printf("Getting auth for %s (%s)...\n", auth["username"], auth["alias"])
						} else {
							fmt.Printf("Getting auth for %s\n...", auth["username"])
						}
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

func findDefaultUserInOrgList(md map[string]interface{}) (data []map[string]interface{}, err error) {
	for k, v := range md {
		switch vv := v.(type) {
		case float64:
		case interface{}:
			for _, u := range vv.(map[string]interface{}) {
				for _, y := range u.([]interface{}) {
					auth := y.(map[string]interface{})
					//check if user matches alias or username
					if auth["isDefaultUsername"] == true || auth["isDefaultDevHubUsername"] == true {
						// Add auth to slice
						data = append(data, auth)
					}
				}
			}
		default:
			fmt.Println(k, "is of a type I don't know how to handle")
		}
	}
	if len(data) == 0 {
		err = fmt.Errorf("Could not find a default user")
	}
	return
}

func getSFDXAuth(user string) (auth UserAuth) {
	cmd := exec.Command("sfdx", "force:org:display", "-u"+user, "--json")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if err := cmd.Start(); err != nil {
		ErrorAndExit(err.Error())
	}

	type authData struct {
		Result UserAuth
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
