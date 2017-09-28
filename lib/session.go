package lib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/heroku/force/error"
	"io/ioutil"
	"net/url"
	"os"
)

func (f *Force) refreshOauth() (err error) {
	attrs := url.Values{}
	attrs.Set("grant_type", "refresh_token")
	attrs.Set("refresh_token", f.Credentials.RefreshToken)
	attrs.Set("client_id", ClientId)
	attrs.Set("format", "json")

	postVars := attrs.Encode()
	req, err := httpRequest("POST", f.refreshTokenURL(), bytes.NewReader([]byte(postVars)))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	fmt.Fprintln(os.Stderr, "Refreshing Session Token")
	res, err := doRequest(req)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if res.StatusCode != 200 {
		err = errors.New("Failed to refresh session.  Please run `force login`.")
		return
	}
	if err != nil {
		return
	}

	var result ForceSession
	json.Unmarshal(body, &result)
	f.UpdateCredentials(result)
	return
}

func (f *Force) refreshSFDX() (err error) {
	fmt.Fprintln(os.Stderr, "Refreshing Session Token Using SFDX")
	sfdxAuth, err := GetSFDXAuth(f.Credentials.UserInfo.UserName)
	if err != nil {
		return
	}
	newCreds := ForceSession{
		AccessToken: sfdxAuth.AccessToken,
		InstanceUrl: sfdxAuth.InstanceUrl,
	}
	f.UpdateCredentials(newCreds)
	return
}

func (f *Force) RefreshSessionOrExit() {
	err := f.RefreshSession()
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func (f *Force) RefreshSession() (err error) {
	if f.Credentials.SessionOptions.RefreshMethod == RefreshOauth {
		err = f.refreshOauth()
	} else if f.Credentials.SessionOptions.RefreshMethod == RefreshSFDX {
		err = f.refreshSFDX()
	} else {
		err = errors.New("Unable to refresh.  Please run `force login`.")
	}
	return
}
