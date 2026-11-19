package sf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The scenarios in these tests were run with sf 2.138.6, and its output was
// the same.

// previewProject resets tracking for the project of trackingProject and
// then changes it: Greeter is modified, Added and Ignored are new, Ignored
// is excluded by .forceignore, GreeterTest is deleted, and a custom object
// with a field is new.
func previewProject(t *testing.T) *fakeOrg {
	t.Helper()
	org := trackingProject(t)
	if _, errOut, status := run("project", "reset", "tracking", "-o", "dev", "-p"); status != 0 {
		t.Fatalf("reset: status %d %s", status, errOut)
	}
	org.queries["RevisionCounter > 5"] = nil
	classes := filepath.Join("force-app", "main", "default", "classes")
	write := func(path, content string) {
		t.Helper()
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(classes, "Greeter.cls"), "public class Greeter { /* changed */ }\n")
	for _, name := range []string{"Added", "Ignored"} {
		write(filepath.Join(classes, name+".cls"), "public class "+name+" {}\n")
		write(filepath.Join(classes, name+".cls-meta.xml"), "<ApexClass/>\n")
	}
	write(".forceignore", "**/Ignored.cls*\n")
	os.Remove(filepath.Join(classes, "GreeterTest.cls"))
	os.Remove(filepath.Join(classes, "GreeterTest.cls-meta.xml"))
	object := filepath.Join("force-app", "main", "default", "objects", "Thing__c")
	write(filepath.Join(object, "Thing__c.object-meta.xml"), "<CustomObject/>\n")
	write(filepath.Join(object, "fields", "Size__c.field-meta.xml"), "<CustomField/>\n")
	return org
}

func TestComponentOfSourceFile(t *testing.T) {
	for path, want := range map[string]string{
		"force-app/main/default/classes/Greeter.cls":                              "ApexClass Greeter",
		"force-app/main/default/classes/Greeter.cls-meta.xml":                     "ApexClass Greeter",
		"force-app/main/default/objects/Thing__c/Thing__c.object-meta.xml":        "CustomObject Thing__c",
		"force-app/main/default/objects/Thing__c/fields/Size__c.field-meta.xml":   "CustomField Thing__c.Size__c",
		"force-app/main/default/objects/Thing__c/listViews/All.listView-meta.xml": "ListView Thing__c.All",
		"force-app/main/default/lwc/hello/hello.js":                               "LightningComponentBundle hello",
		"force-app/main/default/lwc/hello/hello.js-meta.xml":                      "LightningComponentBundle hello",
		"force-app/main/default/triggers/T.trigger":                               "ApexTrigger T",
		"force-app/README.md": " ",
	} {
		metadataType, fullName := componentOfSourceFile(path)
		if got := metadataType + " " + fullName; got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
	components := sourceComponents([]string{"force-app/main/default/lwc/hello/hello.js", "force-app/main/default/lwc/hello/hello.js-meta.xml", "force-app/main/default/lwc/hello/hello.html"})
	if bundle := components["LightningComponentBundle###hello"]; bundle == nil || bundle.xml != "force-app/main/default/lwc/hello/hello.js-meta.xml" || len(bundle.files) != 3 {
		t.Errorf("bundle = %+v", bundle)
	}
}

func TestProjectDeployPreviewListsLocalChanges(t *testing.T) {
	previewProject(t)
	root, _ := os.Getwd()
	want := `
No conflicts found.

Will Delete [1] files.
┌───────────┬─────────────┬─────────────────────────────────────────────────────────┐
│ Type      │ Full Name   │ Path                                                    │
├───────────┼─────────────┼─────────────────────────────────────────────────────────┤
│ ApexClass │ GreeterTest │ force-app/main/default/classes/GreeterTest.cls-meta.xml │
└───────────┴─────────────┴─────────────────────────────────────────────────────────┘


Will Deploy [4] files.
┌──────────────┬──────────────────┬───────────────────────────────────────────────────────────────────────┐
│ Type         │ Full Name        │ Path                                                                  │
├──────────────┼──────────────────┼───────────────────────────────────────────────────────────────────────┤
│ ApexClass    │ Added            │ force-app/main/default/classes/Added.cls-meta.xml                     │
│ ApexClass    │ Greeter          │ force-app/main/default/classes/Greeter.cls-meta.xml                   │
│ CustomObject │ Thing__c         │ force-app/main/default/objects/Thing__c/Thing__c.object-meta.xml      │
│ CustomField  │ Thing__c.Size__c │ force-app/main/default/objects/Thing__c/fields/Size__c.field-meta.xml │
└──────────────┴──────────────────┴───────────────────────────────────────────────────────────────────────┘


Ignored [1] files. These files won't deploy because they're ignored by your
.forceignore file.
┌───────────┬───────────┬─────────────────────────────────────────────────────┐
│ Type      │ Full Name │ Path                                                │
├───────────┼───────────┼─────────────────────────────────────────────────────┤
│ ApexClass │ Ignored   │ force-app/main/default/classes/Ignored.cls-meta.xml │
└───────────┴───────────┴─────────────────────────────────────────────────────┘

`
	expectRun(t, 0, want, "project", "deploy", "preview", "-o", "dev")
	out, _, _ := run("project", "deploy", "preview", "-o", "dev", "--concise")
	if strings.Contains(out, "Ignored") || !strings.Contains(out, "Will Deploy [4] files.") {
		t.Errorf("--concise =\n%s", out)
	}
	out, _, _ = run("project", "deploy", "preview", "-o", "dev", "--json")
	for _, want := range []string{
		`    "ignored": [
      {
        "fullName": "Ignored",
        "type": "ApexClass",
        "path": "` + root + `/force-app/main/default/classes/Ignored.cls-meta.xml",
        "projectRelativePath": "force-app/main/default/classes/Ignored.cls-meta.xml",
        "conflict": false,
        "ignored": true
      }
    ],
    "toDeploy": [
      {
        "type": "ApexClass",
        "fullName": "Added",
        "conflict": false,
        "ignored": false,
        "path": "` + root + `/force-app/main/default/classes/Added.cls-meta.xml",
        "projectRelativePath": "force-app/main/default/classes/Added.cls-meta.xml",
        "operation": "deploy"
      },`,
		`    "toRetrieve": [],
    "toDelete": [
      {
        "type": "ApexClass",
        "fullName": "GreeterTest",
        "conflict": false,
        "ignored": false,
        "path": "` + root + `/force-app/main/default/classes/GreeterTest.cls-meta.xml",
        "projectRelativePath": "force-app/main/default/classes/GreeterTest.cls-meta.xml",
        "operation": "deletePost"
      }
    ],
    "conflicts": []`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("json lacks\n%s\nin\n%s", want, out)
		}
	}
}

func TestPreviewsReportConflictsAndRemoteChanges(t *testing.T) {
	org := previewProject(t)
	member := func(memberType, name string, revision int, obsolete bool) map[string]interface{} {
		return map[string]interface{}{"MemberIdOrName": "id", "MemberType": memberType, "MemberName": name, "IsNameObsolete": obsolete, "RevisionCounter": revision, "IsNewMember": false, "ChangedBy": "005", "LastModifiedDate": "2026-11-02T10:00:00.000+0000"}
	}
	// Greeter changed in the org and locally. OnlyInOrg has no local
	// file. GreeterTest was deleted in the org and locally, and GoneRemote
	// was deleted in the org and never retrieved.
	org.queries["RevisionCounter > 5"] = []map[string]interface{}{
		member("ApexClass", "Greeter", 6, false), member("ApexClass", "NewRemote", 7, false), member("ApexClass", "GoneRemote", 8, true),
		member("CustomField", "Invoice__c.Amount__c", 9, false),
	}
	out, _, status := run("project", "deploy", "preview", "-o", "dev", "--concise")
	if status != 0 || !strings.HasPrefix(out, `
Conflicts [1]. Run the command with the --ignore-conflicts flag to override.
┌───────────┬───────────┬─────────────────────────────────────────────────────┐
│ Type      │ Full Name │ Path                                                │
├───────────┼───────────┼─────────────────────────────────────────────────────┤
│ ApexClass │ Greeter   │ force-app/main/default/classes/Greeter.cls-meta.xml │
└───────────┴───────────┴─────────────────────────────────────────────────────┘
`) || !strings.Contains(out, "Will Deploy [3] files.") {
		t.Errorf("deploy preview with a conflict: status %d\n%s", status, out)
	}
	// The members were recorded without marking them as retrieved.
	_, revisionFile := trackingPaths(t)
	if got := readFile(t, revisionFile); !strings.Contains(got, `"serverMaxRevisionCounter": 9`) || !strings.Contains(got, `"ApexClass__NewRemote": {
            "memberType": "ApexClass",
            "serverRevisionCounter": 7,
            "lastRetrievedFromServer": null,`) || !strings.Contains(got, `"serverRevisionCounter": 6,
            "lastRetrievedFromServer": 1,`) {
		t.Errorf("maxRevision.json =\n%s", got)
	}
	org.queries["RevisionCounter > 9"] = nil

	out, _, _ = run("project", "deploy", "preview", "-o", "dev", "-c", "--concise")
	if !strings.Contains(out, "No conflicts found.") || !strings.Contains(out, "Will Deploy [4] files.") {
		t.Errorf("--ignore-conflicts =\n%s", out)
	}

	want := `
Conflicts [1]. Run the command with the --ignore-conflicts flag to override.
┌───────────┬───────────┬─────────────────────────────────────────────────────┐
│ Type      │ Full Name │ Path                                                │
├───────────┼───────────┼─────────────────────────────────────────────────────┤
│ ApexClass │ Greeter   │ force-app/main/default/classes/Greeter.cls-meta.xml │
└───────────┴───────────┴─────────────────────────────────────────────────────┘


No files will be deleted.

Will Retrieve [2] files.
┌─────────────┬──────────────────────┬──────┐
│ Type        │ Full Name            │ Path │
├─────────────┼──────────────────────┼──────┤
│ ApexClass   │ NewRemote            │      │
│ CustomField │ Invoice__c.Amount__c │      │
└─────────────┴──────────────────────┴──────┘


No files were ignored. Update your .forceignore file if you want to ignore certain files.
`
	expectRun(t, 0, want, "project", "retrieve", "preview", "-o", "dev")
	out, _, _ = run("project", "retrieve", "preview", "-o", "dev", "-c", "--json")
	for _, want := range []string{
		`"fullName": "Greeter",
        "conflict": false,
        "ignored": false,`,
		`      {
        "type": "ApexClass",
        "fullName": "NewRemote",
        "conflict": false,
        "operation": "retrieve",
        "ignored": false
      },`,
		`"conflicts": []`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("retrieve preview json lacks\n%s\nin\n%s", want, out)
		}
	}
}

func TestProjectRetrievePreviewListsRemoteDeletesOfLocalComponents(t *testing.T) {
	org := trackingProject(t)
	run("project", "reset", "tracking", "-o", "dev", "-p")
	org.queries["RevisionCounter > 5"] = []map[string]interface{}{
		{"MemberIdOrName": "id", "MemberType": "ApexClass", "MemberName": "GreeterTest", "IsNameObsolete": true, "RevisionCounter": 6, "IsNewMember": false, "ChangedBy": "005", "LastModifiedDate": "x"},
	}
	out, _, status := run("project", "retrieve", "preview", "-o", "dev", "--concise")
	if status != 0 || !strings.Contains(out, "Will Delete [1] files.") || !strings.Contains(out, "│ ApexClass │ GreeterTest │ force-app/main/default/classes/GreeterTest.cls-meta.xml │") || !strings.Contains(out, "No files will be retrieved.") {
		t.Errorf("status %d\n%s", status, out)
	}
	org.queries["RevisionCounter > 6"] = nil
	out, _, _ = run("project", "retrieve", "preview", "-o", "dev", "--json")
	if !strings.Contains(out, `"operation": "deletePre"`) {
		t.Errorf("json = %s", out)
	}
}

func TestProjectDeployPreviewOfNamedComponents(t *testing.T) {
	previewProject(t)
	classes := "force-app/main/default/classes"
	out, _, status := run("project", "deploy", "preview", "-o", "dev", "-d", classes)
	if status != 0 || !strings.Contains(out, "No files will be deleted.") || !strings.Contains(out, "Will Deploy [2] files.") || !strings.Contains(out, "│ ApexClass │ Added     │") || strings.Contains(out, "Thing__c") || !strings.Contains(out, "Ignored [1] files.") {
		t.Errorf("--source-dir: status %d\n%s", status, out)
	}
	out, _, _ = run("project", "deploy", "preview", "-o", "dev", "-m", "ApexClass:Greeter", "-m", "ApexClass:Nope", "--concise")
	want := `
No conflicts found.

No files will be deleted.

Will Deploy [2] files.
┌───────────┬───────────┬─────────────────────────────────────────────────────┐
│ Type      │ Full Name │ Path                                                │
├───────────┼───────────┼─────────────────────────────────────────────────────┤
│ ApexClass │ Greeter   │ force-app/main/default/classes/Greeter.cls-meta.xml │
│ ApexClass │ Nope      │                                                     │
└───────────┴───────────┴─────────────────────────────────────────────────────┘

`
	if out != want {
		t.Errorf("--metadata =\n%s\nwant\n%s", out, want)
	}
	_, errOut, status := run("project", "deploy", "preview", "-o", "dev", "-d", "nosuch")
	if status != 1 || errOut != "Error (SfError): nosuch: File or folder not found\n\n\n" {
		t.Errorf("missing directory: status %d, stderr %q", status, errOut)
	}
}
