package command

import (
	"net/http"
	"testing"

	"github.com/spf13/cobra"
)

func TestAutoAssignOptions(t *testing.T) {
	tests := []struct {
		name        string
		assign      bool
		noAssign    bool
		expectSet   bool
		expectValue string
	}{
		{
			name:      "no_flags_adds_no_header",
			expectSet: false,
		},
		{
			name:        "assign_sets_header_true",
			assign:      true,
			expectSet:   true,
			expectValue: "true",
		},
		{
			name:        "no_assign_sets_header_false",
			noAssign:    true,
			expectSet:   true,
			expectValue: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().Bool("assign", tt.assign, "")
			cmd.Flags().Bool("no-assign", tt.noAssign, "")

			options := autoAssignOptions(cmd)

			req, err := http.NewRequest("POST", "http://example.com", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %s", err)
			}
			for _, option := range options {
				option(req)
			}

			got := req.Header.Get("Sforce-Auto-Assign")
			if tt.expectSet {
				if got != tt.expectValue {
					t.Errorf("Expected Sforce-Auto-Assign=%q, got %q", tt.expectValue, got)
				}
			} else if got != "" {
				t.Errorf("Expected no Sforce-Auto-Assign header, got %q", got)
			}
		})
	}
}

func TestParseArgumentAttrs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]string
	}{
		{
			name:     "parses_single_field",
			input:    []string{"Name:Acme"},
			expected: map[string]string{"Name": "Acme"},
		},
		{
			name:     "parses_multiple_fields",
			input:    []string{"Name:Acme", "Industry:Technology"},
			expected: map[string]string{"Name": "Acme", "Industry": "Technology"},
		},
		{
			name:     "handles_empty_input",
			input:    []string{},
			expected: map[string]string{},
		},
		{
			name:     "handles_value_with_colons",
			input:    []string{"Description:Value:with:colons"},
			expected: map[string]string{"Description": "Value:with:colons"},
		},
		{
			name:     "handles_empty_value",
			input:    []string{"Name:"},
			expected: map[string]string{"Name": ""},
		},
		{
			name:     "handles_quoted_value_with_spaces",
			input:    []string{"Name:Acme Corp"},
			expected: map[string]string{"Name": "Acme Corp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseArgumentAttrs(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d fields, got %d", len(tt.expected), len(result))
			}
			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("Expected %s=%q, got %s=%q", key, expectedValue, key, result[key])
				}
			}
		})
	}
}
