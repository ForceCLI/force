package main

import (
	"github.com/bmizerany/assert"
	"github.com/ddollar/config"
	"testing"
)

var TestConfig = config.NewConfig("force")

// test that SetActiveLogin in turn sets the "current account"
func TestSetActiveLogin(t *testing.T) {
	SetActiveLogin("clint")
	account, _ := TestConfig.Load("current", "account")
	assert.Equal(t, account, "clint")
}
