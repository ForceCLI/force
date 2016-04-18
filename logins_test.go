package main

import (
	"testing"

	"github.com/bmizerany/assert"
	"github.com/devangel/config"
)

var TestConfig = config.NewConfig("force")

// test that SetActiveLogin in turn sets the "current account"
func TestSetActiveLogin(t *testing.T) {
	// TODO: this is leaking state into actual homedir of person running test suite,
	// setting their active login to `clint`. derp.
	SetActiveLogin("clint")
	account, _ := TestConfig.Load("current", "account")
	assert.Equal(t, account, "clint")
}
