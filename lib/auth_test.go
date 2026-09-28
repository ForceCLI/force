package lib

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	forceConfig "github.com/ForceCLI/force/config"
)

type recordingLogger struct {
	calls int
}

func (r *recordingLogger) Info(args ...interface{}) {
	r.calls++
}

func stubUserInfo(userName string) func(*ForceSession) (UserInfo, error) {
	return func(_ *ForceSession) (UserInfo, error) {
		return UserInfo{
			UserName: userName,
			OrgId:    "00D123456789012",
			UserId:   "005123456789012",
		}, nil
	}
}

func setupTestConfig(t *testing.T) func() {
	original := forceConfig.Config
	dir := t.TempDir()
	if err := forceConfig.UseConfigDirectory(dir); err != nil {
		t.Fatalf("failed to set config directory: %v", err)
	}
	return func() {
		forceConfig.Config = original
		os.RemoveAll(dir)
	}
}

func TestForceSaveLoginLogsOnce(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	originalLogger := Log
	recorder := &recordingLogger{}
	Log = recorder
	defer func() { Log = originalLogger }()

	originalGetUserInfo := getUserInfoFn
	getUserInfoFn = stubUserInfo("tester@example.com")
	defer func() { getUserInfoFn = originalGetUserInfo }()

	session := ForceSession{
		AccessToken: "token",
		InstanceUrl: "https://example.com",
	}

	if _, err := ForceSaveLogin(session, os.Stderr); err != nil {
		t.Fatalf("ForceSaveLogin returned error: %v", err)
	}

	if recorder.calls != 1 {
		t.Fatalf("expected exactly one log entry, got %d", recorder.calls)
	}
}

func TestUpdateCredentialsDoesNotLog(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	originalLogger := Log
	recorder := &recordingLogger{}
	Log = recorder
	defer func() { Log = originalLogger }()

	originalGetUserInfo := getUserInfoFn
	getUserInfoFn = stubUserInfo("tester@example.com")
	defer func() { getUserInfoFn = originalGetUserInfo }()

	force := &Force{
		Credentials: &ForceSession{
			AccessToken:    "old-token",
			InstanceUrl:    "https://example.com",
			SessionOptions: &SessionOptions{},
			UserInfo: &UserInfo{
				UserName: "tester@example.com",
				OrgId:    "00D123456789012",
				UserId:   "005123456789012",
			},
		},
	}

	newCreds := ForceSession{
		AccessToken: "new-token",
		InstanceUrl: "https://example.com",
	}

	force.UpdateCredentials(newCreds)

	if recorder.calls != 0 {
		t.Fatalf("expected no log entries during UpdateCredentials, got %d", recorder.calls)
	}
}

func namespaceQueryServer(t *testing.T, namespace interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/services/data/" + ApiVersion() + "/query"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}
		if q := r.URL.Query().Get("q"); q != "SELECT NamespacePrefix FROM Organization" {
			t.Errorf("unexpected query: %s", q)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"totalSize": 1,
			"done":      true,
			"records": []map[string]interface{}{
				{"NamespacePrefix": namespace},
			},
		})
	}))
}

func TestGetOrgNamespaceReturnsOrganizationNamespacePrefix(t *testing.T) {
	server := namespaceQueryServer(t, "myns")
	defer server.Close()

	force := NewForce(&ForceSession{InstanceUrl: server.URL, AccessToken: "token"})
	namespace, err := force.getOrgNamespace()
	if err != nil {
		t.Fatalf("getOrgNamespace returned error: %v", err)
	}
	if namespace != "myns" {
		t.Fatalf("expected namespace myns, got %q", namespace)
	}
}

func TestGetOrgNamespaceReturnsEmptyStringWithoutNamespace(t *testing.T) {
	server := namespaceQueryServer(t, nil)
	defer server.Close()

	force := NewForce(&ForceSession{InstanceUrl: server.URL, AccessToken: "token"})
	namespace, err := force.getOrgNamespace()
	if err != nil {
		t.Fatalf("getOrgNamespace returned error: %v", err)
	}
	if namespace != "" {
		t.Fatalf("expected empty namespace, got %q", namespace)
	}
}
