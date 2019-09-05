package lib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
)

type ScratchOrg struct {
	UserName    string
	InstanceUrl string
	AuthCode    string
}

type AuthCodeSession struct {
	ForceSession
	RefreshToken string `json:"refresh_token"`
}

func (s *ScratchOrg) tokenURL() string {
	return fmt.Sprintf("%s/services/oauth2/token", s.InstanceUrl)
}

func (f *Force) getScratchOrg(scratchOrgId string) (scratchOrg ScratchOrg, err error) {
	org, err := f.GetRecord("ScratchOrgInfo", scratchOrgId)
	if err != nil {
		err = errors.New("Unable to query scratch orgs.  You must be logged into a Dev Hub org.")
		return
	}
	scratchOrg = ScratchOrg{
		UserName:    org["SignupUsername"].(string),
		InstanceUrl: fmt.Sprintf("https://%s.salesforce.com", org["SignupInstance"].(string)),
		AuthCode:    org["AuthCode"].(string),
	}
	return
}

// Log into a Scratch Org
func (f *Force) ForceLoginNewScratch(scratchOrgId string) (session ForceSession, err error) {
	scratchOrg, err := f.getScratchOrg(scratchOrgId)
	if err != nil {
		return
	}
	session, err = scratchOrg.getSession()
	if err != nil {
		return
	}
	session.SessionOptions = &SessionOptions{
		RefreshMethod: RefreshOauth,
	}
	session.ForceEndpoint = EndpointTest
	session.ClientId = "SalesforceDevelopmentExperience"
	return
}

// Use the AuthCode for a new Scratch Org from the Dev Hub to get a new
// ForceSession
func (s *ScratchOrg) getSession() (session ForceSession, err error) {
	attrs := url.Values{}
	attrs.Set("grant_type", "authorization_code")
	attrs.Set("code", s.AuthCode)
	attrs.Set("client_id", "SalesforceDevelopmentExperience")
	attrs.Set("client_secret", "1384510088588713504")
	attrs.Set("redirect_uri", "http://localhost:1717/OauthRedirect")

	postVars := attrs.Encode()
	req, err := httpRequest("POST", s.tokenURL(), bytes.NewReader([]byte(postVars)))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	fmt.Fprintln(os.Stderr, "Logging into scratch org")
	res, err := doRequest(req)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if res.StatusCode != 200 {
		var oauthError OAuthError
		json.Unmarshal(body, &oauthError)
		err = errors.New("Unable to log into scratch org: " + oauthError.ErrorDescription)
		return
	}
	if err != nil {
		return
	}
	var authSession AuthCodeSession

	err = json.Unmarshal(body, &authSession)
	if err != nil {
		return
	}

	session = authSession.ForceSession
	session.RefreshToken = authSession.RefreshToken

	return session, nil
}

// Create a new Scratch Org from a Dev Hub Org
func (f *Force) CreateScratchOrg() (id string, err error) {
	params := make(map[string]string)
	params["ConnectedAppCallbackUrl"] = "http://localhost:1717/OauthRedirect"
	params["ConnectedAppConsumerKey"] = "SalesforceDevelopmentExperience"
	params["Country"] = "US"
	params["Edition"] = "Developer"
	params["Features"] = "AuthorApex;API;AddCustomApps:30;AddCustomTabs:30;ForceComPlatform;Sites;CustomerSelfService"
	params["OrgName"] = "Force CLI Scratch"
	id, err, messages := f.CreateRecord("ScratchOrgInfo", params)
	if err != nil {
		if len(messages) == 1 && messages[0].ErrorCode == "NOT_FOUND" {
			return "", DevHubOrgRequiredError
		}
		return
	}
	return
}
