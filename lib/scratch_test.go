package lib

import (
	"strings"
	"testing"
)

func TestBuildSettingsMetadata_AddsOrgPreferenceSettings(t *testing.T) {
	files := buildSettingsMetadata([]string{"enableApexApprovalLockUnlock"})

	content, ok := files["unpackaged/settings/Apex.settings"]
	if !ok {
		t.Fatalf("Apex.settings not generated")
	}
	if !strings.Contains(string(content), "<enableApexApprovalLockUnlock>true</enableApexApprovalLockUnlock>") {
		t.Errorf("Apex.settings missing enableApexApprovalLockUnlock preference:\n%s", content)
	}
}

func TestBuildSettingsMetadata_ExcludesApexSettingsWhenUnused(t *testing.T) {
	files := buildSettingsMetadata([]string{"enableEnhancedNotes"})

	if _, ok := files["unpackaged/settings/Apex.settings"]; ok {
		t.Fatalf("Apex.settings should not be generated when no Apex settings requested")
	}
}
