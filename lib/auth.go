package lib

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	. "github.com/ForceCLI/force/config"
	. "github.com/ForceCLI/force/error"
)

func (f *Force) UserInfo() (userinfo UserInfo, err error) {
	url := fmt.Sprintf("%s/services/oauth2/userinfo", f.Credentials.InstanceUrl)
	login, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
	if err != nil {
		return
	}
	err = json.Unmarshal(login, &userinfo)
	return
}

var getUserInfoFn func(*ForceSession) (UserInfo, error)

func init() {
	getUserInfoFn = fetchUserInfo
}

func fetchUserInfo(creds *ForceSession) (userinfo UserInfo, err error) {
	if creds == nil {
		return userinfo, fmt.Errorf("nil force session")
	}
	force := NewForce(creds)
	userinfo, err = force.UserInfo()
	if err != nil {
		return
	}
	me, err := force.GetRecord("User", userinfo.UserId)
	if err != nil {
		Log.Info("Problem getting user data, continuing...")
		err = nil
	}
	userinfo.ProfileId = fmt.Sprintf("%s", me["ProfileId"])

	namespace, err := force.getOrgNamespace()
	if err == nil {
		userinfo.OrgNamespace = namespace
	} else {
		Log.Info("Your profile does not have Modify All Data enabled. Functionallity will be limited.")
		err = nil
	}
	return
}

func (f *Force) getOrgNamespace() (namespace string, err error) {
	describe, err := f.Metadata.DescribeMetadata()
	if err != nil {
		return
	}
	namespace = describe.NamespacePrefix
	return
}

// Save the credentials as the active session with the UserInfo and with the
// default current API version.
func ForceSaveLogin(creds ForceSession, output *os.File) (string, error) {
	return forceSaveLogin(creds, output, true)
}

func forceSaveLogin(creds ForceSession, output *os.File, log bool) (sessionName string, err error) {
	userinfo, err := getUserInfoFn(&creds)
	if err != nil {
		return
	}
	creds.UserInfo = &userinfo
	if creds.SessionOptions == nil {
		creds.SessionOptions = &SessionOptions{}
	}
	creds.SessionOptions.ApiVersion = ApiVersionNumber()

	if log {
		Log.Info(fmt.Sprintf("Logged in as '%s' (API %s)\n", creds.UserInfo.UserName, ApiVersionNumber()))
	}

	if err = SaveLogin(creds); err != nil {
		return
	}
	sessionName = creds.SessionName()
	err = SetActiveLogin(sessionName)
	return
}

func (creds *ForceSession) SessionName() string {
	sessionName := creds.UserInfo.UserName
	if creds.SessionOptions.Alias != "" {
		sessionName = creds.SessionOptions.Alias
	}
	return sessionName
}

func SaveLogin(creds ForceSession) (err error) {
	body, err := json.Marshal(creds)
	if err != nil {
		return
	}
	sessionName := creds.SessionName()
	err = Config.Save("accounts", sessionName, string(body))
	return
}

func ForceLoginAndSaveSoap(endpoint ForceEndpoint, user_name string, password string, output *os.File) (username string, err error) {
	Log.Info("Deprecated call to ForceLoginAndSaveSoap.  Use ForceSoapLoginAtEndpointAndSaveSoap.")
	url := endpointUrl(endpoint)
	return ForceLoginAtEndpointAndSaveSoap(url, username, password, output)
}

func ForceLoginAtEndpointAndSaveSoap(endpoint string, user_name string, password string, output *os.File) (username string, err error) {
	creds, err := ForceSoapLoginAtEndpoint(endpoint, user_name, password)
	if err != nil {
		return
	}

	username, err = ForceSaveLogin(creds, output)
	return
}

// Create a new scratch org, login, and make it active
func ForceScratchLoginAndSave(output *os.File) (username string, err error) {
	return ForceScratchCreateLoginAndSave("", []string{}, "", output)
}

func ForceScratchCreateLoginAndSave(scratchUser string, features []string, edition string, output *os.File) (username string, err error) {
	force, err := ActiveForce()
	if err != nil {
		err = errors.New("You must be logged into a Dev Hub org to authenticate as a scratch org user.")
		return
	}
	fmt.Fprintln(os.Stderr, "Creating new Scratch Org...")
	scratchOrgId, err := force.CreateScratchOrgWithUserFeaturesAndEdition(scratchUser, features, edition)
	if err != nil {
		return
	}
	session, err := force.ForceLoginNewScratch(scratchOrgId)
	if err != nil {
		return
	}
	username, err = ForceSaveLogin(session, output)
	return
}

func ForceLoginAndSave(endpoint ForceEndpoint, output *os.File) (username string, err error) {
	Log.Info("Deprecated call to ForceLoginAndSave.  Use ForceLoginAtEndpointAndSave.")
	url := endpointUrl(endpoint)
	return ForceLoginAtEndpointAndSave(url, output)
}

func ForceLoginAtEndpointAndSave(endpoint string, output *os.File) (username string, err error) {
	return ForceLoginAtEndpointWithPromptAndSave(endpoint, output, "login")
}

func ForceLoginAtEndpointAndSaveWithPort(endpoint string, output *os.File, port int) (username string, err error) {
	return ForceLoginAtEndpointWithPromptAndSaveWithPort(endpoint, output, "login", port)
}

func ForceLoginAtEndpointWithPromptAndSave(endpoint string, output *os.File, prompt string) (username string, err error) {
	creds, err := ForceLoginAtEndpointWithPrompt(endpoint, prompt)
	if err != nil {
		return
	}
	username, err = ForceSaveLogin(creds, output)
	return
}

func ForceLoginAtEndpointWithPromptAndSaveWithPort(endpoint string, output *os.File, prompt string, port int) (username string, err error) {
	creds, err := ForceLoginAtEndpointWithPromptAndPort(endpoint, prompt, port)
	if err != nil {
		return
	}
	username, err = ForceSaveLogin(creds, output)
	return
}

// UpdateCredentials calls CopyCredentialAuthFields and persists the new credentials in config.
func (f *Force) UpdateCredentials(creds ForceSession) {
	f.CopyCredentialAuthFields(&creds)
	forceSaveLogin(*f.Credentials, os.Stderr, false)
}

// CopyCredentialAuthFields copies auth fields from creds into the receiver's Credentials.
func (f *Force) CopyCredentialAuthFields(creds *ForceSession) {
	f.Credentials.AccessToken = creds.AccessToken
	f.Credentials.IssuedAt = creds.IssuedAt
	f.Credentials.InstanceUrl = creds.InstanceUrl
	f.Credentials.Scope = creds.Scope
}

func ActiveForce() (force *Force, err error) {
	creds, err := ActiveCredentials(true)
	if err != nil {
		return
	}
	force = NewForce(&creds)
	return
}

func GetForce(accountName string) (force *Force, err error) {
	creds, err := GetAccountCredentials(accountName)
	if err != nil {
		return
	}
	force = NewForce(&creds)
	return
}

// Add UserInfo and SessionOptions to old ForceSession
func upgradeCredentials(creds *ForceSession) (err error) {
	if creds.SessionOptions != nil && creds.UserInfo != nil && creds.EndpointUrl != "" {
		return
	}
	if creds.SessionOptions == nil {
		creds.SessionOptions = &SessionOptions{
			ApiVersion: ApiVersionNumber(),
		}
		if creds.RefreshToken != "" {
			creds.SessionOptions.RefreshMethod = RefreshOauth
		}
	}
	if creds.UserInfo == nil || creds.UserInfo.UserName == "" {
		force := NewForce(creds)
		err = force.RefreshSession()
		if err != nil {
			return
		}
		var userinfo UserInfo
		userinfo, err = getUserInfoFn(creds)
		if err != nil {
			return
		}
		creds.UserInfo = &userinfo
		_, err = ForceSaveLogin(*creds, os.Stderr)
		if err != nil {
			return
		}
	}
	if creds.EndpointUrl == "" {
		switch creds.ForceEndpoint {
		case EndpointProduction, EndpointTest, EndpointPrerelease, EndpointMobile1:
			creds.EndpointUrl = endpointUrl(creds.ForceEndpoint)
		default:
			creds.EndpointUrl = creds.InstanceUrl
		}
		Log.Info(fmt.Sprintf("Updated Endpoint URL in session to %s", creds.EndpointUrl))
		err = SaveLogin(*creds)
	}
	return
}

func GetAccountCredentials(accountName string) (creds ForceSession, err error) {
	data, err := Config.Load("accounts", accountName)
	if err != nil {
		var sfdxAuth SFDXAuth
		if sfdxAuth, err = GetSFDXAuth(accountName); err == nil {
			if sfdxAuth.Alias == "" && accountName != "" && !strings.EqualFold(accountName, sfdxAuth.Username) {
				sfdxAuth.Alias = accountName
			}
			creds = SFDXAuthToForceSession(sfdxAuth)
			aliasHint := firstNonEmpty(sfdxAuth.Alias, accountName)
			if err = CompleteSFDXSession(&creds, aliasHint); err != nil {
				return creds, err
			}
			if creds.SessionOptions != nil && creds.SessionOptions.ApiVersion != "" && creds.SessionOptions.ApiVersion != ApiVersionNumber() {
				_ = SetApiVersion(creds.SessionOptions.ApiVersion)
			}
			err = nil
			return
		}
		err = fmt.Errorf("Could not find account, %s.  Please log in first.", accountName)
		return
	}
	err = json.Unmarshal([]byte(data), &creds)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	err = upgradeCredentials(&creds)
	if err != nil {
		// Couldn't update the credentials.  Force re-login.
		_ = Config.Delete("accounts", accountName)
		_ = Config.DeleteLocalOrGlobal("current", "account")
		ErrorAndExit("Cannot update stored session.  Please log in again.")
	}
	if creds.SessionOptions.ApiVersion != "" && creds.SessionOptions.ApiVersion != ApiVersionNumber() {
		SetApiVersion(creds.SessionOptions.ApiVersion)
	}
	return
}

func ActiveCredentials(requireCredentials bool) (creds ForceSession, err error) {
	account, err := ActiveLogin()
	if requireCredentials && (err != nil || strings.TrimSpace(account) == "") {
		ErrorAndExit("Please login before running this command.")
	}
	creds, err = GetAccountCredentials(strings.TrimSpace(account))
	if requireCredentials && err != nil {
		ErrorAndExit("Failed to load credentials. %v", err)
	}
	if !requireCredentials && err != nil {
		err = nil
	}
	return
}

func ActiveLogin() (account string, err error) {
	account, err = Config.LoadLocalOrGlobal("current", "account")
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

func ForceLoginAtEndpointAndSaveDeviceFlow(endpoint string, output *os.File, prompt string) (username string, err error) {
	creds, err := ForceLoginAtEndpointDeviceFlow(endpoint, prompt)
	if err != nil {
		return
	}
	username, err = ForceSaveLogin(creds, output)
	return
}
