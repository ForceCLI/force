package command_test

import (
	. "github.com/ForceCLI/force/lib"
	"github.com/bmizerany/assert"
	"github.com/devangel/config"
	"os"
	"os/exec"
	"testing"
)

var TestConfig = config.NewConfig("force")

// test that SetActiveLogin in turn sets the "current account"
func TestSetActiveLogin(t *testing.T) {
	prevAcct, err := TestConfig.Load("current", "account")
	if err != nil {
		prevAcct = ""
	}
	SetActiveLogin("clint")
	account, _ := TestConfig.Load("current", "account")
	SetActiveLogin(prevAcct)
	assert.Equal(t, account, "clint")
}

/* test that ActiveCredentials exits with a status code of 1 if there is no matching
 * credential file for the active login.
 */
func TestActiveCredentialsMissingFile(t *testing.T) {
	if os.Getenv("TEST_MISSING_CREDS") == "1" {
		SetActiveLogin("no_matching_credentials_file")
		ActiveCredentials(true)
		return
	}
	prevAcct, err := TestConfig.Load("current", "account")
	if err != nil {
		prevAcct = ""
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestActiveCredentialsMissingFile")
	cmd.Env = append(os.Environ(), "TEST_MISSING_CREDS=1")
	err = cmd.Run()
	SetActiveLogin(prevAcct)
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, expect exit status 1", err)
}
