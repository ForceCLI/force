package main

import (
	"testing"
)

func TestParseApiVersion(t *testing.T) {
	// Bare version number
	parseApiVersion([]string{"1.0"})
	if apiVersionNumber != "1.0" {
		t.Errorf("apiVersionNumber = %v, want 1.0", apiVersionNumber)
	}
	if apiVersion != "v1.0" {
		t.Errorf("apiVersion = %v, want v1.0", apiVersion)
	}

	// With v-prefix
	parseApiVersion([]string{"v2.0"})
	if apiVersionNumber != "2.0" {
		t.Errorf("apiVersionNumber = %v, want 2.0", apiVersionNumber)
	}
	if apiVersion != "v2.0" {
		t.Errorf("apiVersion = %v, want v2.0", apiVersion)
	}
}
