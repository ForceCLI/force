package lib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const existingOpportunity = `<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
    <actionOverrides>
        <actionName>Edit</actionName>
        <type>Default</type>
    </actionOverrides>
    <actionOverrides>
        <actionName>View</actionName>
        <type>Default</type>
    </actionOverrides>
    <enableFeeds>true</enableFeeds>
    <fields>
        <fullName>Amount__c</fullName>
        <label>Amount</label>
        <type>Currency</type>
    </fields>
    <fields>
        <fullName>Stage__c</fullName>
        <label>Old Stage</label>
        <type>Text</type>
    </fields>
    <listViews>
        <fullName>All</fullName>
        <filterScope>Everything</filterScope>
        <label>All</label>
    </listViews>
</CustomObject>
`

func retrievedFields(fields ...string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">`)
	for _, f := range fields {
		b.WriteString("\n    ")
		b.WriteString(f)
	}
	b.WriteString("\n</CustomObject>\n")
	return b.String()
}

func TestMergeComponentXml_InsertsNewChildInSortedPosition(t *testing.T) {
	retrieved := retrievedFields(`<fields>
        <fullName>Buyer__c</fullName>
        <label>Buyer</label>
        <type>Lookup</type>
    </fields>`)
	merged, err := mergeComponentXml([]byte(existingOpportunity), []byte(retrieved), []string{"fields", "listViews"})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(existingOpportunity, `    <fields>
        <fullName>Stage__c</fullName>`, `    <fields>
        <fullName>Buyer__c</fullName>
        <label>Buyer</label>
        <type>Lookup</type>
    </fields>
    <fields>
        <fullName>Stage__c</fullName>`, 1)
	if string(merged) != want {
		t.Errorf("merged =\n%s\nwant\n%s", merged, want)
	}
}

func TestMergeComponentXml_ReplacesExistingChildInPlace(t *testing.T) {
	retrieved := retrievedFields(`<fields>
        <fullName>Stage__c</fullName>
        <label>New Stage</label>
        <type>Picklist</type>
    </fields>`)
	merged, err := mergeComponentXml([]byte(existingOpportunity), []byte(retrieved), []string{"fields", "listViews"})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(existingOpportunity, `        <fullName>Stage__c</fullName>
        <label>Old Stage</label>
        <type>Text</type>`, `        <fullName>Stage__c</fullName>
        <label>New Stage</label>
        <type>Picklist</type>`, 1)
	if string(merged) != want {
		t.Errorf("merged =\n%s\nwant\n%s", merged, want)
	}
}

func TestMergeComponentXml_AppendsChildSortedAfterAllExisting(t *testing.T) {
	retrieved := retrievedFields(`<webLinks>
        <fullName>Google</fullName>
        <url>https://www.google.com</url>
    </webLinks>`)
	merged, err := mergeComponentXml([]byte(existingOpportunity), []byte(retrieved), []string{"fields", "listViews", "webLinks"})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(existingOpportunity, "    </listViews>\n", `    </listViews>
    <webLinks>
        <fullName>Google</fullName>
        <url>https://www.google.com</url>
    </webLinks>
`, 1)
	if string(merged) != want {
		t.Errorf("merged =\n%s\nwant\n%s", merged, want)
	}
}

func TestMergeComponentXml_ReplacesAllElementsOfNonChildTag(t *testing.T) {
	retrieved := retrievedFields(`<actionOverrides>
        <actionName>Edit</actionName>
        <type>Flexipage</type>
    </actionOverrides>`, `<enableFeeds>false</enableFeeds>`)
	merged, err := mergeComponentXml([]byte(existingOpportunity), []byte(retrieved), []string{"fields", "listViews"})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(existingOpportunity, `    <actionOverrides>
        <actionName>Edit</actionName>
        <type>Default</type>
    </actionOverrides>
    <actionOverrides>
        <actionName>View</actionName>
        <type>Default</type>
    </actionOverrides>
    <enableFeeds>true</enableFeeds>`, `    <actionOverrides>
        <actionName>Edit</actionName>
        <type>Flexipage</type>
    </actionOverrides>
    <enableFeeds>false</enableFeeds>`, 1)
	if string(merged) != want {
		t.Errorf("merged =\n%s\nwant\n%s", merged, want)
	}
}

func TestMergeComponentXml_IntoRootWithNoElements(t *testing.T) {
	existing := `<?xml version="1.0" encoding="UTF-8"?>
<CustomLabels xmlns="http://soap.sforce.com/2006/04/metadata">
</CustomLabels>
`
	retrieved := `<?xml version="1.0" encoding="UTF-8"?>
<CustomLabels xmlns="http://soap.sforce.com/2006/04/metadata">
    <labels>
        <fullName>Greeting</fullName>
        <value>Hello</value>
    </labels>
</CustomLabels>
`
	merged, err := mergeComponentXml([]byte(existing), []byte(retrieved), []string{"labels"})
	if err != nil {
		t.Fatal(err)
	}
	if string(merged) != retrieved {
		t.Errorf("merged =\n%s\nwant\n%s", merged, retrieved)
	}
}

func TestMergeComponentXml_RejectsDifferentRootElement(t *testing.T) {
	_, err := mergeComponentXml([]byte(existingOpportunity), []byte(`<Workflow></Workflow>`), []string{"fields"})
	if err == nil {
		t.Error("expected an error for mismatched root elements")
	}
}

func TestMergeComponentXml_RejectsInvalidExistingFile(t *testing.T) {
	_, err := mergeComponentXml([]byte(`<CustomObject><fields>`), []byte(retrievedFields()), []string{"fields"})
	if err == nil {
		t.Error("expected an error for an unparseable existing file")
	}
}

func TestContainerForEntry(t *testing.T) {
	tests := []struct {
		name       string
		wantType   string
		wantMember string
		wantOk     bool
	}{
		{"objects/Opportunity.object", "CustomObject", "Opportunity", true},
		{"workflows/Case.workflow", "Workflow", "Case", true},
		{"labels/CustomLabels.labels", "CustomLabels", "CustomLabels", true},
		{"matchingRules/Account.matchingRule", "MatchingRules", "Account", true},
		{"classes/Foo.cls", "", "", false},
		{"objects/.object", "", "", false},
		{"objectTranslations/Account-en_US.objectTranslation", "", "", false},
	}
	for _, tt := range tests {
		c, member, ok := containerForEntry(tt.name)
		if ok != tt.wantOk || c.name != tt.wantType || member != tt.wantMember {
			t.Errorf("containerForEntry(%q) = %q, %q, %v; want %q, %q, %v", tt.name, c.name, member, ok, tt.wantType, tt.wantMember, tt.wantOk)
		}
	}
}

func writeTestFile(t *testing.T, root, name, content string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const buyerField = `<fields>
        <fullName>Buyer__c</fullName>
        <label>Buyer</label>
        <type>Lookup</type>
    </fields>`

func TestMergingEntryDiskWriter_MergesChildComponentIntoExistingObject(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "objects/Opportunity.object", existingOpportunity)
	query := ForceMetadataQuery{{Name: []string{"CustomField"}, Members: []string{"Opportunity.Buyer__c"}}}
	write := MergingEntryDiskWriter(root, query)

	if err := write("objects/Opportunity.object", strings.NewReader(retrievedFields(buyerField))); err != nil {
		t.Fatal(err)
	}
	got := readTestFile(t, root, "objects/Opportunity.object")
	for _, want := range []string{"<fullName>Buyer__c</fullName>", "<fullName>Amount__c</fullName>", "<fullName>All</fullName>", "<enableFeeds>true</enableFeeds>"} {
		if !strings.Contains(got, want) {
			t.Errorf("merged file is missing %s:\n%s", want, got)
		}
	}
}

func TestMergingEntryDiskWriter_WritesNewFileWhenNoneExists(t *testing.T) {
	root := t.TempDir()
	query := ForceMetadataQuery{{Name: []string{"CustomField"}, Members: []string{"Opportunity.Buyer__c"}}}
	write := MergingEntryDiskWriter(root, query)

	retrieved := retrievedFields(buyerField)
	if err := write("objects/Opportunity.object", strings.NewReader(retrieved)); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, root, "objects/Opportunity.object"); got != retrieved {
		t.Errorf("file =\n%s\nwant\n%s", got, retrieved)
	}
}

func TestMergingEntryDiskWriter_ReplacesFileWhenParentIsRequested(t *testing.T) {
	for _, member := range []string{"Opportunity", "opportunity", "*"} {
		root := t.TempDir()
		writeTestFile(t, root, "objects/Opportunity.object", existingOpportunity)
		query := ForceMetadataQuery{{Name: []string{"CustomObject"}, Members: []string{member}}}
		write := MergingEntryDiskWriter(root, query)

		retrieved := retrievedFields(buyerField)
		if err := write("objects/Opportunity.object", strings.NewReader(retrieved)); err != nil {
			t.Fatal(err)
		}
		if got := readTestFile(t, root, "objects/Opportunity.object"); got != retrieved {
			t.Errorf("member %q: file =\n%s\nwant\n%s", member, got, retrieved)
		}
	}
}

func TestMergingEntryDiskWriter_MergesSecondEntryForSameFile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "objects/Opportunity.object", existingOpportunity)
	query := ForceMetadataQuery{
		{Name: []string{"CustomObject"}, Members: []string{"Opportunity"}},
		{Name: []string{"CustomField"}, Members: []string{"Opportunity.Buyer__c"}},
	}
	write := MergingEntryDiskWriter(root, query)

	// A query split into multiple retrieve calls can return the same file
	// once per batch.
	if err := write("objects/Opportunity.object", strings.NewReader(existingOpportunity)); err != nil {
		t.Fatal(err)
	}
	if err := write("objects/Opportunity.object", strings.NewReader(retrievedFields(buyerField))); err != nil {
		t.Fatal(err)
	}
	got := readTestFile(t, root, "objects/Opportunity.object")
	for _, want := range []string{"<fullName>Buyer__c</fullName>", "<fullName>Amount__c</fullName>"} {
		if !strings.Contains(got, want) {
			t.Errorf("merged file is missing %s:\n%s", want, got)
		}
	}
}

func TestMergingEntryDiskWriter_WritesOtherEntriesUnchanged(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "classes/Foo.cls", "old")
	write := MergingEntryDiskWriter(root, ForceMetadataQuery{{Name: []string{"ApexClass"}, Members: []string{"Foo"}}})
	if err := write("classes/Foo.cls", strings.NewReader("new")); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, root, "classes/Foo.cls"); got != "new" {
		t.Errorf("file = %q, want %q", got, "new")
	}
}

func TestMergingEntryDiskWriter_RejectsPathTraversal(t *testing.T) {
	write := MergingEntryDiskWriter(t.TempDir(), nil)
	if err := write("../objects/Account.object", strings.NewReader("<CustomObject/>")); err == nil {
		t.Error("expected an error for an entry outside root")
	}
}
