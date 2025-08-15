package command

import (
	"encoding/xml"
	"fmt"
	"reflect"
	"regexp"
	"strings"
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

func (f *fakeQuerier) Query(soql string, _ ...func(*lib.QueryOptions)) (lib.ForceQueryResult, error) {
	// Extract flow name from SOQL query (handles both = and LIKE operators)
	re := regexp.MustCompile(`WHERE FlowDefinitionView.ApiName (?:=|LIKE) '([^']+)'`)
	matches := re.FindStringSubmatch(soql)
	if len(matches) > 1 {
		flowName := matches[1]
		// For LIKE queries, do case-insensitive matching
		if strings.Contains(soql, "LIKE") {
			// Case-insensitive search through results
			for key, res := range f.results {
				if strings.EqualFold(key, flowName) {
					return res, nil
				}
			}
		} else {
			// Exact match for = operator
			if res, ok := f.results[flowName]; ok {
				return res, nil
			}
		}
	}
	// Return empty result if not found
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

// Test that existing destructiveChangesPost.xml is merged with flow deletions
func TestProcessSmartFlowVersion_MergeExistingDestructive(t *testing.T) {
	name := "MyFlow"
	fq := &fakeQuerier{results: map[string]lib.ForceQueryResult{
		name: {Records: []lib.ForceRecord{
			{"VersionNumber": float64(2), "Status": "Inactive"},
			{"VersionNumber": float64(3), "Status": "Active"},
		}},
	}}

	// Existing destructiveChangesPost.xml with some other metadata
	existingDC := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>TestClass</members>
        <name>ApexClass</name>
    </types>
    <version>61.0</version>
</Package>`)

	files := lib.ForceMetadataFiles{
		"flows/MyFlow.flow":          []byte("<flow></flow>"),
		"flows/MyFlow.flow-meta.xml": []byte("<fullName>MyFlow</fullName>"),
		"destructiveChangesPost.xml": existingDC,
	}

	out, err := processSmartFlowVersion(fq, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that destructiveChangesPost.xml contains both ApexClass and Flow types
	dc, ok := out["destructiveChangesPost.xml"]
	if !ok {
		t.Fatalf("missing destructiveChangesPost.xml")
	}

	var pkg struct {
		Types []struct {
			Members []string `xml:"members"`
			Name    string   `xml:"name"`
		} `xml:"types"`
	}
	if err := xml.Unmarshal(dc, &pkg); err != nil {
		t.Fatalf("unmarshal dc xml: %v", err)
	}

	foundApexClass := false
	foundFlow := false

	for _, typ := range pkg.Types {
		if typ.Name == "ApexClass" {
			for _, m := range typ.Members {
				if m == "TestClass" {
					foundApexClass = true
				}
			}
		}
		if typ.Name == "Flow" {
			for _, m := range typ.Members {
				if m == "MyFlow-2" {
					foundFlow = true
				}
			}
		}
	}

	if !foundApexClass {
		t.Errorf("existing ApexClass member missing from merged destructiveChangesPost.xml")
	}
	if !foundFlow {
		t.Errorf("new Flow member missing from merged destructiveChangesPost.xml")
	}
}

// Test handling flows in destructiveChanges.xml with versions in org
func TestProcessDestructiveFlows_WithVersions(t *testing.T) {
	flowName := "TestFlow"
	fq := &fakeQuerier{results: map[string]lib.ForceQueryResult{
		flowName: {Records: []lib.ForceRecord{
			{"VersionNumber": float64(1), "Status": "Active", "FlowDefinitionView.ApiName": "TestFlow"},
			{"VersionNumber": float64(2), "Status": "Inactive", "FlowDefinitionView.ApiName": "TestFlow"},
			{"VersionNumber": float64(3), "Status": "Active", "FlowDefinitionView.ApiName": "TestFlow"},
		}},
	}}

	destructiveXml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>TestFlow</members>
        <name>Flow</name>
    </types>
    <version>59.0</version>
</Package>`)

	packageXml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <version>59.0</version>
</Package>`)

	files := lib.ForceMetadataFiles{
		"destructiveChanges.xml": destructiveXml,
		"package.xml":            packageXml,
	}

	out, err := processDestructiveFlows(fq, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check destructiveChanges.xml has versioned flows
	dc, ok := out["destructiveChanges.xml"]
	if !ok {
		t.Fatalf("missing destructiveChanges.xml")
	}

	var pkg DestructivePackage
	if err := xml.Unmarshal(dc, &pkg); err != nil {
		t.Fatalf("failed to parse destructiveChanges.xml: %v", err)
	}

	// Should have TestFlow-1, TestFlow-2, TestFlow-3
	expectedVersions := []string{"TestFlow-1", "TestFlow-2", "TestFlow-3"}
	foundVersions := 0
	for _, tpe := range pkg.Types {
		if tpe.Name == "Flow" {
			for _, member := range tpe.Members {
				for _, expected := range expectedVersions {
					if member == expected {
						foundVersions++
					}
				}
			}
		}
	}

	if foundVersions != 3 {
		t.Errorf("expected 3 versioned flows, found %d", foundVersions)
	}

	// Check FlowDefinition was added to package.xml
	pkgXmlData, ok := out["package.xml"]
	if !ok {
		t.Fatalf("missing package.xml")
	}

	type pkgType struct {
		Members []string `xml:"members"`
		Name    string   `xml:"name"`
	}
	type pkgStruct struct {
		Types []pkgType `xml:"types"`
	}
	var pkgFile pkgStruct
	if err := xml.Unmarshal(pkgXmlData, &pkgFile); err != nil {
		t.Fatalf("failed to parse package.xml: %v", err)
	}

	foundFlowDef := false
	for _, tpe := range pkgFile.Types {
		if tpe.Name == "FlowDefinition" {
			for _, m := range tpe.Members {
				if m == flowName {
					foundFlowDef = true
				}
			}
		}
	}

	if !foundFlowDef {
		t.Errorf("FlowDefinition for %s not added to package.xml", flowName)
	}

	// Check FlowDefinition file was created
	flowDefFile := fmt.Sprintf("flowDefinitions/%s.flowDefinition-meta.xml", flowName)
	if _, ok := out[flowDefFile]; !ok {
		t.Errorf("FlowDefinition file %s not created", flowDefFile)
	}
}

// Test handling flows in destructiveChanges.xml with no versions in org
func TestProcessDestructiveFlows_NoVersions(t *testing.T) {
	flowName := "NonExistentFlow"
	fq := &fakeQuerier{results: map[string]lib.ForceQueryResult{
		flowName: {Records: []lib.ForceRecord{}}, // No versions
	}}

	destructiveXml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>NonExistentFlow</members>
        <name>Flow</name>
    </types>
    <version>59.0</version>
</Package>`)

	files := lib.ForceMetadataFiles{
		"destructiveChanges.xml": destructiveXml,
	}

	out, err := processDestructiveFlows(fq, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that flow was removed from destructiveChanges.xml
	dc, ok := out["destructiveChanges.xml"]
	if !ok {
		t.Fatalf("missing destructiveChanges.xml")
	}

	var pkg DestructivePackage
	if err := xml.Unmarshal(dc, &pkg); err != nil {
		t.Fatalf("failed to parse destructiveChanges.xml: %v", err)
	}

	// Should have no members left
	for _, tpe := range pkg.Types {
		if tpe.Name == "Flow" {
			for _, member := range tpe.Members {
				if member == flowName {
					t.Errorf("flow %s should have been removed from destructiveChanges.xml", flowName)
				}
			}
		}
	}
}

// Test handling mixed versioned and unversioned flows
func TestProcessDestructiveFlows_MixedFlows(t *testing.T) {
	fq := &fakeQuerier{results: map[string]lib.ForceQueryResult{
		"FlowWithVersions": {Records: []lib.ForceRecord{
			{"VersionNumber": float64(1), "Status": "Active", "FlowDefinitionView.ApiName": "FlowWithVersions"},
		}},
		"FlowWithoutVersions": {Records: []lib.ForceRecord{}},
	}}

	destructiveXml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>FlowWithVersions</members>
        <members>FlowWithoutVersions</members>
        <members>AlreadyVersioned-1</members>
        <name>Flow</name>
    </types>
    <version>59.0</version>
</Package>`)

	files := lib.ForceMetadataFiles{
		"destructiveChanges.xml": destructiveXml,
		"package.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <version>59.0</version>
</Package>`),
	}

	out, err := processDestructiveFlows(fq, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dc, ok := out["destructiveChanges.xml"]
	if !ok {
		t.Fatalf("missing destructiveChanges.xml")
	}

	var pkg DestructivePackage
	if err := xml.Unmarshal(dc, &pkg); err != nil {
		t.Fatalf("failed to parse destructiveChanges.xml: %v", err)
	}

	// Check expected members
	expectedMembers := map[string]bool{
		"FlowWithVersions-1": false,
		"AlreadyVersioned-1": false,
	}
	notExpected := []string{"FlowWithoutVersions", "FlowWithVersions"}

	for _, tpe := range pkg.Types {
		if tpe.Name == "Flow" {
			for _, member := range tpe.Members {
				if _, ok := expectedMembers[member]; ok {
					expectedMembers[member] = true
				}
				for _, ne := range notExpected {
					if member == ne {
						t.Errorf("unexpected member %s in destructiveChanges.xml", member)
					}
				}
			}
		}
	}

	for member, found := range expectedMembers {
		if !found {
			t.Errorf("expected member %s not found in destructiveChanges.xml", member)
		}
	}
}

// Test handling flows with case-mismatched names in destructive changes
func TestProcessDestructiveFlows_CaseMismatch(t *testing.T) {
	// Org has flow with name "MyFlow" but destructive changes has "myflow"
	fq := &fakeQuerier{results: map[string]lib.ForceQueryResult{
		"myflow": {Records: []lib.ForceRecord{
			{"VersionNumber": float64(1), "Status": "Active", "FlowDefinitionView.ApiName": "MyFlow"},
			{"VersionNumber": float64(2), "Status": "Inactive", "FlowDefinitionView.ApiName": "MyFlow"},
		}},
	}}

	destructiveXml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>myflow</members>
        <name>Flow</name>
    </types>
    <version>59.0</version>
</Package>`)

	files := lib.ForceMetadataFiles{
		"destructiveChanges.xml": destructiveXml,
		"package.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <version>59.0</version>
</Package>`),
	}

	out, err := processDestructiveFlows(fq, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dc, ok := out["destructiveChanges.xml"]
	if !ok {
		t.Fatalf("missing destructiveChanges.xml")
	}

	var pkg DestructivePackage
	if err := xml.Unmarshal(dc, &pkg); err != nil {
		t.Fatalf("failed to parse destructiveChanges.xml: %v", err)
	}

	// Should have MyFlow-1 and MyFlow-2 (using correct casing from org)
	expectedVersions := []string{"MyFlow-1", "MyFlow-2"}
	foundVersions := 0
	for _, tpe := range pkg.Types {
		if tpe.Name == "Flow" {
			for _, member := range tpe.Members {
				for _, expected := range expectedVersions {
					if member == expected {
						foundVersions++
					}
				}
				// Should NOT have the lowercase version
				if member == "myflow-1" || member == "myflow-2" {
					t.Errorf("found lowercase version %s, should use org casing", member)
				}
			}
		}
	}

	if foundVersions != 2 {
		t.Errorf("expected 2 versioned flows with correct casing, found %d", foundVersions)
	}

	// Check FlowDefinition was added with correct casing
	flowDefFile := "flowDefinitions/MyFlow.flowDefinition-meta.xml"
	if _, ok := out[flowDefFile]; !ok {
		t.Errorf("FlowDefinition file %s not created with correct casing", flowDefFile)
	}
}

// Test handling duplicate flows with different casing in destructive changes
func TestProcessDestructiveFlows_DuplicateCasing(t *testing.T) {
	// Both queries return the same flow (with correct casing from org)
	fq := &fakeQuerier{results: map[string]lib.ForceQueryResult{
		"Member_Plan_After_Save": {Records: []lib.ForceRecord{
			{"VersionNumber": float64(2), "Status": "Active", "FlowDefinitionView.ApiName": "Member_Plan_After_save"},
			{"VersionNumber": float64(3), "Status": "Inactive", "FlowDefinitionView.ApiName": "Member_Plan_After_save"},
		}},
		"Member_Plan_After_save": {Records: []lib.ForceRecord{
			{"VersionNumber": float64(2), "Status": "Active", "FlowDefinitionView.ApiName": "Member_Plan_After_save"},
			{"VersionNumber": float64(3), "Status": "Inactive", "FlowDefinitionView.ApiName": "Member_Plan_After_save"},
		}},
	}}

	destructiveXml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>Member_Plan_After_Save</members>
        <members>Member_Plan_After_save</members>
        <name>Flow</name>
    </types>
    <version>59.0</version>
</Package>`)

	files := lib.ForceMetadataFiles{
		"destructiveChanges.xml": destructiveXml,
		"package.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <version>59.0</version>
</Package>`),
	}

	out, err := processDestructiveFlows(fq, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dc, ok := out["destructiveChanges.xml"]
	if !ok {
		t.Fatalf("missing destructiveChanges.xml")
	}

	var pkg DestructivePackage
	if err := xml.Unmarshal(dc, &pkg); err != nil {
		t.Fatalf("failed to parse destructiveChanges.xml: %v", err)
	}

	// Should have only unique versions from org (Member_Plan_After_save-2 and Member_Plan_After_save-3)
	expectedVersions := map[string]bool{
		"Member_Plan_After_save-2": false,
		"Member_Plan_After_save-3": false,
	}

	actualMembers := []string{}
	for _, tpe := range pkg.Types {
		if tpe.Name == "Flow" {
			actualMembers = tpe.Members
			for _, member := range tpe.Members {
				if expected, ok := expectedVersions[member]; ok && !expected {
					expectedVersions[member] = true
				} else if ok && expected {
					t.Errorf("duplicate member %s in destructiveChanges.xml", member)
				}
			}
		}
	}

	// Check we have exactly the expected versions
	for version, found := range expectedVersions {
		if !found {
			t.Errorf("expected version %s not found in destructiveChanges.xml", version)
		}
	}

	// Should have exactly 2 members (deduplicated)
	if len(actualMembers) != 2 {
		t.Errorf("expected exactly 2 unique versions, got %d: %v", len(actualMembers), actualMembers)
	}

	// Check FlowDefinition was added with correct casing (only one)
	flowDefFile := "flowDefinitions/Member_Plan_After_save.flowDefinition-meta.xml"
	if _, ok := out[flowDefFile]; !ok {
		t.Errorf("FlowDefinition file %s not created with correct casing from org", flowDefFile)
	}
}

// Test that FlowDefinition is only added when there's an active version
func TestProcessDestructiveFlows_NoFlowDefinitionForInactiveOnly(t *testing.T) {
	fq := &fakeQuerier{results: map[string]lib.ForceQueryResult{
		"InactiveOnlyFlow": {Records: []lib.ForceRecord{
			{"VersionNumber": float64(1), "Status": "Inactive", "FlowDefinitionView.ApiName": "InactiveOnlyFlow"},
			{"VersionNumber": float64(2), "Status": "Obsolete", "FlowDefinitionView.ApiName": "InactiveOnlyFlow"},
		}},
		"ActiveFlow": {Records: []lib.ForceRecord{
			{"VersionNumber": float64(1), "Status": "Active", "FlowDefinitionView.ApiName": "ActiveFlow"},
			{"VersionNumber": float64(2), "Status": "Inactive", "FlowDefinitionView.ApiName": "ActiveFlow"},
		}},
	}}

	destructiveXml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>InactiveOnlyFlow</members>
        <members>ActiveFlow</members>
        <name>Flow</name>
    </types>
    <version>59.0</version>
</Package>`)

	files := lib.ForceMetadataFiles{
		"destructiveChanges.xml": destructiveXml,
		"package.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <version>59.0</version>
</Package>`),
	}

	out, err := processDestructiveFlows(fq, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that FlowDefinition was NOT added for InactiveOnlyFlow
	inactiveFlowDefFile := "flowDefinitions/InactiveOnlyFlow.flowDefinition-meta.xml"
	if _, ok := out[inactiveFlowDefFile]; ok {
		t.Errorf("FlowDefinition file %s should not be created for flow with no active versions", inactiveFlowDefFile)
	}

	// Check that FlowDefinition WAS added for ActiveFlow
	activeFlowDefFile := "flowDefinitions/ActiveFlow.flowDefinition-meta.xml"
	if _, ok := out[activeFlowDefFile]; !ok {
		t.Errorf("FlowDefinition file %s should be created for flow with active version", activeFlowDefFile)
	}

	// Check package.xml only has ActiveFlow in FlowDefinition
	pkgXml, ok := out["package.xml"]
	if !ok {
		t.Fatalf("missing package.xml")
	}

	type pkgType struct {
		Members []string `xml:"members"`
		Name    string   `xml:"name"`
	}
	type pkgStruct struct {
		Types []pkgType `xml:"types"`
	}
	var pkg pkgStruct
	if err := xml.Unmarshal(pkgXml, &pkg); err != nil {
		t.Fatalf("failed to parse package.xml: %v", err)
	}

	for _, tpe := range pkg.Types {
		if tpe.Name == "FlowDefinition" {
			for _, m := range tpe.Members {
				if m == "InactiveOnlyFlow" {
					t.Errorf("InactiveOnlyFlow should not be in FlowDefinition members")
				}
				if m == "ActiveFlow" {
					// This is expected
					continue
				}
			}
		}
	}
}
