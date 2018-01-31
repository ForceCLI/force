package lib

import (
	"encoding/json"
	"fmt"
	. "github.com/ForceCLI/force/error"
	"os"
	"os/exec"
)

type SFDXAuth struct {
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

func UseSFDXSession(authData SFDXAuth) {
	creds := ForceSession{
		AccessToken:   authData.AccessToken,
		InstanceUrl:   authData.InstanceUrl,
		ForceEndpoint: EndpointCustom,
		UserInfo: &UserInfo{
			OrgId: authData.Id,
		},
		SessionOptions: &SessionOptions{
			ApiVersion:    ApiVersionNumber(),
			RefreshMethod: RefreshSFDX,
			Alias:         authData.Alias,
		},
	}
	ForceSaveLogin(creds, os.Stderr)
}

func GetSFDXAuth(user string) (auth SFDXAuth, err error) {
	fmt.Println("Getting SFDX AUTH FOR " + user)
	cmd := exec.Command("sfdx", "force:org:display", "-u"+user, "--json")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	err = cmd.Start()
	if err != nil {
		return
	}

	type authData struct {
		Result SFDXAuth
	}
	var aData authData
	if err := json.NewDecoder(stdout).Decode(&aData); err != nil {
		ErrorAndExit(err.Error())
	}
	if err := cmd.Wait(); err != nil {
		ErrorAndExit(err.Error())
	}
	auth = aData.Result
	return
}
