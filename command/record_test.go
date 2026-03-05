package command

import (
	"testing"
)

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
