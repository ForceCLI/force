package command_test

import (
	"github.com/bmizerany/assert"
	. "github.com/heroku/force/command"
	"testing"
)

// test that all avialable commands come with at least a name and short usage information
func TestUsage(t *testing.T) {
	for _, cmd := range Commands {
		assert.NotEqual(t, cmd.Name(), "")
		assert.NotEqual(t, cmd.Short, "")
	}
}
