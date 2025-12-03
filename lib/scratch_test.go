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

func TestBuildSettingsMetadata_AddsUserManagementSettings(t *testing.T) {
	files := buildSettingsMetadata([]string{"permsetsInFieldCreation"})

	content, ok := files["unpackaged/settings/UserManagement.settings"]
	if !ok {
		t.Fatalf("UserManagement.settings not generated")
	}
	if !strings.Contains(string(content), "<permsetsInFieldCreation>true</permsetsInFieldCreation>") {
		t.Errorf("UserManagement.settings missing permsetsInFieldCreation preference:\n%s", content)
	}
}

func TestBuildSettingsMetadata_ExcludesUserManagementSettingsWhenUnused(t *testing.T) {
	files := buildSettingsMetadata([]string{"enableEnhancedNotes"})

	if _, ok := files["unpackaged/settings/UserManagement.settings"]; ok {
		t.Fatalf("UserManagement.settings should not be generated when not requested")
	}
}
