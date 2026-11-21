package command

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func sourceDirForDirectory(t *testing.T, directory string) string {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().StringP("directory", "d", "src", "relative path to package.xml")
	if err := cmd.Flags().Set("directory", directory); err != nil {
		t.Fatalf("failed to set directory flag: %v", err)
	}
	return sourceDir(cmd)
}

func TestSourceDir_AbsoluteTrailingSlashStripped(t *testing.T) {
	root := sourceDirForDirectory(t, "/tmp/sf-education-cloud/shape_1780009540656/")
	want := "/tmp/sf-education-cloud/shape_1780009540656"
	if root != want {
		t.Errorf("Expected %s, got %s", want, root)
	}
}

func TestSourceDir_AbsoluteNoTrailingSlash(t *testing.T) {
	root := sourceDirForDirectory(t, "/tmp/sf-education-cloud/shape_1780009540656")
	want := "/tmp/sf-education-cloud/shape_1780009540656"
	if root != want {
		t.Errorf("Expected %s, got %s", want, root)
	}
}

func TestSourceDir_RelativeTrailingSlashStripped(t *testing.T) {
	root := sourceDirForDirectory(t, "unpackaged/")
	// Relative paths are resolved against the working directory; the result
	// must not retain a trailing separator.
	if filepath.Base(root) != "unpackaged" {
		t.Errorf("Expected base 'unpackaged', got %s (full: %s)", filepath.Base(root), root)
	}
}

func writeImportFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

func importFileNames(files ForceMetadataFiles) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, filepath.ToSlash(name))
	}
	sort.Strings(names)
	return names
}

const importTestPackageXml = `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
	<types>
		<members>Project__c</members>
		<name>CustomObject</name>
	</types>
	<version>62.0</version>
</Package>
`

// A directory in source format is converted to metadata format: an object's
// component files are composed into the object's file, a folder file takes
// its metadata format name, and a -meta.xml file of a type with no content
// file loses the suffix. The directory's package.xml is deployed as it is.
func TestImportFiles_ConvertsSourceFormat(t *testing.T) {
	root := t.TempDir()
	writeImportFixture(t, root, map[string]string{
		"package.xml": importTestPackageXml,
		"objects/Project__c/Project__c.object-meta.xml": `<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
    <deploymentStatus>Deployed</deploymentStatus>
    <label>Project</label>
    <nameField>
        <label>Project Name</label>
        <type>Text</type>
    </nameField>
    <pluralLabel>Projects</pluralLabel>
    <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`,
		"objects/Project__c/fields/Budget__c.field-meta.xml": `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>Budget__c</fullName>
    <label>Budget</label>
    <precision>18</precision>
    <scale>2</scale>
    <type>Currency</type>
</CustomField>
`,
		"reports/Demo_Reports.reportFolder-meta.xml": `<?xml version="1.0" encoding="UTF-8"?>
<ReportFolder xmlns="http://soap.sforce.com/2006/04/metadata">
    <accessType>Public</accessType>
    <name>Demo Reports</name>
</ReportFolder>
`,
		"reports/Demo_Reports/By_Status.report-meta.xml": "<Report/>",
		"tabs/Project__c.tab-meta.xml":                   "<CustomTab/>",
		"classes/ProjectController.cls":                  "public class ProjectController {}",
		"classes/ProjectController.cls-meta.xml":         "<ApexClass/>",
		"lwc/projectList/projectList.js":                 "export default class {}",
		"lwc/projectList/projectList.js-meta.xml":        "<LightningComponentBundle/>",
		"lwc/projectList/__tests__/projectList.test.js":  "test",
	})

	files, err := importFiles(root)
	if err != nil {
		t.Fatalf("importFiles failed: %v", err)
	}
	want := []string{
		"classes/ProjectController.cls",
		"classes/ProjectController.cls-meta.xml",
		"lwc/projectList/projectList.js",
		"lwc/projectList/projectList.js-meta.xml",
		"objects/Project__c.object",
		"package.xml",
		"reports/Demo_Reports-meta.xml",
		"reports/Demo_Reports/By_Status.report",
		"tabs/Project__c.tab",
	}
	if got := importFileNames(files); !reflect.DeepEqual(got, want) {
		t.Fatalf("converted files:\n got %v\nwant %v", got, want)
	}
	if string(files["package.xml"]) != importTestPackageXml {
		t.Errorf("expected the directory's package.xml to be deployed as it is, got:\n%s", files["package.xml"])
	}
	object := string(files[filepath.FromSlash("objects/Project__c.object")])
	for _, wantText := range []string{"<label>Project</label>", "<fullName>Budget__c</fullName>", "<type>Currency</type>"} {
		if !strings.Contains(object, wantText) {
			t.Errorf("expected the composed object to contain %s, got:\n%s", wantText, object)
		}
	}
}

// A directory in metadata format is read as it is, including files outside
// the directories of known metadata types.
func TestImportFiles_KeepsMetadataFormat(t *testing.T) {
	root := t.TempDir()
	fixture := map[string]string{
		"package.xml":                            importTestPackageXml,
		"objects/Project__c.object":              "<CustomObject/>",
		"reports/Demo_Reports-meta.xml":          "<ReportFolder/>",
		"reports/Demo_Reports/By_Status.report":  "<Report/>",
		"classes/ProjectController.cls":          "public class ProjectController {}",
		"classes/ProjectController.cls-meta.xml": "<ApexClass/>",
		"unknownType/Thing.thing":                "<Thing/>",
	}
	writeImportFixture(t, root, fixture)

	files, err := importFiles(root)
	if err != nil {
		t.Fatalf("importFiles failed: %v", err)
	}
	if len(files) != len(fixture) {
		t.Fatalf("expected %d files, got %v", len(fixture), importFileNames(files))
	}
	for name, content := range fixture {
		if got := string(files[filepath.FromSlash(name)]); got != content {
			t.Errorf("%s: got %q, want %q", name, got, content)
		}
	}
}

// A static resource expanded into a directory, as sf stores a zip resource,
// is imported as one zipped .resource file named in the package.xml by the
// resource's name, and the directory's other files are not deployed.
func TestImportFiles_ZipsExpandedStaticResource(t *testing.T) {
	root := t.TempDir()
	writeImportFixture(t, root, map[string]string{
		"staticresources/Leaflet.resource-meta.xml": "<StaticResource/>",
		"staticresources/Leaflet/leaflet.js":        "js",
		"staticresources/Leaflet/images/marker.png": "png",
		"classes/ProjectController.cls":             "public class ProjectController {}",
		"classes/ProjectController.cls-meta.xml":    "<ApexClass/>",
	})

	files, err := importFiles(root)
	if err != nil {
		t.Fatalf("importFiles failed: %v", err)
	}
	want := []string{
		"classes/ProjectController.cls",
		"classes/ProjectController.cls-meta.xml",
		"staticresources/Leaflet.resource",
		"staticresources/Leaflet.resource-meta.xml",
	}
	if got := importFileNames(files); !reflect.DeepEqual(got, want) {
		t.Fatalf("imported files:\n got %v\nwant %v", got, want)
	}
	data := files[filepath.FromSlash("staticresources/Leaflet.resource")]
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Leaflet.resource is not a zip: %v", err)
	}
	var entries []string
	for _, f := range reader.File {
		entries = append(entries, f.Name)
	}
	sort.Strings(entries)
	if wantEntries := []string{"images/marker.png", "leaflet.js"}; !reflect.DeepEqual(entries, wantEntries) {
		t.Errorf("zip entries = %v, want %v", entries, wantEntries)
	}
}
