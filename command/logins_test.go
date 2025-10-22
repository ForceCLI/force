package command_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	forceConfig "github.com/ForceCLI/force/config"
	. "github.com/ForceCLI/force/lib"
	"github.com/bmizerany/assert"
)

// test that SetActiveLogin in turn sets the "current account"
func TestSetActiveLogin(t *testing.T) {
	origConfig := forceConfig.Config
	tempDir := t.TempDir()
	if err := forceConfig.UseConfigDirectory(tempDir); err != nil {
		t.Fatalf("failed to set config directory: %v", err)
	}
	t.Cleanup(func() {
		forceConfig.Config = origConfig
		os.RemoveAll(tempDir)
	})

	prevAcct, err := forceConfig.Config.Load("current", "account")
	if err != nil {
		prevAcct = ""
	}
	SetActiveLogin("clint")
	account, _ := forceConfig.Config.Load("current", "account")
	SetActiveLogin(prevAcct)
	assert.Equal(t, account, "clint")
}

/* test that ActiveCredentials exits with a status code of 1 if there is no matching
 * credential file for the active login.
 */
func TestActiveCredentialsMissingFile(t *testing.T) {
	if os.Getenv("TEST_MISSING_CREDS") == "1" {
		dir := os.Getenv("TEST_FORCE_CONFIG_DIR")
		if dir == "" {
			t.Fatal("missing TEST_FORCE_CONFIG_DIR")
		}
		if err := forceConfig.UseConfigDirectory(dir); err != nil {
			t.Fatalf("failed to set config directory: %v", err)
		}
		SetActiveLogin("no_matching_credentials_file")
		ActiveCredentials(true)
		return
	}
	origConfig := forceConfig.Config
	tempRoot := t.TempDir()
	t.Cleanup(func() {
		forceConfig.Config = origConfig
		os.RemoveAll(tempRoot)
	})
	dir := filepath.Join(tempRoot, "config")
	if err := forceConfig.UseConfigDirectory(dir); err != nil {
		t.Fatalf("failed to set config directory: %v", err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestActiveCredentialsMissingFile")
	cmd.Env = append(os.Environ(),
		"TEST_MISSING_CREDS=1",
		"TEST_FORCE_CONFIG_DIR="+dir,
	)
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, expect exit status 1", err)
}
