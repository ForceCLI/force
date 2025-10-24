package lib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	. "github.com/ForceCLI/force/error"
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
	// Follow Redirects and re-POST upon a 302 response.
	res, err := doRequest(req, redirectPostOn302)
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
	if f.Credentials == nil {
		return fmt.Errorf("missing credentials for refresh")
	}

	username := ""
	if f.Credentials.UserInfo != nil {
		username = strings.TrimSpace(f.Credentials.UserInfo.UserName)
	}
	if username == "" && f.Credentials.SessionOptions != nil {
		username = strings.TrimSpace(f.Credentials.SessionOptions.Alias)
	}
	if username == "" {
		return fmt.Errorf("unable to determine SFDX username for refresh")
	}

	sfdxAuth, err := GetSFDXAuth(username)
	if err != nil {
		return
	}

	refreshed := SFDXAuthToForceSession(sfdxAuth)
	if strings.TrimSpace(refreshed.AccessToken) == "" {
		return fmt.Errorf("SFDX auth for %s missing access token", username)
	}

	if f.Credentials.SessionOptions == nil {
		f.Credentials.SessionOptions = &SessionOptions{}
	}
	if refreshed.SessionOptions != nil {
		if refreshed.SessionOptions.Alias != "" {
			f.Credentials.SessionOptions.Alias = refreshed.SessionOptions.Alias
		}
		if refreshed.SessionOptions.ApiVersion != "" {
			f.Credentials.SessionOptions.ApiVersion = refreshed.SessionOptions.ApiVersion
		}
	}

	if refreshed.EndpointUrl != "" {
		f.Credentials.EndpointUrl = refreshed.EndpointUrl
	}
	if refreshed.ClientId != "" {
		f.Credentials.ClientId = refreshed.ClientId
	}

	currentToken := strings.TrimSpace(f.Credentials.AccessToken)
	newToken := strings.TrimSpace(refreshed.AccessToken)
	currentInstance := strings.TrimSpace(f.Credentials.InstanceUrl)
	newInstance := strings.TrimSpace(refreshed.InstanceUrl)

	if currentToken == newToken && currentInstance == newInstance {
		// Try to use the Salesforce refresh token flow as a fallback.
		if strings.TrimSpace(refreshed.RefreshToken) == "" {
			return SessionRefreshError
		}

		originalMethod := f.Credentials.SessionOptions.RefreshMethod
		f.Credentials.SessionOptions.RefreshMethod = RefreshOauth
		f.Credentials.RefreshToken = refreshed.RefreshToken
		if refreshed.SessionOptions != nil && refreshed.SessionOptions.ApiVersion != "" {
			f.Credentials.SessionOptions.ApiVersion = refreshed.SessionOptions.ApiVersion
		}
		err = f.refreshOauth()
		f.Credentials.SessionOptions.RefreshMethod = originalMethod
		return err
	}

	updateFromSFDXSession(f, refreshed)
	return SaveLogin(*f.Credentials)
}

func updateFromSFDXSession(f *Force, refreshed ForceSession) {
	f.CopyCredentialAuthFields(&refreshed)

	if trimmed := strings.TrimSpace(refreshed.RefreshToken); trimmed != "" {
		f.Credentials.RefreshToken = trimmed
	}
	if trimmed := strings.TrimSpace(refreshed.ClientId); trimmed != "" {
		f.Credentials.ClientId = trimmed
	}
	if trimmed := strings.TrimSpace(refreshed.EndpointUrl); trimmed != "" {
		f.Credentials.EndpointUrl = trimmed
	}
	if trimmed := strings.TrimSpace(refreshed.Id); trimmed != "" {
		f.Credentials.Id = trimmed
	}

	if refreshed.UserInfo != nil {
		f.Credentials.UserInfo = refreshed.UserInfo
	}
	if f.Credentials.SessionOptions == nil {
		f.Credentials.SessionOptions = &SessionOptions{}
	}
	if refreshed.SessionOptions != nil {
		if refreshed.SessionOptions.ApiVersion != "" {
			f.Credentials.SessionOptions.ApiVersion = refreshed.SessionOptions.ApiVersion
		}
		if refreshed.SessionOptions.Alias != "" {
			f.Credentials.SessionOptions.Alias = refreshed.SessionOptions.Alias
		}
	}
	if f.Credentials.SessionOptions.RefreshMethod == RefreshUnavailable {
		f.Credentials.SessionOptions.RefreshMethod = RefreshSFDX
	}
}

func (f *Force) RefreshSessionOrExit() {
	err := f.RefreshSession()
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func (f *Force) RefreshSession() error {
	if f.Credentials.SessionOptions == nil {
		return SessionRefreshUnavailable
	}
	if f.Credentials.SessionOptions.RefreshFunc != nil {
		return f.Credentials.SessionOptions.RefreshFunc(f)
	} else if f.Credentials.SessionOptions.RefreshMethod == RefreshOauth {
		return f.refreshOauth()
	} else if f.Credentials.SessionOptions.RefreshMethod == RefreshSFDX {
		return f.refreshSFDX()
	}
	return SessionRefreshUnavailable
}
