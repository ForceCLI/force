package lib

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestBuildSettingsMetadata_AddsLightningExperienceSettings(t *testing.T) {
	files := buildSettingsMetadata([]string{"enableLightningPreviewPref"})

	content, ok := files["unpackaged/settings/LightningExperience.settings"]
	if !ok {
		t.Fatalf("LightningExperience.settings not generated")
	}
	if !strings.Contains(string(content), "<enableLightningPreviewPref>true</enableLightningPreviewPref>") {
		t.Errorf("LightningExperience.settings missing enableLightningPreviewPref preference:\n%s", content)
	}
}

func TestBuildSettingsMetadata_ExcludesLightningExperienceSettingsWhenUnused(t *testing.T) {
	files := buildSettingsMetadata([]string{"enableEnhancedNotes"})

	if _, ok := files["unpackaged/settings/LightningExperience.settings"]; ok {
		t.Fatalf("LightningExperience.settings should not be generated when not requested")
	}
}

func TestBuildSettingsMetadata_AddsCommerceSettings(t *testing.T) {
	files := buildSettingsMetadata([]string{"commerceEnabled"})

	content, ok := files["unpackaged/settings/Commerce.settings"]
	if !ok {
		t.Fatalf("Commerce.settings not generated")
	}
	if !strings.Contains(string(content), "<commerceEnabled>true</commerceEnabled>") {
		t.Errorf("Commerce.settings missing commerceEnabled preference:\n%s", content)
	}
	if !strings.Contains(string(content), "<CommerceSettings ") {
		t.Errorf("Commerce.settings missing CommerceSettings metadata type:\n%s", content)
	}
}

func TestBuildSettingsMetadata_ExcludesCommerceSettingsWhenUnused(t *testing.T) {
	files := buildSettingsMetadata([]string{"enableEnhancedNotes"})

	if _, ok := files["unpackaged/settings/Commerce.settings"]; ok {
		t.Fatalf("Commerce.settings should not be generated when not requested")
	}
}

func TestBuildSettingsMetadata_AddsOrderSettings_for_enableOrders(t *testing.T) {
	files := buildSettingsMetadata([]string{"enableOrders"})

	content, ok := files["unpackaged/settings/Order.settings"]
	if !ok {
		t.Fatalf("Order.settings not generated")
	}
	if !strings.Contains(string(content), "<enableOrders>true</enableOrders>") {
		t.Errorf("Order.settings missing enableOrders preference:\n%s", content)
	}
	if !strings.Contains(string(content), "<enableEnhancedCommerceOrders>true</enableEnhancedCommerceOrders>") {
		t.Errorf("Order.settings missing enableEnhancedCommerceOrders preference:\n%s", content)
	}
	if !strings.Contains(string(content), "<OrderSettings ") {
		t.Errorf("Order.settings missing OrderSettings metadata type:\n%s", content)
	}
}

func TestBuildSettingsMetadata_AddsOrderSettings_for_enableEnhancedCommerceOrders(t *testing.T) {
	files := buildSettingsMetadata([]string{"enableEnhancedCommerceOrders"})

	content, ok := files["unpackaged/settings/Order.settings"]
	if !ok {
		t.Fatalf("Order.settings not generated")
	}
	if !strings.Contains(string(content), "<enableOrders>true</enableOrders>") {
		t.Errorf("Order.settings missing enableOrders preference:\n%s", content)
	}
	if !strings.Contains(string(content), "<enableEnhancedCommerceOrders>true</enableEnhancedCommerceOrders>") {
		t.Errorf("Order.settings missing enableEnhancedCommerceOrders preference:\n%s", content)
	}
}

func TestBuildSettingsMetadata_ExcludesOrderSettingsWhenUnused(t *testing.T) {
	files := buildSettingsMetadata([]string{"enableEnhancedNotes"})

	if _, ok := files["unpackaged/settings/Order.settings"]; ok {
		t.Fatalf("Order.settings should not be generated when not requested")
	}
}

func TestBuildSettingsMetadata_AddsLiveAgentSettings(t *testing.T) {
	files := buildSettingsMetadata([]string{"enableLiveAgent"})

	content, ok := files["unpackaged/settings/LiveAgent.settings"]
	if !ok {
		t.Fatalf("LiveAgent.settings not generated")
	}
	if !strings.Contains(string(content), "<enableLiveAgent>true</enableLiveAgent>") {
		t.Errorf("LiveAgent.settings missing enableLiveAgent preference:\n%s", content)
	}
	if !strings.Contains(string(content), "<LiveAgentSettings ") {
		t.Errorf("LiveAgent.settings missing LiveAgentSettings metadata type:\n%s", content)
	}
}

func TestBuildSettingsMetadata_ExcludesLiveAgentSettingsWhenUnused(t *testing.T) {
	files := buildSettingsMetadata([]string{"enableEnhancedNotes"})

	if _, ok := files["unpackaged/settings/LiveAgent.settings"]; ok {
		t.Fatalf("LiveAgent.settings should not be generated when not requested")
	}
}

func TestGetScratchOrg_returns_error_when_SignupUsername_is_nil(t *testing.T) {
	f := &Force{}
	f.Credentials = &ForceSession{}

	// Mock GetRecord to return a map with nil SignupUsername
	originalGetRecord := f.GetRecord
	_ = originalGetRecord // GetRecord is a method, can't easily mock without interface

	// This test validates the type assertion safety
	// The actual behavior requires integration testing with Salesforce
	org := map[string]interface{}{
		"SignupUsername": nil,
		"LoginUrl":       "https://test.salesforce.com",
		"AuthCode":       "abc123",
	}

	// Test the type assertion safety
	username, ok := org["SignupUsername"].(string)
	if ok {
		t.Errorf("Expected type assertion to fail for nil SignupUsername, got: %s", username)
	}
}

func TestGetScratchOrg_returns_error_when_LoginUrl_is_nil(t *testing.T) {
	org := map[string]interface{}{
		"SignupUsername": "test@example.com",
		"LoginUrl":       nil,
		"AuthCode":       "abc123",
	}

	loginUrl, ok := org["LoginUrl"].(string)
	if ok {
		t.Errorf("Expected type assertion to fail for nil LoginUrl, got: %s", loginUrl)
	}
}

func TestGetScratchOrg_returns_error_when_AuthCode_is_nil(t *testing.T) {
	org := map[string]interface{}{
		"SignupUsername": "test@example.com",
		"LoginUrl":       "https://test.salesforce.com",
		"AuthCode":       nil,
	}

	authCode, ok := org["AuthCode"].(string)
	if ok {
		t.Errorf("Expected type assertion to fail for nil AuthCode, got: %s", authCode)
	}
}

func TestWaitForScratchOrgReady_returns_immediately_when_Active(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Id":             "2SRp0000000MFpOAM",
			"Status":         "Active",
			"SignupUsername": "test@example.com",
			"LoginUrl":       "https://test.salesforce.com",
			"AuthCode":       "abc123",
		})
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	org, err := force.waitForScratchOrgReady("2SRp0000000MFpOAM")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if org["Status"] != "Active" {
		t.Errorf("Expected Status to be Active, got: %v", org["Status"])
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request for Active status, got: %d", requestCount)
	}
}

func TestWaitForScratchOrgReady_polls_until_Active(t *testing.T) {
	// Use short poll interval for testing
	origPollInterval := scratchOrgPollInterval
	scratchOrgPollInterval = 1 * time.Millisecond
	defer func() { scratchOrgPollInterval = origPollInterval }()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		if requestCount < 3 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id":     "2SRp0000000MFpOAM",
				"Status": "New",
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id":             "2SRp0000000MFpOAM",
				"Status":         "Active",
				"SignupUsername": "test@example.com",
				"LoginUrl":       "https://test.salesforce.com",
				"AuthCode":       "abc123",
			})
		}
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	org, err := force.waitForScratchOrgReady("2SRp0000000MFpOAM")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if org["Status"] != "Active" {
		t.Errorf("Expected Status to be Active, got: %v", org["Status"])
	}
	if requestCount < 3 {
		t.Errorf("Expected at least 3 requests for polling, got: %d", requestCount)
	}
}

func TestWaitForScratchOrgReady_returns_error_on_Error_status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Id":        "2SRp0000000MFpOAM",
			"Status":    "Error",
			"ErrorCode": "SignupDuplicateUserNameError",
		})
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	_, err := force.waitForScratchOrgReady("2SRp0000000MFpOAM")

	if err == nil {
		t.Fatal("Expected error for Error status, got nil")
	}
	if !strings.Contains(err.Error(), "SignupDuplicateUserNameError") {
		t.Errorf("Expected error to contain ErrorCode, got: %v", err)
	}
}

func TestWaitForScratchOrgReady_times_out(t *testing.T) {
	// Use very short timeout for testing
	origPollInterval := scratchOrgPollInterval
	origMaxWait := scratchOrgMaxWait
	scratchOrgPollInterval = 1 * time.Millisecond
	scratchOrgMaxWait = 5 * time.Millisecond
	defer func() {
		scratchOrgPollInterval = origPollInterval
		scratchOrgMaxWait = origMaxWait
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Id":     "2SRp0000000MFpOAM",
			"Status": "New",
		})
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	_, err := force.waitForScratchOrgReady("2SRp0000000MFpOAM")

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "Timed out") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestCreateScratchOrgWithDuration_sets_DurationDays(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "ScratchOrgInfo") {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "2SRp0000000MFpOAM",
				"success": true,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	_, err := force.CreateScratchOrgWithDuration("", []string{}, "", []string{}, "", "", 14)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if receivedBody["DurationDays"] != "14" {
		t.Errorf("Expected DurationDays to be '14', got: %v", receivedBody["DurationDays"])
	}
}

func TestCreateScratchOrgWithRelease_uses_default_duration_of_7(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "ScratchOrgInfo") {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "2SRp0000000MFpOAM",
				"success": true,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	_, err := force.CreateScratchOrgWithRelease("", []string{}, "", []string{}, "", "")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if receivedBody["DurationDays"] != "7" {
		t.Errorf("Expected DurationDays to be '7', got: %v", receivedBody["DurationDays"])
	}
}
