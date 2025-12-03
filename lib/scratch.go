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
		InstanceUrl: org["LoginUrl"].(string),
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
	session.ClientId = "PlatformCLI"
	return
}

// Use the AuthCode for a new Scratch Org from the Dev Hub to get a new
// ForceSession
func (s *ScratchOrg) getSession() (session ForceSession, err error) {
	attrs := url.Values{}
	attrs.Set("grant_type", "authorization_code")
	attrs.Set("code", s.AuthCode)
	attrs.Set("client_id", "PlatformCLI")
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

// DeploySettings deploys settings metadata to a scratch org
func (f *Force) DeploySettings(settings []string) error {
	files := buildSettingsMetadata(settings)

	// Deploy the metadata
	options := ForceDeployOptions{}
	result, err := f.Metadata.Deploy(files, options)
	if err != nil {
		return fmt.Errorf("settings deployment failed: %w", err)
	}

	// Check if deployment succeeded
	if !result.Success {
		errorMsg := fmt.Sprintf("Settings deployment failed with status: %s", result.Status)
		if result.ErrorMessage != "" {
			errorMsg += fmt.Sprintf(": %s", result.ErrorMessage)
		}
		if len(result.Details.ComponentFailures) > 0 {
			errorMsg += "\nComponent failures:"
			for _, failure := range result.Details.ComponentFailures {
				errorMsg += fmt.Sprintf("\n  - %s: %s", failure.FileName, failure.Problem)
			}
		}
		return errors.New(errorMsg)
	}

	return nil
}

func buildSettingsMetadata(settings []string) ForceMetadataFiles {
	files := make(ForceMetadataFiles)

	// Create package.xml at root of zip
	packageXml := `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>*</members>
        <name>Settings</name>
    </types>
    <version>64.0</version>
</Package>`
	files["unpackaged/package.xml"] = []byte(packageXml)

	// Create settings file for each requested setting
	apexSettings := false
	userManagementSettings := false

	for _, setting := range settings {
		switch setting {
		case "enableEnhancedNotes":
			enhancedNotesSettings := `<?xml version="1.0" encoding="UTF-8"?>
<EnhancedNotesSettings xmlns="http://soap.sforce.com/2006/04/metadata">
    <enableEnhancedNotes>true</enableEnhancedNotes>
    <enableTasksOnEnhancedNotes>true</enableTasksOnEnhancedNotes>
</EnhancedNotesSettings>`
			files["unpackaged/settings/EnhancedNotes.settings"] = []byte(enhancedNotesSettings)
		case "enableQuote":
			quoteSettings := `<?xml version="1.0" encoding="UTF-8"?>
<QuoteSettings xmlns="http://soap.sforce.com/2006/04/metadata">
    <enableQuote>true</enableQuote>
</QuoteSettings>`
			files["unpackaged/settings/Quote.settings"] = []byte(quoteSettings)
		case "networksEnabled":
			communitiesSettings := `<?xml version="1.0" encoding="UTF-8"?>
<CommunitiesSettings xmlns="http://soap.sforce.com/2006/04/metadata">
    <enableNetworksEnabled>true</enableNetworksEnabled>
</CommunitiesSettings>`
			files["unpackaged/settings/Communities.settings"] = []byte(communitiesSettings)
		case "enableApexApprovalLockUnlock":
			apexSettings = true
		case "permsetsInFieldCreation":
			userManagementSettings = true
		}
	}

	if apexSettings {
		var apexBuffer bytes.Buffer
		apexBuffer.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<ApexSettings xmlns="http://soap.sforce.com/2006/04/metadata">
    <enableApexApprovalLockUnlock>true</enableApexApprovalLockUnlock>
</ApexSettings>`)
		files["unpackaged/settings/Apex.settings"] = apexBuffer.Bytes()
	}

	if userManagementSettings {
		var userMgmtBuffer bytes.Buffer
		userMgmtBuffer.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<UserManagementSettings xmlns="http://soap.sforce.com/2006/04/metadata">
    <permsetsInFieldCreation>true</permsetsInFieldCreation>
</UserManagementSettings>`)
		files["unpackaged/settings/UserManagement.settings"] = userMgmtBuffer.Bytes()
	}

	return files
}

// Create a new Scratch Org from a Dev Hub Org
func (f *Force) CreateScratchOrg() (id string, err error) {
	return f.CreateScratchOrgWithUser("")
}

func (f *Force) CreateScratchOrgWithUser(username string) (id string, err error) {
	return f.CreateScratchOrgWithUserAndFeatures(username, []string{})
}

func (f *Force) CreateScratchOrgWithUserAndFeatures(username string, features []string) (id string, err error) {
	return f.CreateScratchOrgWithUserFeaturesAndEdition(username, features, "")
}

func (f *Force) CreateScratchOrgWithUserFeaturesAndEdition(username string, features []string, edition string) (id string, err error) {
	return f.CreateScratchOrgWithUserFeaturesEditionAndSettings(username, features, edition, []string{})
}

func (f *Force) CreateScratchOrgWithUserFeaturesEditionAndSettings(username string, features []string, edition string, settings []string) (id string, err error) {
	params := make(map[string]string)
	params["ConnectedAppCallbackUrl"] = "http://localhost:1717/OauthRedirect"
	params["ConnectedAppConsumerKey"] = "PlatformCLI"
	params["Country"] = "US"

	if edition != "" {
		params["Edition"] = edition
	} else {
		params["Edition"] = "Developer"
	}

	baseFeatures := "AuthorApex;API;AddCustomApps:30;AddCustomTabs:30;ForceComPlatform;Sites;CustomerSelfService"
	if len(features) > 0 {
		for _, feature := range features {
			baseFeatures += ";" + feature
		}
	}
	params["Features"] = baseFeatures

	// Note: settings parameter is kept for later deployment after org creation

	params["OrgName"] = "Force CLI Scratch"
	if username != "" {
		params["Username"] = username
	}
	id, err, messages := f.CreateRecord("ScratchOrgInfo", params)
	if err != nil {
		if len(messages) == 1 && messages[0].ErrorCode == "NOT_FOUND" {
			return "", DevHubOrgRequiredError
		}
		return
	}
	return
}
