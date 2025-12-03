package command

import (
	"strings"
	"testing"
)

func TestExpandProductsToFeatures_NoFeaturesOrProducts(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{}, []ScratchFeature{}, map[string]string{})
	if len(result) != 0 {
		t.Errorf("Expected empty slice, got %v", result)
	}
}

func TestExpandProductsToFeatures_SingleFeature(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{}, []ScratchFeature{PersonAccounts}, map[string]string{})
	if len(result) != 1 {
		t.Errorf("Expected 1 feature, got %d", len(result))
	}
	if result[0] != "PersonAccounts" {
		t.Errorf("Expected PersonAccounts, got %s", result[0])
	}
}

func TestExpandProductsToFeatures_MultipleFeatures(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{}, []ScratchFeature{PersonAccounts, ContactsToMultipleAccounts}, map[string]string{})
	if len(result) != 2 {
		t.Errorf("Expected 2 features, got %d", len(result))
	}
	featureMap := make(map[string]bool)
	for _, f := range result {
		featureMap[f] = true
	}
	if !featureMap["PersonAccounts"] {
		t.Error("Expected PersonAccounts in result")
	}
	if !featureMap["ContactsToMultipleAccounts"] {
		t.Error("Expected ContactsToMultipleAccounts in result")
	}
}

func TestExpandProductsToFeatures_FSCProduct(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{FSC}, []ScratchFeature{}, map[string]string{})
	if len(result) != 3 {
		t.Errorf("Expected 3 features from FSC, got %d", len(result))
	}
	featureMap := make(map[string]bool)
	for _, f := range result {
		featureMap[f] = true
	}
	if !featureMap["PersonAccounts"] {
		t.Error("Expected PersonAccounts in FSC")
	}
	if !featureMap["ContactsToMultipleAccounts"] {
		t.Error("Expected ContactsToMultipleAccounts in FSC")
	}
	if !featureMap["FinancialServicesUser:10"] {
		t.Error("Expected FinancialServicesUser:10 in FSC (with default quantity)")
	}
}

func TestExpandProductsToFeatures_ProductAndFeature_Deduplication(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{FSC}, []ScratchFeature{PersonAccounts}, map[string]string{})
	if len(result) != 3 {
		t.Errorf("Expected 3 unique features (FSC includes PersonAccounts), got %d", len(result))
	}
}

func TestExpandProductsToFeatures_MultipleProductsAndFeatures(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{FSC}, []ScratchFeature{PersonAccounts, ContactsToMultipleAccounts}, map[string]string{})
	if len(result) != 3 {
		t.Errorf("Expected 3 unique features, got %d", len(result))
	}
}

func TestExpandProductsToFeatures_FeatureWithDefaultQuantity(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{}, []ScratchFeature{FinancialServicesUser}, map[string]string{})
	if len(result) != 1 {
		t.Errorf("Expected 1 feature, got %d", len(result))
	}
	if result[0] != "FinancialServicesUser:10" {
		t.Errorf("Expected FinancialServicesUser:10, got %s", result[0])
	}
}

func TestExpandProductsToFeatures_FeatureWithCustomQuantity(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{}, []ScratchFeature{FinancialServicesUser}, map[string]string{"FinancialServicesUser": "5"})
	if len(result) != 1 {
		t.Errorf("Expected 1 feature, got %d", len(result))
	}
	if result[0] != "FinancialServicesUser:5" {
		t.Errorf("Expected FinancialServicesUser:5, got %s", result[0])
	}
}

func TestExpandProductsToFeatures_FSCProductWithCustomQuantity(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{FSC}, []ScratchFeature{}, map[string]string{"FinancialServicesUser": "20"})
	if len(result) != 3 {
		t.Errorf("Expected 3 features from FSC, got %d", len(result))
	}
	var foundFSU bool
	for _, f := range result {
		if strings.HasPrefix(f, "FinancialServicesUser:") {
			foundFSU = true
			if f != "FinancialServicesUser:20" {
				t.Errorf("Expected FinancialServicesUser:20, got %s", f)
			}
		}
	}
	if !foundFSU {
		t.Error("Expected FinancialServicesUser with custom quantity in result")
	}
}

func TestExpandProductsToFeatures_StateAndCountryPicklist(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{}, []ScratchFeature{StateAndCountryPicklist}, map[string]string{})
	if len(result) != 1 {
		t.Errorf("Expected 1 feature, got %d", len(result))
	}
	if result[0] != "StateAndCountryPicklist" {
		t.Errorf("Expected StateAndCountryPicklist, got %s", result[0])
	}
}

func TestExpandProductsToFeatures_MixedFeaturesIncludingStateAndCountry(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{}, []ScratchFeature{PersonAccounts, StateAndCountryPicklist, ContactsToMultipleAccounts}, map[string]string{})
	if len(result) != 3 {
		t.Errorf("Expected 3 features, got %d", len(result))
	}
	featureMap := make(map[string]bool)
	for _, f := range result {
		featureMap[f] = true
	}
	if !featureMap["PersonAccounts"] {
		t.Error("Expected PersonAccounts in result")
	}
	if !featureMap["StateAndCountryPicklist"] {
		t.Error("Expected StateAndCountryPicklist in result")
	}
	if !featureMap["ContactsToMultipleAccounts"] {
		t.Error("Expected ContactsToMultipleAccounts in result")
	}
}

func TestExpandProductsToFeatures_Communities(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{}, []ScratchFeature{Communities}, map[string]string{})
	if len(result) != 1 {
		t.Errorf("Expected 1 feature, got %d", len(result))
	}
	if result[0] != "Communities" {
		t.Errorf("Expected Communities, got %s", result[0])
	}
}

func TestExpandProductsToFeatures_CommunitiesProduct(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{CommunitiesProduct}, []ScratchFeature{}, map[string]string{})
	if len(result) != 1 {
		t.Errorf("Expected 1 feature from communities product, got %d", len(result))
	}
	if result[0] != "Communities" {
		t.Errorf("Expected Communities, got %s", result[0])
	}
}

func TestExpandProductsToFeatures_HealthCloudProduct(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{HealthCloudProduct}, []ScratchFeature{}, map[string]string{})
	if len(result) != 2 {
		t.Errorf("Expected 2 features from health cloud product, got %d", len(result))
	}
	featureMap := make(map[string]bool)
	for _, f := range result {
		featureMap[f] = true
	}
	if !featureMap["HealthCloudAddOn"] {
		t.Error("Expected HealthCloudAddOn in health cloud product")
	}
	if !featureMap["HealthCloudUser"] {
		t.Error("Expected HealthCloudUser in health cloud product")
	}
}

func TestExpandProductsToFeatures_ApexUserModeWithPermset(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{}, []ScratchFeature{ApexUserModeWithPermset}, map[string]string{})
	if len(result) != 1 {
		t.Fatalf("Expected 1 feature, got %d", len(result))
	}
	if result[0] != "ApexUserModeWithPermset" {
		t.Errorf("Expected ApexUserModeWithPermset, got %s", result[0])
	}
}

func TestExpandProductsToFeatures_EventLogFile(t *testing.T) {
	result := expandProductsToFeatures([]ScratchProduct{}, []ScratchFeature{EventLogFile}, map[string]string{})
	if len(result) != 1 {
		t.Fatalf("Expected 1 feature, got %d", len(result))
	}
	if result[0] != "EventLogFile" {
		t.Errorf("Expected EventLogFile, got %s", result[0])
	}
}

func TestExpandProductsToSettings_NoProductsOrSettings(t *testing.T) {
	result := expandProductsToSettings([]ScratchProduct{}, []ScratchSetting{})
	if len(result) != 0 {
		t.Errorf("Expected empty slice, got %v", result)
	}
}

func TestExpandProductsToSettings_SingleSetting(t *testing.T) {
	result := expandProductsToSettings([]ScratchProduct{}, []ScratchSetting{EnableEnhancedNotes})
	if len(result) != 1 {
		t.Errorf("Expected 1 setting, got %d", len(result))
	}
	if result[0] != "enableEnhancedNotes" {
		t.Errorf("Expected enableEnhancedNotes, got %s", result[0])
	}
}

func TestExpandProductsToSettings_CommunitiesProduct(t *testing.T) {
	result := expandProductsToSettings([]ScratchProduct{CommunitiesProduct}, []ScratchSetting{})
	if len(result) != 1 {
		t.Errorf("Expected 1 setting from communities product, got %d", len(result))
	}
	if result[0] != "networksEnabled" {
		t.Errorf("Expected networksEnabled, got %s", result[0])
	}
}

func TestExpandProductsToSettings_CommunitiesProductWithAdditionalSetting(t *testing.T) {
	result := expandProductsToSettings([]ScratchProduct{CommunitiesProduct}, []ScratchSetting{EnableEnhancedNotes})
	if len(result) != 2 {
		t.Errorf("Expected 2 settings, got %d", len(result))
	}
	settingMap := make(map[string]bool)
	for _, s := range result {
		settingMap[s] = true
	}
	if !settingMap["networksEnabled"] {
		t.Error("Expected networksEnabled in result")
	}
	if !settingMap["enableEnhancedNotes"] {
		t.Error("Expected enableEnhancedNotes in result")
	}
}

func TestExpandProductsToSettings_Deduplication(t *testing.T) {
	result := expandProductsToSettings([]ScratchProduct{CommunitiesProduct}, []ScratchSetting{NetworksEnabled})
	if len(result) != 1 {
		t.Errorf("Expected 1 unique setting (communities includes networksEnabled), got %d", len(result))
	}
	if result[0] != "networksEnabled" {
		t.Errorf("Expected networksEnabled, got %s", result[0])
	}
}

func TestScratchEditionIds_AllEditionsDefined(t *testing.T) {
	expectedEditions := map[string]bool{
		"Developer":           true,
		"Enterprise":          true,
		"Group":               true,
		"Professional":        true,
		"PartnerDeveloper":    true,
		"PartnerEnterprise":   true,
		"PartnerGroup":        true,
		"PartnerProfessional": true,
	}

	if len(ScratchEditionIds) != len(expectedEditions) {
		t.Errorf("Expected %d editions, got %d", len(expectedEditions), len(ScratchEditionIds))
	}

	for _, ids := range ScratchEditionIds {
		if len(ids) != 1 {
			t.Errorf("Expected 1 ID per edition, got %d", len(ids))
			continue
		}
		editionName := ids[0]
		if !expectedEditions[editionName] {
			t.Errorf("Unexpected edition: %s", editionName)
		}
	}
}

func TestConvertSettingsToStrings_NoSettings(t *testing.T) {
	result := convertSettingsToStrings([]ScratchSetting{})
	if len(result) != 0 {
		t.Errorf("Expected empty slice, got %v", result)
	}
}

func TestConvertSettingsToStrings_SingleSetting(t *testing.T) {
	result := convertSettingsToStrings([]ScratchSetting{EnableEnhancedNotes})
	if len(result) != 1 {
		t.Errorf("Expected 1 setting, got %d", len(result))
	}
	if result[0] != "enableEnhancedNotes" {
		t.Errorf("Expected enableEnhancedNotes, got %s", result[0])
	}
}

func TestConvertSettingsToStrings_EnableQuote(t *testing.T) {
	result := convertSettingsToStrings([]ScratchSetting{EnableQuote})
	if len(result) != 1 {
		t.Errorf("Expected 1 setting, got %d", len(result))
	}
	if result[0] != "enableQuote" {
		t.Errorf("Expected enableQuote, got %s", result[0])
	}
}

func TestConvertSettingsToStrings_NetworksEnabled(t *testing.T) {
	result := convertSettingsToStrings([]ScratchSetting{NetworksEnabled})
	if len(result) != 1 {
		t.Errorf("Expected 1 setting, got %d", len(result))
	}
	if result[0] != "networksEnabled" {
		t.Errorf("Expected networksEnabled, got %s", result[0])
	}
}

func TestConvertSettingsToStrings_EnableApexApprovalLockUnlock(t *testing.T) {
	result := convertSettingsToStrings([]ScratchSetting{EnableApexApprovalLockUnlock})
	if len(result) != 1 {
		t.Errorf("Expected 1 setting, got %d", len(result))
	}
	if result[0] != "enableApexApprovalLockUnlock" {
		t.Errorf("Expected enableApexApprovalLockUnlock, got %s", result[0])
	}
}

func TestConvertSettingsToStrings_PermsetsInFieldCreation(t *testing.T) {
	result := convertSettingsToStrings([]ScratchSetting{PermsetsInFieldCreation})
	if len(result) != 1 {
		t.Errorf("Expected 1 setting, got %d", len(result))
	}
	if result[0] != "permsetsInFieldCreation" {
		t.Errorf("Expected permsetsInFieldCreation, got %s", result[0])
	}
}

func TestScratchSettingIds_AllSettingsDefined(t *testing.T) {
	expectedSettings := map[string]bool{
		"enableEnhancedNotes":          true,
		"enableQuote":                  true,
		"networksEnabled":              true,
		"enableApexApprovalLockUnlock": true,
		"permsetsInFieldCreation":      true,
	}

	if len(ScratchSettingIds) != len(expectedSettings) {
		t.Errorf("Expected %d settings, got %d", len(expectedSettings), len(ScratchSettingIds))
	}

	for _, ids := range ScratchSettingIds {
		if len(ids) != 1 {
			t.Errorf("Expected 1 ID per setting, got %d", len(ids))
			continue
		}
		settingName := ids[0]
		if !expectedSettings[settingName] {
			t.Errorf("Unexpected setting: %s", settingName)
		}
	}
}
