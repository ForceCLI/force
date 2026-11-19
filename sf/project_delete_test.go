package sf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// deployDeleted is the deploy result of an org that deleted the class named
// by the test.
func deployDeleted(name string) string {
	return strings.Replace(deploySucceeded, `<created>true</created><createdDate>2026-01-02T03:04:06.000Z</createdDate><deleted>false</deleted><fileName>classes/Greeter.cls</fileName><fullName>Greeter</fullName>`,
		`<created>false</created><createdDate>2026-01-02T03:04:06.000Z</createdDate><deleted>true</deleted><fileName>classes/`+name+`.cls</fileName><fullName>`+name+`</fullName>`, 1)
}

const deleteFailed = `<checkOnly>false</checkOnly>` +
	`<details><componentFailures><changed>false</changed><componentType>ApexClass</componentType><created>false</created><deleted>false</deleted><fileName>destructiveChanges.xml</fileName><fullName>destructiveChanges.xml</fullName><problem>No ApexClass named: Greeter found</problem><problemType>Warning</problemType><success>false</success></componentFailures></details>` +
	`<done>true</done><id>0Af000000000001AAA</id><numberComponentErrors>0</numberComponentErrors><numberComponentsDeployed>0</numberComponentsDeployed><numberComponentsTotal>0</numberComponentsTotal>` +
	`<numberTestErrors>0</numberTestErrors><numberTestsCompleted>0</numberTestsCompleted><numberTestsTotal>0</numberTestsTotal><status>Failed</status><success>false</success>`

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestProjectDeleteSourceChecksItsFlags(t *testing.T) {
	deploySetup(t)
	for _, c := range []struct {
		args   []string
		status int
		want   string
	}{
		{nil, 2, "Error (2): The following error occurred:\n  Exactly one of the following must be provided: --metadata, --source-dir\nSee more help with --help\n\n\n"},
		{[]string{"-m", "ApexClass:Greeter", "-p", "force-app"}, 2, "Error (2): The following errors occurred:\n  --metadata cannot also be provided when using --source-dir\n  --source-dir cannot also be provided when using --metadata\nSee more help with --help\n\n\n"},
		{[]string{"-m", "ApexClass:Greeter", "-f", "-r"}, 2, "Error (2): The following error occurred:\n  All of the following must be provided when using --force-overwrite: --track-source\nSee more help with --help\n\n\n"},
		{[]string{"-m", "ApexClass:Greeter", "-t", "-c", "-r"}, 2, "Error (2): The following error occurred:\n  --check-only=true cannot also be provided when using --track-source\nSee more help with --help\n\n\n"},
		{[]string{"-m", "ApexClass:Greeter", "-r", "--test-level", "RunSpecifiedTests"}, 1, "Error (NoTestsSpecifiedError): You must specify tests using the --tests flag if the --test-level flag is set to RunSpecifiedTests.\n\n\n"},
		{[]string{"-p", "force-app/nope", "-r"}, 1, "Error (SfError): force-app/nope: File or folder not found\n\n\n"},
	} {
		out, errOut, status := run(append([]string{"project", "delete", "source"}, c.args...)...)
		if status != c.status || errOut != c.want || out != "" {
			t.Errorf("%v: status %d, stdout %q, stderr %q", c.args, status, out, errOut)
		}
	}
}

func TestProjectDeleteSourceDeletesInTheOrgAndLocally(t *testing.T) {
	_, fake, project := deploySetup(t)
	fake.statusResult = deployDeleted("Greeter")
	classes := filepath.Join(project, "force-app", "main", "default", "classes")
	out, errOut, status := run("project", "delete", "source", "-m", "ApexClass:Greeter", "-r")
	if status != 0 || errOut != "" {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	want := `
 ─────────────── Deleting Metadata ───────────────

 Deleting with SOAP API

 ✔ Preparing T
 ◯ Waiting for the org to respond - Skipped
 ✔ Deploying Metadata T
   ▸ Components: 1/1 (100%)
 ◯ Running Tests - Skipped
 ◯ Updating Source Tracking - Skipped
 ✔ Done T

 Status: Succeeded
 Deploy ID: 0Af000000000001AAA
 Target Org: me@example.com
 Elapsed Time: T



=== Deleted Source

┌───────────┬───────────┬─────────────────────────────────────────────────────┐
│ FULL NAME │ TYPE      │ PROJECT PATH                                        │
├───────────┼───────────┼─────────────────────────────────────────────────────┤
│ Greeter   │ ApexClass │ force-app/main/default/classes/Greeter.cls          │
│ Greeter   │ ApexClass │ force-app/main/default/classes/Greeter.cls-meta.xml │
└───────────┴───────────┴─────────────────────────────────────────────────────┘

`
	if got := durationPattern.ReplaceAllString(out, "T"); got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}
	// The class is named in the manifest of deletions and nothing is
	// deployed with it.
	if manifest := fake.deployed["destructiveChangesPost.xml"]; !strings.Contains(manifest, "<members>Greeter</members>") || !strings.Contains(manifest, "<name>ApexClass</name>") {
		t.Errorf("destructiveChangesPost.xml = %s", manifest)
	}
	if strings.Contains(fake.deployed["package.xml"], "<types>") || len(fake.deployed) != 2 {
		t.Errorf("deployed files = %v", fake.deployed)
	}
	if exists(filepath.Join(classes, "Greeter.cls")) || exists(filepath.Join(classes, "Greeter.cls-meta.xml")) {
		t.Error("the files of the deleted class are still there")
	}
	if !exists(filepath.Join(classes, "GreeterTest.cls")) {
		t.Error("the files of another class were deleted")
	}
}

func TestProjectDeleteSourceReportsJSON(t *testing.T) {
	_, fake, project := deploySetup(t)
	fake.statusResult = deployDeleted("GreeterTest")
	classes := filepath.Join(project, "force-app", "main", "default", "classes")
	out, errOut, status := run("project", "delete", "source", "-p", "force-app/main/default/classes/GreeterTest.cls", "-r", "--json")
	if status != 0 {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	for _, want := range []string{`    "success": true,
    "deletedSource": [
      {
        "fullName": "GreeterTest",
        "type": "ApexClass",
        "state": "Deleted",
        "filePath": "` + filepath.Join(classes, "GreeterTest.cls") + `"
      },
      {
        "fullName": "GreeterTest",
        "type": "ApexClass",
        "state": "Deleted",
        "filePath": "` + filepath.Join(classes, "GreeterTest.cls-meta.xml") + `"
      }
    ],
    "outboundFiles": [],
    "deployedSource": [],
    "deletes": [
      {
        "checkOnly": false,`, `          "runTestResult": {
            "numFailures": 0,
            "numTestsRun": 0,
            "totalTime": 0,
            "codeCoverage": [],`} {
		if !strings.Contains(out, want) {
			t.Errorf("json lacks %s:\n%s", want, out)
		}
	}
	if exists(filepath.Join(classes, "GreeterTest.cls")) || !exists(filepath.Join(classes, "Greeter.cls")) {
		t.Error("the wrong files were deleted")
	}
}

func TestProjectDeleteSourceKeepsFilesWhenTheOrgRefuses(t *testing.T) {
	_, fake, project := deploySetup(t)
	fake.statusResult = deleteFailed
	class := filepath.Join(project, "force-app", "main", "default", "classes", "Greeter.cls")
	out, errOut, status := run("project", "delete", "source", "-m", "ApexClass:Greeter", "-r")
	if status != 1 || errOut != "" {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	for _, want := range []string{" ✘ Deploying Metadata ", "   ▸ Components: ✘\n ◼ Running Tests\n ◼ Updating Source Tracking\n ◼ Done\n", `


Component Failures [1]
┌─────────┬────────────────────────┬───────────────────────────────────┐
│ Type    │ Name                   │ Problem                           │
├─────────┼────────────────────────┼───────────────────────────────────┤
│ Warning │ destructiveChanges.xml │ No ApexClass named: Greeter found │
└─────────┴────────────────────────┴───────────────────────────────────┘
`} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if !exists(class) {
		t.Error("the files of a class that was not deleted in the org were deleted")
	}
	// The files of a component that the org did not report are listed
	// with the path before the state.
	out, _, status = run("project", "delete", "source", "-m", "ApexClass:Greeter", "-r", "--json")
	if status != 1 || !strings.Contains(out, `"filePath": "`+class+`",
        "state": "Deleted"`) || !strings.Contains(out, "\"status\": 1,\n") {
		t.Errorf("status %d\n%s", status, out)
	}
}

func TestProjectDeleteSourceCheckOnlyKeepsFiles(t *testing.T) {
	_, fake, project := deploySetup(t)
	fake.statusResult = deployDeleted("Greeter")
	out, errOut, status := run("project", "delete", "source", "-m", "ApexClass:Greeter", "-r", "-c")
	if status != 0 || !strings.Contains(out, "=== Deleted Source") {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	if !strings.Contains(fake.options, "<checkOnly>true</checkOnly>") && !strings.Contains(fake.options, "checkOnly>true<") {
		t.Errorf("deploy options = %s", fake.options)
	}
	if !exists(filepath.Join(project, "force-app", "main", "default", "classes", "Greeter.cls")) {
		t.Error("a check deleted the files")
	}
}

func TestProjectDeleteSourceAsksBeforeDeleting(t *testing.T) {
	_, fake, project := deploySetup(t)
	classes := filepath.Join(project, "force-app", "main", "default", "classes")
	// The test has no input, which is not a yes.
	out, errOut, status := run("project", "delete", "source", "-m", "ApexClass:Greeter", "-m", "ApexClass:OnlyInOrg")
	want := "? This operation will delete the following metadata in your org:\nApexClass:OnlyInOrg\n\n\n" +
		"This operation will delete the following files on your computer and in your org:\n" +
		filepath.Join(classes, "Greeter.cls-meta.xml") + "\n" + filepath.Join(classes, "Greeter.cls") + "\n" +
		"Are you sure you want to proceed? (y/N) "
	if status != 1 || out != want || errOut != "Error (Prompt.delete.cancelError): The request to delete metadata was canceled.\n\n\n" {
		t.Errorf("status %d, stdout %q, stderr %q", status, out, errOut)
	}
	if len(fake.deployed) != 0 || !exists(filepath.Join(classes, "Greeter.cls")) {
		t.Error("a delete that was not confirmed was carried out")
	}
}

func TestProjectDeleteSourceListsComponentsThatAreOnlyInTheOrg(t *testing.T) {
	_, fake, _ := deploySetup(t)
	fake.statusResult = deployDeleted("OnlyInOrg")
	out, errOut, status := run("project", "delete", "source", "-m", "ApexClass:OnlyInOrg", "-r")
	if status != 0 || !strings.Contains(out, `=== Deleted Source

┌───────────┬───────────┬──────────────┐
│ FULL NAME │ TYPE      │ PROJECT PATH │
├───────────┼───────────┼──────────────┤
│ OnlyInOrg │ ApexClass │              │
└───────────┴───────────┴──────────────┘
`) {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	if !strings.Contains(fake.deployed["destructiveChangesPost.xml"], "<members>OnlyInOrg</members>") {
		t.Errorf("destructiveChangesPost.xml = %s", fake.deployed["destructiveChangesPost.xml"])
	}
	out, _, status = run("project", "delete", "source", "-m", "ApexClass:OnlyInOrg", "-r", "--json")
	if status != 0 || !strings.Contains(out, `"deletedSource": [],`) {
		t.Errorf("status %d\n%s", status, out)
	}
}

func TestProjectDeleteSourceRemovesTheDirectoryOfAnObject(t *testing.T) {
	_, fake, project := deploySetup(t)
	fake.statusResult = deploySucceeded
	out, errOut, status := run("project", "delete", "source", "-m", "CustomObject:Invoice__c", "-r")
	if status != 0 {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	objects := filepath.Join(project, "force-app", "main", "default", "objects")
	if exists(filepath.Join(objects, "Invoice__c")) || !exists(objects) {
		t.Error("the directory of the object was not removed, or the objects directory was")
	}
	if !strings.Contains(out, "force-app/main/default/objects/Invoice__c/fields/Amount__c.field-meta.xml") {
		t.Errorf("output lacks the field file:\n%s", out)
	}
}

func TestProjectDeleteSourceUpdatesSourceTracking(t *testing.T) {
	org, fake, project := trackedDeploySetup(t)
	class := filepath.Join(project, "force-app", "main", "default", "classes", "Greeter.cls")

	// A class changed both locally and in the org is not deleted.
	os.WriteFile(class, []byte("public class Greeter { /* local */ }"), 0o644)
	org.queries["RevisionCounter > 1"] = []map[string]interface{}{sourceMemberRecord("ApexClass", "Greeter", 2, false)}
	org.queries["RevisionCounter > 2"] = nil
	out, errOut, status := run("project", "delete", "source", "-m", "ApexClass:Greeter", "-r", "-t")
	if status != 1 || !strings.Contains(out, "│ Conflict │ Greeter   │ ApexClass │ "+class+"-meta.xml │") ||
		errOut != "Error (sourceConflictDetected): We couldn't complete the operation due to conflicts. Verify that you want to keep the existing versions, then run the command again with the --force-overwrite (-f) option.\n\n\n" {
		t.Errorf("status %d\n%s\nstderr %q", status, out, errOut)
	}
	// sf prints the table before the JSON error too.
	out, _, status = run("project", "delete", "source", "-m", "ApexClass:Greeter", "-r", "-t", "--json")
	if status != 1 || !strings.HasPrefix(out, "┌──────────┬") || !strings.Contains(out, `"name": "sourceConflictDetected"`) || !strings.Contains(out, `"state": "Conflict",`) {
		t.Errorf("json: status %d\n%s", status, out)
	}
	if len(fake.deployed) != 0 || !exists(class) {
		t.Fatal("a delete with conflicts was carried out")
	}

	fake.statusResult = deployDeleted("Greeter")
	afterDeploy(org, func() {
		org.queries["RevisionCounter > 2"] = []map[string]interface{}{sourceMemberRecord("ApexClass", "Greeter", 3, true)}
	})
	out, errOut, status = run("project", "delete", "source", "-m", "ApexClass:Greeter", "-r", "-t", "-f")
	if status != 0 || errOut != "Updating source tracking... done\n" || !strings.Contains(out, " ◯ Updating Source Tracking - Skipped\n") {
		t.Fatalf("status %d, stderr %q\n%s", status, errOut, out)
	}
	if exists(class) {
		t.Error("the class file is still there")
	}
	_, revisionFile := trackingPaths(t)
	if revisions := readFile(t, revisionFile); !strings.Contains(revisions, `"ApexClass__Greeter": {
            "memberType": "ApexClass",
            "serverRevisionCounter": 3,
            "lastRetrievedFromServer": 3,
            "isNameObsolete": true`) {
		t.Errorf("maxRevision.json =\n%s", revisions)
	}
	// Neither side has a change left.
	org.queries["RevisionCounter > 3"] = nil
	expectRun(t, 0, "No changes to deploy\n", "project", "deploy", "start")
}
