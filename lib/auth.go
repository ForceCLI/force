package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	. "github.com/heroku/force/config"
	. "github.com/heroku/force/error"
)

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

func ForceSaveLogin(creds ForceCredentials, output *os.File) (username string, err error) {
	force := NewForce(&creds)
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
		fmt.Fprintln(output, "Problem getting user data, continuing...")
		//return
	}
	fmt.Fprintf(output, "Logged in as '%s' (API %s)\n", me["Username"], apiVersion)
	title := fmt.Sprintf("\033];%s\007", me["Username"])
	creds.ProfileId = fmt.Sprintf("%s", me["ProfileId"])
	creds.ApiVersion = strings.TrimPrefix(apiVersion, "v")
	fmt.Fprintf(output, title)

	describe, err := force.Metadata.DescribeMetadata()

	if err == nil {
		creds.Namespace = describe.NamespacePrefix
	} else {
		fmt.Fprintf(output, "Your profile does not have Modify All Data enabled. Functionallity will be limited.\n")
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

func ForceLoginAndSaveSoap(endpoint ForceEndpoint, user_name string, password string, output *os.File) (username string, err error) {
	creds, err := ForceSoapLogin(endpoint, user_name, password)
	if err != nil {
		return
	}

	username, err = ForceSaveLogin(creds, output)
	//fmt.Printf("Creds %+v", creds)
	return
}

func ForceLoginAndSave(endpoint ForceEndpoint, output *os.File) (username string, err error) {
	creds, err := ForceLogin(endpoint)
	if err != nil {
		return
	}
	username, err = ForceSaveLogin(creds, output)
	return
}

func (f *Force) UpdateCredentials(creds ForceCredentials) {
	f.Credentials.AccessToken = creds.AccessToken
	f.Credentials.IssuedAt = creds.IssuedAt
	f.Credentials.InstanceUrl = creds.InstanceUrl
	f.Credentials.Scope = creds.Scope
	f.Credentials.Id = creds.Id
	ForceSaveLogin(*f.Credentials, os.Stderr)
}

func ActiveForce() (force *Force, err error) {
	creds, err := ActiveCredentials(true)
	if err != nil {
		return
	}
	force = NewForce(&creds)
	return
}

func ActiveCredentials(requireCredentials bool) (creds ForceCredentials, err error) {
	account, err := ActiveLogin()
	if requireCredentials && (err != nil || strings.TrimSpace(account) == "") {
		ErrorAndExit("Please login before running this command.")
	}
	data, err := Config.Load("accounts", strings.TrimSpace(account))
	if requireCredentials && err != nil {
		ErrorAndExit("Failed to load credentials. %v", err)
	}
	if err == nil {
		_ = json.Unmarshal([]byte(data), &creds)
		if creds.ApiVersion != "" {
			apiVersionNumber = creds.ApiVersion
			apiVersion = "v" + apiVersionNumber
		}
	}
	if !requireCredentials && err != nil {
		err = nil
	}
	return
}

func ActiveLogin() (account string, err error) {
	account, err = Config.Load("current", "account")
	if err != nil {
		accounts, _ := Config.List("accounts")
		if len(accounts) > 0 {
			SetActiveLoginDefault()
		}
	}
	return
}

func SetActiveLoginDefault() (account string) {
	accounts, _ := Config.List("accounts")
	if len(accounts) > 0 {
		account = accounts[0]
		SetActiveLogin(account)
	}
	return
}

func SetActiveLogin(account string) (err error) {
	err = Config.Save("current", "account", account)
	return
}

func SetActiveCreds(authData UserAuth) {
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
