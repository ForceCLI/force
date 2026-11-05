package lib

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	lazyTestOrgId  = "00D123456789012AAA"
	lazyTestUser   = "scratch@example.com"
	lazyTestUserId = "005123456789012AAA"
)

// lazySFDXSession mimics what SFDXAuthToForceSession builds from an auth
// file that has no userId.
func lazySFDXSession() *ForceSession {
	return &ForceSession{
		AccessToken: "token",
		InstanceUrl: "https://example.my.salesforce.com",
		Id:          lazyTestOrgId,
		UserInfo:    &UserInfo{UserName: lazyTestUser, OrgId: lazyTestOrgId},
	}
}

// countUserInfoFetches stubs getUserInfoFn and counts how often it is called.
func countUserInfoFetches(t *testing.T, info UserInfo, err error) *int {
	t.Helper()
	calls := 0
	original := getUserInfoFn
	getUserInfoFn = func(_ *ForceSession) (UserInfo, error) {
		calls++
		return info, err
	}
	t.Cleanup(func() { getUserInfoFn = original })
	return &calls
}

func fullUserInfo() UserInfo {
	return UserInfo{
		UserName:     lazyTestUser,
		OrgId:        lazyTestOrgId,
		UserId:       lazyTestUserId,
		ProfileId:    "00e123456789012AAA",
		OrgNamespace: "ns",
	}
}

func TestCompleteSFDXSessionFetchesMissingUserInfo(t *testing.T) {
	calls := countUserInfoFetches(t, fullUserInfo(), nil)
	creds := lazySFDXSession()

	if err := CompleteSFDXSession(creds, "scratch"); err != nil {
		t.Fatalf("CompleteSFDXSession: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("expected one user info fetch, got %d", *calls)
	}
	if creds.UserInfo.UserId != lazyTestUserId || creds.UserInfo.OrgNamespace != "ns" {
		t.Fatalf("user info not populated: %+v", creds.UserInfo)
	}
}

func TestCompleteSFDXSessionSkipsFetchWhenIdsAreKnown(t *testing.T) {
	calls := countUserInfoFetches(t, fullUserInfo(), nil)
	creds := lazySFDXSession()
	creds.UserInfo.UserId = lazyTestUserId

	if err := CompleteSFDXSession(creds, "scratch"); err != nil {
		t.Fatalf("CompleteSFDXSession: %v", err)
	}
	if *calls != 0 {
		t.Fatalf("expected no fetch, got %d", *calls)
	}
}

func TestCompleteSFDXSessionLazyDoesNotFetchUserInfo(t *testing.T) {
	calls := countUserInfoFetches(t, fullUserInfo(), nil)
	creds := lazySFDXSession()

	if err := CompleteSFDXSessionLazy(creds, "scratch"); err != nil {
		t.Fatalf("CompleteSFDXSessionLazy: %v", err)
	}
	if *calls != 0 {
		t.Fatalf("expected no user info fetch, got %d", *calls)
	}
	if creds.UserInfo.UserName != lazyTestUser || creds.UserInfo.OrgId != lazyTestOrgId {
		t.Fatalf("known user info lost: %+v", creds.UserInfo)
	}
	if creds.SessionOptions.Alias != "scratch" {
		t.Fatalf("alias = %q, want scratch", creds.SessionOptions.Alias)
	}
}

func TestCompleteSFDXSessionLazyFetchesWhenUserIsUnknown(t *testing.T) {
	calls := countUserInfoFetches(t, fullUserInfo(), nil)
	creds := &ForceSession{AccessToken: "token", InstanceUrl: "https://example.my.salesforce.com"}

	if err := CompleteSFDXSessionLazy(creds, ""); err != nil {
		t.Fatalf("CompleteSFDXSessionLazy: %v", err)
	}
	if *calls != 1 || creds.UserInfo == nil || creds.UserInfo.UserName != lazyTestUser {
		t.Fatalf("expected one fetch filling the user name, got %d fetches and %+v", *calls, creds.UserInfo)
	}
	if creds.Id != lazyTestOrgId {
		t.Fatalf("org id = %q, want %s", creds.Id, lazyTestOrgId)
	}
}

func TestEnsureSessionUserInfoFetchesOnce(t *testing.T) {
	calls := countUserInfoFetches(t, fullUserInfo(), nil)
	creds := lazySFDXSession()

	for i := 0; i < 2; i++ {
		if err := EnsureSessionUserInfo(creds); err != nil {
			t.Fatalf("EnsureSessionUserInfo: %v", err)
		}
	}
	if *calls != 1 {
		t.Fatalf("expected 1 fetch, got %d", *calls)
	}
	if creds.UserInfo.UserId != lazyTestUserId || creds.UserInfo.OrgNamespace != "ns" {
		t.Fatalf("user info not populated: %+v", creds.UserInfo)
	}
}

func TestEnsureSessionUserInfoKeepsKnownFieldsWhenResponseLacksThem(t *testing.T) {
	info := fullUserInfo()
	info.UserName, info.OrgId = "", ""
	countUserInfoFetches(t, info, nil)
	creds := lazySFDXSession()

	if err := EnsureSessionUserInfo(creds); err != nil {
		t.Fatalf("EnsureSessionUserInfo: %v", err)
	}
	if creds.UserInfo.UserName != lazyTestUser || creds.UserInfo.OrgId != lazyTestOrgId {
		t.Fatalf("known fields overwritten: %+v", creds.UserInfo)
	}
}

func TestEnsureSessionUserInfoSkipsCompleteSessions(t *testing.T) {
	calls := countUserInfoFetches(t, fullUserInfo(), nil)
	creds := lazySFDXSession()
	creds.UserInfo.UserId = lazyTestUserId

	if err := EnsureSessionUserInfo(creds); err != nil {
		t.Fatalf("EnsureSessionUserInfo: %v", err)
	}
	if *calls != 0 {
		t.Fatalf("expected no fetch for a session with a user id, got %d", *calls)
	}
}

func TestEnsureSessionUserInfoReturnsFetchErrors(t *testing.T) {
	countUserInfoFetches(t, UserInfo{}, errors.New("boom"))
	creds := lazySFDXSession()

	if err := EnsureSessionUserInfo(creds); err == nil {
		t.Fatal("expected an error")
	}
	if creds.UserInfo.UserName != lazyTestUser {
		t.Fatalf("session modified on error: %+v", creds.UserInfo)
	}
}

func TestSessionLoadersFetchUserInfoUnlessLazy(t *testing.T) {
	tests := []struct {
		name       string
		load       func() (*Force, error)
		wantCalls  int
		wantUserId string
	}{
		{"GetForce", func() (*Force, error) { return GetForce(lazyTestUser) }, 1, lazyTestUserId},
		{"ActiveForce", ActiveForce, 1, lazyTestUserId},
		{"GetForceLazy", func() (*Force, error) { return GetForceLazy(lazyTestUser) }, 0, ""},
		{"ActiveForceLazy", ActiveForceLazy, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer setupTestConfig(t)()
			dir := t.TempDir()
			authFile := fmt.Sprintf(`{
	"accessToken": "token",
	"instanceUrl": "https://example.my.salesforce.com",
	"orgId": %q,
	"username": %q
}`, lazyTestOrgId, lazyTestUser)
			if err := os.WriteFile(filepath.Join(dir, lazyTestUser+".json"), []byte(authFile), 0600); err != nil {
				t.Fatalf("failed to write auth file: %v", err)
			}
			t.Setenv("FORCE_SFDX_STATE_DIRS", dir)
			if err := SetActiveLogin(lazyTestUser); err != nil {
				t.Fatalf("SetActiveLogin: %v", err)
			}
			calls := countUserInfoFetches(t, fullUserInfo(), nil)

			f, err := tt.load()
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if *calls != tt.wantCalls {
				t.Fatalf("expected %d fetches, got %d", tt.wantCalls, *calls)
			}
			if f.Credentials.UserInfo.UserId != tt.wantUserId {
				t.Fatalf("user id = %q, want %q", f.Credentials.UserInfo.UserId, tt.wantUserId)
			}
		})
	}
}

func TestEnsureSessionUserInfoKeepsAuthFileNamespaceWhenResponseLacksIt(t *testing.T) {
	info := fullUserInfo()
	info.OrgNamespace = ""
	countUserInfoFetches(t, info, nil)
	creds := lazySFDXSession()
	creds.UserInfo.OrgNamespace = "authns"

	if err := EnsureSessionUserInfo(creds); err != nil {
		t.Fatalf("EnsureSessionUserInfo: %v", err)
	}
	if creds.UserInfo.OrgNamespace != "authns" {
		t.Fatalf("namespace = %q, want authns", creds.UserInfo.OrgNamespace)
	}
}

func TestUpdateFromSFDXSessionMergesUserInfo(t *testing.T) {
	loaded := fullUserInfo()
	tests := []struct {
		name      string
		current   *UserInfo
		refreshed UserInfo
		want      UserInfo
	}{
		{
			name:      "keeps fields the auth file lacks",
			current:   &loaded,
			refreshed: UserInfo{UserName: lazyTestUser, OrgId: lazyTestOrgId},
			want:      loaded,
		},
		{
			name:      "matches the user name case-insensitively",
			current:   &loaded,
			refreshed: UserInfo{UserName: strings.ToUpper(lazyTestUser), OrgId: lazyTestOrgId},
			want: UserInfo{
				UserName:     strings.ToUpper(lazyTestUser),
				OrgId:        lazyTestOrgId,
				UserId:       lazyTestUserId,
				ProfileId:    loaded.ProfileId,
				OrgNamespace: loaded.OrgNamespace,
			},
		},
		{
			name:      "prefers auth file values",
			current:   &loaded,
			refreshed: UserInfo{UserName: lazyTestUser, OrgId: lazyTestOrgId, OrgNamespace: "newns"},
			want: UserInfo{
				UserName:     lazyTestUser,
				OrgId:        lazyTestOrgId,
				UserId:       lazyTestUserId,
				ProfileId:    loaded.ProfileId,
				OrgNamespace: "newns",
			},
		},
		{
			name:      "drops fields when the alias now points at another user",
			current:   &loaded,
			refreshed: UserInfo{UserName: "other@example.com", OrgId: lazyTestOrgId},
			want:      UserInfo{UserName: "other@example.com", OrgId: lazyTestOrgId},
		},
		{
			name:      "drops fields when the alias now points at another org",
			current:   &loaded,
			refreshed: UserInfo{UserName: lazyTestUser, OrgId: "00D999999999999AAA"},
			want:      UserInfo{UserName: lazyTestUser, OrgId: "00D999999999999AAA"},
		},
		{
			name:      "uses the refreshed info when nothing is loaded",
			current:   nil,
			refreshed: UserInfo{UserName: lazyTestUser, OrgId: lazyTestOrgId},
			want:      UserInfo{UserName: lazyTestUser, OrgId: lazyTestOrgId},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := lazySFDXSession()
			creds.UserInfo = tt.current
			f := NewForce(creds)
			refreshed := tt.refreshed

			updateFromSFDXSession(f, ForceSession{AccessToken: "new", UserInfo: &refreshed})

			if *f.Credentials.UserInfo != tt.want {
				t.Fatalf("user info = %+v, want %+v", *f.Credentials.UserInfo, tt.want)
			}
		})
	}
}

func TestQueryProfileLoadsMissingProfileId(t *testing.T) {
	const profileId = "00e123456789012AAA"
	tests := []struct {
		name      string
		profileId string
		wantCalls int
	}{
		{"fetches when the session has a user id but no profile id", "", 1},
		{"skips the fetch when the profile id is known", profileId, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var query string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query = r.URL.Query().Get("q")
				fmt.Fprint(w, `{"totalSize":1,"done":true,"records":[{"Id":"00e123456789012AAA"}]}`)
			}))
			defer server.Close()
			calls := countUserInfoFetches(t, fullUserInfo(), nil)
			creds := lazySFDXSession()
			creds.InstanceUrl = server.URL
			creds.UserInfo.UserId = lazyTestUserId
			creds.UserInfo.ProfileId = tt.profileId

			if _, err := NewForce(creds).QueryProfile("Id"); err != nil {
				t.Fatalf("QueryProfile: %v", err)
			}
			if *calls != tt.wantCalls {
				t.Fatalf("expected %d fetches, got %d", tt.wantCalls, *calls)
			}
			if !strings.Contains(query, "Id='"+profileId+"'") {
				t.Fatalf("query %q does not filter on the profile id", query)
			}
		})
	}
}

func TestQueryProfileReturnsFetchErrors(t *testing.T) {
	countUserInfoFetches(t, UserInfo{}, errors.New("boom"))

	if _, err := NewForce(lazySFDXSession()).QueryProfile("Id"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestForceEnsureUserInfoLoadsUserInfo(t *testing.T) {
	calls := countUserInfoFetches(t, fullUserInfo(), nil)
	f := NewForce(lazySFDXSession())

	if err := f.EnsureUserInfo(); err != nil {
		t.Fatalf("EnsureUserInfo: %v", err)
	}
	if *calls != 1 || f.Credentials.UserInfo.ProfileId == "" {
		t.Fatalf("expected the Force helper to load user info, got %d fetches and %+v", *calls, f.Credentials.UserInfo)
	}
}
