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
