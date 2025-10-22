package lib

import (
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
