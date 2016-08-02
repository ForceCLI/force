package main

import (
	"testing"

	"github.com/bmizerany/assert"
)

// test that all avialable commands come with at least a name and short usage information
func TestUsage(t *testing.T) {
	for _, cmd := range commands {
		assert.NotEqual(t, cmd.Name(), "")
		assert.NotEqual(t, cmd.Short, "")
	}
}
