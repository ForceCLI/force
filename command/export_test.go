package command

import (
	"testing"
)

func TestIsExcluded(t *testing.T) {
	excluded := []string{"ApexClass", "CustomThing"}
	excludeMetadataNames = append(excludeMetadataNames, excluded...)

	testCases := []struct {
		input    string
		expected bool
	}{
		{"custom", false},
		{"CustomThing", true},
	}

	for _, test := range testCases {
		t.Run(test.input, func(t *testing.T) {
			got := isExcluded(test.input)

			if got != test.expected {
				t.Errorf("Expected %v got %v for %s entry", test.expected, got, test.input)
			}
		})
	}

}
