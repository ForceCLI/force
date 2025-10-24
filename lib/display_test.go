package lib

import (
	"os"
	"os/exec"
	"testing"
)

// TestDisplayForceSobject_ValidFields tests that DisplayForceSobject works with valid field data
func TestDisplayForceSobject_ValidFields(t *testing.T) {
	sobject := ForceSobject{
		"fields": []interface{}{
			map[string]interface{}{
				"name": "Id",
				"type": "id",
			},
			map[string]interface{}{
				"name": "Status",
				"type": "picklist",
				"picklistValues": []interface{}{
					map[string]interface{}{"value": "Open"},
					map[string]interface{}{"value": "Closed"},
				},
			},
		},
	}

	// This should not panic
	DisplayForceSobject(sobject)
}

// TestDisplayForceSobject_NilFields tests that DisplayForceSobject exits gracefully with nil fields
func TestDisplayForceSobject_NilFields(t *testing.T) {
	if os.Getenv("CRASH_TEST") == "1" {
		sobject := ForceSobject{
			"fields": nil,
		}
		DisplayForceSobject(sobject)
		return
	}

	// Run the test in a subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestDisplayForceSobject_NilFields")
	cmd.Env = append(os.Environ(), "CRASH_TEST=1")
	err := cmd.Run()

	// We expect the subprocess to exit with status 1 (from ErrorAndExit)
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return // Expected: process exited with non-zero status
	}

	t.Fatalf("expected process to exit with error, got %v", err)
}

// TestDisplayForceSobject_EmptyFields tests that DisplayForceSobject works with empty field array
func TestDisplayForceSobject_EmptyFields(t *testing.T) {
	sobject := ForceSobject{
		"fields": []interface{}{},
	}

	// This should not panic
	DisplayForceSobject(sobject)
}
