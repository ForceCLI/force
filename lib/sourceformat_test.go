package lib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const retrievedAccountField = `<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
    <fields>
        <fullName>Ecommerce_Email_ID__c</fullName>
        <externalId>false</externalId>
        <label>Ecommerce Email ID</label>
        <required>false</required>
        <type>Text</type>
        <unique>false</unique>
    </fields>
</CustomObject>
`

func TestSourceFormatEntryWriter_WritesRetrievedFieldAsFieldFile(t *testing.T) {
	root := t.TempDir()
	objectMeta := "existing object file"
	writeTestFile(t, root, "objects/Account/Account.object-meta.xml", objectMeta)
	query := ForceMetadataQuery{{Name: []string{"CustomField"}, Members: []string{"Account.Ecommerce_Email_ID__c"}}}
	write := SourceFormatEntryWriter(root, query)

	if err := write("objects/Account.object", strings.NewReader(retrievedAccountField)); err != nil {
		t.Fatal(err)
	}
	got := readTestFile(t, root, "objects/Account/fields/Ecommerce_Email_ID__c.field-meta.xml")
	want := `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>Ecommerce_Email_ID__c</fullName>
    <externalId>false</externalId>
    <label>Ecommerce Email ID</label>
    <required>false</required>
    <type>Text</type>
    <unique>false</unique>
</CustomField>
`
	if got != want {
		t.Errorf("field file =\n%s\nwant\n%s", got, want)
	}
	if got := readTestFile(t, root, "objects/Account/Account.object-meta.xml"); got != objectMeta {
		t.Errorf("object file was replaced:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(root, "objects", "Account.object")); !os.IsNotExist(err) {
		t.Error("metadata format object file should not be written")
	}
}

func TestSourceFormatEntryWriter_WritesObjectFileWhenObjectIsRequested(t *testing.T) {
	root := t.TempDir()
	query := ForceMetadataQuery{{Name: []string{"CustomObject"}, Members: []string{"Account"}}}
	write := SourceFormatEntryWriter(root, query)

	retrieved := strings.Replace(retrievedAccountField, "    <fields>", "    <enableFeeds>true</enableFeeds>\n    <fields>", 1)
	if err := write("objects/Account.object", strings.NewReader(retrieved)); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, root, "objects/Account/Account.object-meta.xml"); !strings.Contains(got, "<enableFeeds>true</enableFeeds>") {
		t.Errorf("object file =\n%s", got)
	}
	readTestFile(t, root, "objects/Account/fields/Ecommerce_Email_ID__c.field-meta.xml")
}

func TestSourceFormatEntryWriter_RenamesEntries(t *testing.T) {
	tests := []struct {
		entry string
		want  string
	}{
		{"classes/Foo.cls", "classes/Foo.cls"},
		{"classes/Foo.cls-meta.xml", "classes/Foo.cls-meta.xml"},
		{"layouts/Account-Account Layout.layout", "layouts/Account-Account Layout.layout-meta.xml"},
		{"customMetadata/Setting.Default.md", "customMetadata/Setting.Default.md-meta.xml"},
		{"reports/Sales-meta.xml", "reports/Sales.reportFolder-meta.xml"},
		{"reports/Sales/Pipeline.report", "reports/Sales/Pipeline.report-meta.xml"},
		{"lwc/hello/hello.js", "lwc/hello/hello.js"},
	}
	for _, tt := range tests {
		root := t.TempDir()
		write := SourceFormatEntryWriter(root, nil)
		if err := write(tt.entry, strings.NewReader("content")); err != nil {
			t.Fatal(err)
		}
		if got := readTestFile(t, root, tt.want); got != "content" {
			t.Errorf("%s: %s = %q", tt.entry, tt.want, got)
		}
	}
}

func TestSourceFormatEntryWriter_SkipsPackageXml(t *testing.T) {
	root := t.TempDir()
	write := SourceFormatEntryWriter(root, nil)
	if err := write("package.xml", strings.NewReader("<Package/>")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "package.xml")); !os.IsNotExist(err) {
		t.Error("package.xml should not be written into a source format directory")
	}
}
