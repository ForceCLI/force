package lib

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	. "github.com/ForceCLI/force/error"
)

var SessionRefreshError = errors.New("Failed to refresh session.  Please run `force login`.")
var SessionRefreshUnavailable = errors.New("Unable to refresh.  Please run `force login`.")

func (f *Force) refreshOauth() (err error) {
	result, err := f.requestOauthRefresh()
	if err != nil {
		return
	}
	f.UpdateCredentials(result)
	return
}

// RefreshTokenError is returned when the OAuth token endpoint refuses a
// refresh token.  It holds the reason the endpoint gave.  errors.Is reports
// it as SessionRefreshError, whose message it has.
type RefreshTokenError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *RefreshTokenError) Error() string {
	return SessionRefreshError.Error()
}

func (e *RefreshTokenError) Is(target error) bool {
	return target == SessionRefreshError
}

// RefreshAccessToken exchanges the refresh token for a new access token and
// updates the credentials in memory.  It does not save them.
func (f *Force) RefreshAccessToken() error {
	result, err := f.requestOauthRefresh()
	if err != nil {
		return err
	}
	f.CopyCredentialAuthFields(&result)
	return nil
}

func (f *Force) requestOauthRefresh() (result ForceSession, err error) {
	// A refresh token issued with a DPoP-bound token is bound to the same
	// key, so the refresh request needs a proof signed with it.
	key, err := f.Credentials.dpopPrivateKey()
	if err != nil {
		return
	}

	attrs := url.Values{}
	attrs.Set("grant_type", "refresh_token")
	if key != nil {
		// The hybrid_refresh grant also returns ui_sid, the unbound session
		// ID for the web UI, which a DPoP-bound access token can't be used
		// for.
		attrs.Set("grant_type", "hybrid_refresh")
	}
	attrs.Set("refresh_token", f.Credentials.RefreshToken)
	attrs.Set("client_id", ClientId)
	if f.Credentials.ClientId != "" {
		attrs.Set("client_id", f.Credentials.ClientId)
	}
	attrs.Set("format", "json")
	Log.Info("Refreshing Session Token")
	res, body, nonce, err := postTokenRequest(f.refreshTokenURL(), attrs, key, f.Credentials.dpopNonce())
	if err != nil {
		return
	}
	if res.StatusCode != 200 {
		refused := &RefreshTokenError{}
		json.Unmarshal(body, refused)
		err = refused
		return
	}

	json.Unmarshal(body, &result)
	if key != nil {
		result.DPoPNonce = nonce
		f.Credentials.rememberDPoPNonce(nonce)
	}
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
		f.Credentials.UserInfo = mergeRefreshedUserInfo(f.Credentials.UserInfo, refreshed.UserInfo)
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

// mergeRefreshedUserInfo keeps the fields an SFDX auth file can lack. An alias
// can be pointed at another login between refreshes, so the fields are only
// kept when the refreshed session is for the same user in the same org.
func mergeRefreshedUserInfo(current, refreshed *UserInfo) *UserInfo {
	if current == nil || current.OrgId != refreshed.OrgId || !strings.EqualFold(current.UserName, refreshed.UserName) {
		return refreshed
	}
	merged := *refreshed
	merged.UserId = firstNonEmpty(refreshed.UserId, current.UserId)
	merged.ProfileId = firstNonEmpty(refreshed.ProfileId, current.ProfileId)
	merged.OrgNamespace = firstNonEmpty(refreshed.OrgNamespace, current.OrgNamespace)
	return &merged
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
