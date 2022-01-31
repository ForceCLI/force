package lib

import (
	"bytes"
	"encoding/json"
	"errors"
	. "github.com/ForceCLI/force/error"
	"io/ioutil"
	"net/url"
)

var SessionRefreshError = errors.New("Failed to refresh session.  Please run `force login`.")
var SessionRefreshUnavailable = errors.New("Unable to refresh.  Please run `force login`.")

func (f *Force) refreshOauth() (err error) {
	attrs := url.Values{}
	attrs.Set("grant_type", "refresh_token")
	attrs.Set("refresh_token", f.Credentials.RefreshToken)
	attrs.Set("client_id", ClientId)
	if f.Credentials.ClientId != "" {
		attrs.Set("client_id", f.Credentials.ClientId)
	}
	attrs.Set("format", "json")

	postVars := attrs.Encode()
	req, err := httpRequest("POST", f.refreshTokenURL(), bytes.NewReader([]byte(postVars)))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	Log.Info("Refreshing Session Token")
	res, err := doRequest(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if res.StatusCode != 200 {
		err = SessionRefreshError
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
	Log.Info("Refreshing Session Token Using SFDX")
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

func (f *Force) RefreshSession() error {
	if f.Credentials.SessionOptions.RefreshFunc != nil {
		return f.Credentials.SessionOptions.RefreshFunc(f)
	} else if f.Credentials.SessionOptions.RefreshMethod == RefreshOauth {
		return f.refreshOauth()
	} else if f.Credentials.SessionOptions.RefreshMethod == RefreshSFDX {
		return f.refreshSFDX()
	}
	return SessionRefreshUnavailable
}
