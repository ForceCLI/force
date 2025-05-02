package command

import (
	"encoding/xml"
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"github.com/ForceCLI/force/lib"
)

// fakeQuerier simulates org queries for flow versions
type fakeQuerier struct {
	results map[string]lib.ForceQueryResult
}

// Test that package.xml members are updated to versioned flow names
func TestProcessSmartFlowVersion_PackageXmlUpdate(t *testing.T) {
	name := "MyFlow"
	// No existing versions => newVer = 1
	fq := &fakeQuerier{results: map[string]lib.ForceQueryResult{
		name: {Records: []lib.ForceRecord{}},
	}}
	// Sample package.xml with one Flow member
	pkgXml := []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>%s</members>
        <name>Flow</name>
    </types>
    <version>%s</version>
</Package>`, name, lib.ApiVersionNumber()))
	files := lib.ForceMetadataFiles{
		"flows/MyFlow.flow":          []byte("<flow></flow>"),
		"flows/MyFlow.flow-meta.xml": []byte("<fullName>MyFlow</fullName>"),
		"package.xml":                pkgXml,
	}
	out, err := processSmartFlowVersion(fq, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated, ok := out["package.xml"]
	if !ok {
		t.Fatalf("package.xml missing after processing")
	}
	// Unmarshal and verify member name updated
	type pkgType struct {
		Members []string `xml:"members"`
		Name    string   `xml:"name"`
	}
	type pkgStruct struct {
		Types []pkgType `xml:"types"`
	}
	var pkg pkgStruct
	if err := xml.Unmarshal(updated, &pkg); err != nil {
		t.Fatalf("failed to parse updated package.xml: %v", err)
	}
	found := false
	for _, tpe := range pkg.Types {
		if tpe.Name == "Flow" {
			for _, m := range tpe.Members {
				if m == "MyFlow-1" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Errorf("expected member MyFlow-1 in package.xml, got: %s", string(updated))
	}
}

func (f *fakeQuerier) Query(_ string, _ ...func(*lib.QueryOptions)) (lib.ForceQueryResult, error) {
	// Return the first available result
	for _, res := range f.results {
		return res, nil
	}
	return lib.ForceQueryResult{}, nil
}

// Test no flows present: files unchanged
func TestProcessSmartFlowVersion_NoFlows(t *testing.T) {
	fq := &fakeQuerier{results: map[string]lib.ForceQueryResult{}}
	files := lib.ForceMetadataFiles{
		"classes/MyClass.cls": []byte("class MyClass {}"),
	}
	out, err := processSmartFlowVersion(fq, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(out, files) {
		t.Errorf("expected files unchanged, got %v", out)
	}
}

// Test single active version: should assign next version, no destructiveChangesPost.xml
func TestProcessSmartFlowVersion_SingleActive(t *testing.T) {
	name := "MyFlow"
	fq := &fakeQuerier{results: map[string]lib.ForceQueryResult{
		name: {Records: []lib.ForceRecord{{"VersionNumber": float64(1), "Status": "Active"}}},
	}}
	files := lib.ForceMetadataFiles{
		"flows/MyFlow.flow":          []byte("<flow></flow>"),
		"flows/MyFlow.flow-meta.xml": []byte("<fullName>MyFlow</fullName>"),
	}
	out, err := processSmartFlowVersion(fq, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expect new version 2
	if _, ok := out["flows/MyFlow-2.flow"]; !ok {
		t.Errorf("missing new flow file")
	}
	// Expect meta fullName updated
	meta, ok := out["flows/MyFlow-2.flow-meta.xml"]
	if !ok {
		t.Fatalf("missing new meta file")
	}
	if !regexp.MustCompile(`<fullName>MyFlow-2</fullName>`).Match(meta) {
		t.Errorf("meta fullName not updated, got %s", meta)
	}
	// No destructiveChangesPost.xml
	if _, ok = out["destructiveChangesPost.xml"]; ok {
		t.Errorf("unexpected destructiveChangesPost.xml")
	}
}

// Test mixed versions: inactive and active
func TestProcessSmartFlowVersion_InactiveAndActive(t *testing.T) {
	name := "MyFlow"
	fq := &fakeQuerier{results: map[string]lib.ForceQueryResult{
		name: {Records: []lib.ForceRecord{
			{"VersionNumber": float64(2), "Status": "Inactive"},
			{"VersionNumber": float64(3), "Status": "Active"},
		}},
	}}
	files := lib.ForceMetadataFiles{
		"flows/MyFlow.flow":          []byte("<flow></flow>"),
		"flows/MyFlow.flow-meta.xml": []byte("<fullName>MyFlow</fullName>"),
	}
	out, err := processSmartFlowVersion(fq, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// New version should be 1
	if _, ok := out["flows/MyFlow-1.flow"]; !ok {
		t.Errorf("missing new flow version 1")
	}
	// Expect destructiveChangesPost.xml with MyFlow-2 only
	dc, ok := out["destructiveChangesPost.xml"]
	if !ok {
		t.Fatalf("missing destructiveChangesPost.xml")
	}
	var pkg struct {
		Types []struct {
			Members []string `xml:"members"`
		} `xml:"types"`
	}
	if err := xml.Unmarshal(dc, &pkg); err != nil {
		t.Fatalf("unmarshal dc xml: %v", err)
	}
	found := false
	for _, typ := range pkg.Types {
		for _, m := range typ.Members {
			if m == "MyFlow-2" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("destructiveChangesPost missing MyFlow-2, got %s", string(dc))
	}
}
