package lib

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
)

func memberNames(n int, prefix string) []string {
	members := make([]string, n)
	for i := range members {
		members[i] = fmt.Sprintf("%s%d", prefix, i)
	}
	return members
}

func countMembers(query ForceMetadataQuery) int {
	count := 0
	for _, element := range query {
		count += len(element.Name) * len(element.Members)
	}
	return count
}

func TestSplitQuery_UnderLimit(t *testing.T) {
	query := ForceMetadataQuery{
		{Name: []string{"ApexClass"}, Members: []string{"*"}},
		{Name: []string{"Report"}, Members: memberNames(100, "r")},
	}
	batches := splitQuery(query, 10000)
	if len(batches) != 1 {
		t.Fatalf("got %d batches, want 1", len(batches))
	}
	if countMembers(batches[0]) != 101 {
		t.Errorf("got %d members, want 101", countMembers(batches[0]))
	}
}

func TestSplitQuery_SingleElementOverLimit(t *testing.T) {
	query := ForceMetadataQuery{
		{Name: []string{"Report"}, Members: memberNames(25000, "r")},
	}
	batches := splitQuery(query, 10000)
	if len(batches) != 3 {
		t.Fatalf("got %d batches, want 3", len(batches))
	}
	total := 0
	for i, batch := range batches {
		n := countMembers(batch)
		if n > 10000 {
			t.Errorf("batch %d has %d members, exceeds limit", i, n)
		}
		total += n
	}
	if total != 25000 {
		t.Errorf("total members = %d, want 25000", total)
	}
	// No member should be lost or duplicated
	seen := make(map[string]bool)
	for _, batch := range batches {
		for _, element := range batch {
			for _, member := range element.Members {
				if seen[member] {
					t.Fatalf("member %s appears in multiple batches", member)
				}
				seen[member] = true
			}
		}
	}
}

func TestSplitQuery_MultipleElementsAccumulate(t *testing.T) {
	query := ForceMetadataQuery{
		{Name: []string{"Report"}, Members: memberNames(6000, "r")},
		{Name: []string{"Dashboard"}, Members: memberNames(6000, "d")},
	}
	batches := splitQuery(query, 10000)
	if len(batches) != 2 {
		t.Fatalf("got %d batches, want 2", len(batches))
	}
	if n := countMembers(batches[0]); n != 10000 {
		t.Errorf("first batch has %d members, want 10000", n)
	}
	if n := countMembers(batches[1]); n != 2000 {
		t.Errorf("second batch has %d members, want 2000", n)
	}
	// The split element's name must carry over to the second batch
	if batches[1][0].Name[0] != "Dashboard" {
		t.Errorf("second batch starts with %v, want Dashboard", batches[1][0].Name)
	}
}

func TestSplitQuery_MultiNameElement(t *testing.T) {
	query := ForceMetadataQuery{
		{Name: []string{"Report", "Dashboard"}, Members: memberNames(8000, "m")},
	}
	batches := splitQuery(query, 10000)
	if len(batches) != 2 {
		t.Fatalf("got %d batches, want 2", len(batches))
	}
	if total := countMembers(batches[0]) + countMembers(batches[1]); total != 16000 {
		t.Errorf("total members = %d, want 16000", total)
	}
}

func TestSplitQuery_EmptyMembersPreserved(t *testing.T) {
	query := ForceMetadataQuery{
		{Name: []string{"Settings"}, Members: nil},
		{Name: []string{"ApexClass"}, Members: []string{"*"}},
	}
	batches := splitQuery(query, 10000)
	if len(batches) != 1 {
		t.Fatalf("got %d batches, want 1", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Fatalf("got %d elements, want 2", len(batches[0]))
	}
	if batches[0][0].Name[0] != "Settings" {
		t.Errorf("first element = %v, want Settings", batches[0][0].Name)
	}
}

func TestMergePackageXml(t *testing.T) {
	doc1 := []byte(xml.Header + `<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>ClassOne</members>
        <name>ApexClass</name>
    </types>
    <types>
        <members>ReportA</members>
        <members>ReportB</members>
        <name>Report</name>
    </types>
    <version>62.0</version>
</Package>`)
	doc2 := []byte(xml.Header + `<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>ReportC</members>
        <name>Report</name>
    </types>
    <types>
        <members>DashOne</members>
        <name>Dashboard</name>
    </types>
    <version>62.0</version>
</Package>`)

	merged, err := mergePackageXml(doc1, doc2)
	if err != nil {
		t.Fatal(err)
	}
	var p Package
	if err := xml.Unmarshal(merged, &p); err != nil {
		t.Fatal(err)
	}
	if p.Version != "62.0" {
		t.Errorf("version = %q, want 62.0", p.Version)
	}
	if p.Xmlns != "http://soap.sforce.com/2006/04/metadata" {
		t.Errorf("xmlns = %q", p.Xmlns)
	}
	if len(p.Types) != 3 {
		t.Fatalf("got %d types, want 3", len(p.Types))
	}
	byName := make(map[string][]string)
	for _, mt := range p.Types {
		byName[mt.Name] = mt.Members
	}
	if got := strings.Join(byName["Report"], ","); got != "ReportA,ReportB,ReportC" {
		t.Errorf("Report members = %s", got)
	}
	if got := strings.Join(byName["ApexClass"], ","); got != "ClassOne" {
		t.Errorf("ApexClass members = %s", got)
	}
	if got := strings.Join(byName["Dashboard"], ","); got != "DashOne" {
		t.Errorf("Dashboard members = %s", got)
	}
	if !strings.HasPrefix(string(merged), xml.Header) {
		t.Errorf("merged package.xml missing XML header")
	}
}

func TestIsRetrieveLimitError(t *testing.T) {
	limitErr := fmt.Errorf("LIMIT_EXCEEDED: Too many files in retrieve call, limit is: 10000")
	if !isRetrieveLimitError(limitErr) {
		t.Error("expected retrieve limit error to be detected")
	}
	if isRetrieveLimitError(fmt.Errorf("LIMIT_EXCEEDED: Too many API calls")) {
		t.Error("other LIMIT_EXCEEDED errors should not match")
	}
	if isRetrieveLimitError(nil) {
		t.Error("nil should not match")
	}
}

func TestSplitQuery_Bisect(t *testing.T) {
	// Bisecting a batch, as done when the server reports LIMIT_EXCEEDED,
	// must always produce smaller batches so the retry terminates
	query := ForceMetadataQuery{
		{Name: []string{"ApexClass"}, Members: []string{"*"}},
		{Name: []string{"Report"}, Members: memberNames(5000, "r")},
	}
	total := countQueryMembers(query)
	halves := splitQuery(query, (total+1)/2)
	if len(halves) != 2 {
		t.Fatalf("got %d batches, want 2", len(halves))
	}
	for i, half := range halves {
		if n := countQueryMembers(half); n >= total {
			t.Errorf("half %d has %d members, not smaller than %d", i, n, total)
		}
	}
}

func TestPackageXmlToQuery(t *testing.T) {
	data := []byte(xml.Header + `<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>Foo</members>
        <members>Bar</members>
        <name>ApexClass</name>
    </types>
    <types>
        <members>*</members>
        <name>CustomObject</name>
    </types>
    <version>62.0</version>
</Package>`)
	query, err := PackageXmlToQuery(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(query) != 2 {
		t.Fatalf("got %d elements, want 2", len(query))
	}
	if query[0].Name[0] != "ApexClass" || strings.Join(query[0].Members, ",") != "Foo,Bar" {
		t.Errorf("first element = %+v", query[0])
	}
	if query[1].Name[0] != "CustomObject" || strings.Join(query[1].Members, ",") != "*" {
		t.Errorf("second element = %+v", query[1])
	}
}

func TestMergePackageXml_Invalid(t *testing.T) {
	if _, err := mergePackageXml([]byte("<Package><types>")); err == nil {
		t.Fatal("expected error for malformed package.xml")
	}
}
