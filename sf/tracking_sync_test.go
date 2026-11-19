package sf

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ForceCLI/force/lib"
)

// The scenario of these tests was run with sf 2.138.6 and with this mode
// against a scratch org, and the output and the tracking files were the
// same.

func sourceMemberRecord(memberType, name string, revision int, obsolete bool) map[string]interface{} {
	return map[string]interface{}{"MemberIdOrName": "id", "MemberType": memberType, "MemberName": name, "IsNameObsolete": obsolete, "RevisionCounter": revision, "IsNewMember": false, "ChangedBy": "005", "LastModifiedDate": "2026-11-02T10:00:00.000+0000"}
}

// trackedDeploySetup returns a project whose org tracks source, with the
// tracking files reset: the class Greeter is at revision 1 in the org.
func trackedDeploySetup(t *testing.T) (*fakeOrg, *fakeDeploy, string) {
	t.Helper()
	org, fake, project := deploySetup(t)
	if err := lib.UpdateSFDXAuthFields("me@example.com", lib.SFDXAuthFields{"tracksSource": true}); err != nil {
		t.Fatal(err)
	}
	org.handlers[sourceMemberDescribe] = func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"name":"SourceMember"}`)) }
	org.queries["RevisionCounter > 0"] = []map[string]interface{}{sourceMemberRecord("ApexClass", "Greeter", 1, false)}
	org.queries["RevisionCounter > 1"] = nil
	previous := trackingPollInterval
	trackingPollInterval = time.Millisecond
	t.Cleanup(func() { trackingPollInterval = previous })
	if _, errOut, status := run("project", "reset", "tracking", "-p"); status != 0 {
		t.Fatalf("reset: status %d %s", status, errOut)
	}
	return org, fake, project
}

// afterDeploy runs then when the fake org has received a deploy, so that a
// test can make the org report source members that the deploy changed.
func afterDeploy(org *fakeOrg, then func()) {
	metadata := org.handlers["/services/Soap/m/67.0"]
	org.handlers["/services/Soap/m/67.0"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		metadata(w, r)
		if match := soapOperationPattern.FindSubmatch(body); match != nil && string(match[1]) == "deploy" {
			then()
		}
	}
}

func TestTracksSourceFollowsTheAuthorization(t *testing.T) {
	loggedInFakeOrg(t)
	org, err := loadOrg("dev")
	if err != nil {
		t.Fatal(err)
	}
	// An org that is neither a scratch org nor a sandbox does not track
	// source, and the answer is recorded.
	if tracksSource(org) {
		t.Error("a production org tracks source")
	}
	if fields, _ := lib.ReadSFDXAuthFields("me@example.com"); fields["tracksSource"] != false {
		t.Errorf("tracksSource = %v", fields["tracksSource"])
	}
	org.Fields = lib.SFDXAuthFields{"isScratch": true}
	if !tracksSource(org) {
		t.Error("a scratch org does not track source")
	}
	org.Fields = lib.SFDXAuthFields{"isScratch": true, "tracksSource": false}
	if tracksSource(org) {
		t.Error("tracking that was disabled is on")
	}
}

func TestProjectDeployStartDeploysLocalChanges(t *testing.T) {
	org, fake, project := trackedDeploySetup(t)
	expectRun(t, 0, "No changes to deploy\n", "project", "deploy", "start")
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "status": "Nothing to deploy",
    "files": []
  },
  "warnings": []
}
`, "project", "deploy", "start", "--json")

	classes := filepath.Join(project, "force-app", "main", "default", "classes")
	os.WriteFile(filepath.Join(classes, "Greeter.cls"), []byte("public class Greeter { /* changed */ }"), 0o644)
	// The org reports the new revision of the class after the deploy.
	afterDeploy(org, func() {
		org.queries["RevisionCounter > 1"] = []map[string]interface{}{sourceMemberRecord("ApexClass", "Greeter", 2, false), sourceMemberRecord("Profile", "Admin", 3, false)}
	})
	out, errOut, status := run("project", "deploy", "start")
	if status != 0 {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	if !strings.Contains(durationPattern.ReplaceAllString(out, "T"), " ✔ Updating Source Tracking T\n   ▸ Members: 1/1 (100%)\n") {
		t.Errorf("output =\n%s", out)
	}
	// Only the changed class is deployed.
	if fake.deployed["classes/Greeter.cls"] != "public class Greeter { /* changed */ }" || fake.deployed["classes/GreeterTest.cls"] != "" || strings.Contains(fake.deployed["package.xml"], "CustomObject") {
		t.Errorf("deployed files = %v", fake.deployed)
	}
	// The class is in sync at its new revision. The profile that the org
	// changed with it is recorded and not marked as retrieved.
	_, revisionFile := trackingPaths(t)
	revisions := readFile(t, revisionFile)
	if !strings.Contains(revisions, `"ApexClass__Greeter": {
            "memberType": "ApexClass",
            "serverRevisionCounter": 2,
            "lastRetrievedFromServer": 2,`) || !strings.Contains(revisions, `"Profile__Admin": {
            "memberType": "Profile",
            "serverRevisionCounter": 3,
            "lastRetrievedFromServer": null,`) {
		t.Errorf("maxRevision.json =\n%s", revisions)
	}
	org.queries["RevisionCounter > 3"] = nil
	expectRun(t, 0, "No changes to deploy\n", "project", "deploy", "start")

	// A component whose files were deleted is deleted in the org, and its
	// files are reported relative to the project.
	os.Remove(filepath.Join(classes, "GreeterTest.cls"))
	os.Remove(filepath.Join(classes, "GreeterTest.cls-meta.xml"))
	fake.statusResult = strings.Replace(deploySucceeded, `<created>true</created><createdDate>2026-01-02T03:04:06.000Z</createdDate><deleted>false</deleted><fileName>classes/Greeter.cls</fileName><fullName>Greeter</fullName>`,
		`<created>false</created><createdDate>2026-01-02T03:04:06.000Z</createdDate><deleted>true</deleted><fileName>classes/GreeterTest.cls</fileName><fullName>GreeterTest</fullName>`, 1)
	afterDeploy(org, func() {
		org.queries["RevisionCounter > 3"] = []map[string]interface{}{sourceMemberRecord("ApexClass", "GreeterTest", 4, true)}
	})
	out, errOut, status = run("project", "deploy", "start", "--json")
	if status != 0 {
		t.Fatalf("delete: status %d, stderr %s\n%s", status, errOut, out)
	}
	if manifest := fake.deployed["destructiveChangesPost.xml"]; !strings.Contains(manifest, "<members>GreeterTest</members>") || !strings.Contains(manifest, "<name>ApexClass</name>") {
		t.Errorf("destructiveChangesPost.xml = %s", manifest)
	}
	if fake.deployed["classes/Greeter.cls"] != "" {
		t.Errorf("an unchanged class was deployed with the deletion: %v", fake.deployed)
	}
	for _, want := range []string{`"state": "Deleted",
        "filePath": "force-app/main/default/classes/GreeterTest.cls"`, `"filePath": "force-app/main/default/classes/GreeterTest.cls-meta.xml"`} {
		if !strings.Contains(out, want) {
			t.Errorf("json lacks %s:\n%s", want, out)
		}
	}
	org.queries["RevisionCounter > 4"] = nil
	expectRun(t, 0, "No changes to deploy\n", "project", "deploy", "start")
}

func TestDeployAndRetrieveStopOnConflicts(t *testing.T) {
	org, fake, project := trackedDeploySetup(t)
	class := filepath.Join(project, "force-app", "main", "default", "classes", "Greeter.cls")
	os.WriteFile(class, []byte("public class Greeter { /* local */ }"), 0o644)
	org.queries["RevisionCounter > 1"] = []map[string]interface{}{sourceMemberRecord("ApexClass", "Greeter", 2, false)}
	org.queries["RevisionCounter > 2"] = nil

	out, errOut, status := run("project", "deploy", "start")
	if status != 1 || !strings.Contains(out, "│ Conflict │ Greeter   │ ApexClass │ "+class+"-meta.xml │") || !strings.Contains(out, "│ STATE    │ FULL NAME │ TYPE      │ FILE PATH") ||
		errOut != "Error (1): There are changes in the org that conflict with the local changes you're trying to deploy.\n\n\nTry this:\n\nTo overwrite the remote changes, rerun this command with the --ignore-conflicts flag.\nTo overwrite the local changes, run the \"sf project retrieve start\" command with the --ignore-conflicts flag.\n\n" {
		t.Errorf("deploy: status %d\n%s\nstderr %q", status, out, errOut)
	}
	if len(fake.deployed) != 0 {
		t.Errorf("a deploy with conflicts was sent: %v", fake.deployed)
	}
	out, _, status = run("project", "deploy", "start", "--json")
	for _, want := range []string{`"name": "SourceConflictError"`, `"message": "2 conflicts detected"`, `"context": "DeployMetadata"`, `"state": "Conflict",
      "fullName": "Greeter",
      "type": "ApexClass",
      "filePath": "` + class + `-meta.xml"`} {
		if status != 1 || !strings.Contains(out, want) {
			t.Errorf("deploy json lacks %s: status %d\n%s", want, status, out)
		}
	}

	out, errOut, status = run("project", "retrieve", "start")
	if status != 1 || !strings.Contains(out, " ✘ Sending request to org ") || !strings.Contains(out, " Status: Failed\n") || !strings.Contains(out, "│ Conflict │ Greeter ") ||
		!strings.HasPrefix(errOut, "Error (1): There are changes in your local files that conflict with the org changes you're trying to retrieve.\n\n\nTry this:\n\nTo overwrite the local changes, rerun this command with the --ignore-conflicts flag.\n") {
		t.Errorf("retrieve: status %d\n%s\nstderr %q", status, out, errOut)
	}

	// With --ignore-conflicts the retrieve replaces the local change.
	fake.retrieveZip = retrieveZip(t, map[string]string{
		"classes/Greeter.cls":          "public class Greeter { /* org */ }",
		"classes/Greeter.cls-meta.xml": greeterMeta,
		"package.xml":                  "<Package/>",
	})
	out, errOut, status = run("project", "retrieve", "start", "-c")
	if status != 0 || !strings.Contains(out, "│ Changed │ Greeter │ ApexClass │ force-app/main/default/classes/Greeter.cls          │\n│ Changed │ Greeter │ ApexClass │ force-app/main/default/classes/Greeter.cls-meta.xml │") {
		t.Fatalf("retrieve -c: status %d, stderr %s\n%s", status, errOut, out)
	}
	if !strings.Contains(fake.options, "<members>Greeter</members>") {
		t.Errorf("retrieve request = %s", fake.options)
	}
	if got := readFile(t, class); got != "public class Greeter { /* org */ }" {
		t.Errorf("class = %q", got)
	}
	_, revisionFile := trackingPaths(t)
	if revisions := readFile(t, revisionFile); !strings.Contains(revisions, `"serverRevisionCounter": 2,
            "lastRetrievedFromServer": 2,`) {
		t.Errorf("maxRevision.json =\n%s", revisions)
	}
	// Local files and org are in sync again.
	expectRun(t, 0, "No changes to deploy\n", "project", "deploy", "start")
	out, _, status = run("project", "retrieve", "start")
	if status != 0 || !strings.HasSuffix(out, "\nNothing retrieved\n") || !strings.Contains(out, " ◼ Waiting for the org to respond\n") {
		t.Errorf("nothing to retrieve: status %d\n%s", status, out)
	}
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "files": []
  },
  "warnings": []
}
`, "project", "retrieve", "start", "--json")
}

func TestProjectRetrieveStartAppliesRemoteChanges(t *testing.T) {
	org, fake, project := trackedDeploySetup(t)
	classes := filepath.Join(project, "force-app", "main", "default", "classes")
	// In the org, Remote is new and GreeterTest was deleted.
	org.queries["RevisionCounter > 1"] = []map[string]interface{}{sourceMemberRecord("ApexClass", "Remote", 2, false), sourceMemberRecord("ApexClass", "GreeterTest", 3, true)}
	org.queries["RevisionCounter > 3"] = nil
	fake.retrieveZip = retrieveZip(t, map[string]string{
		"classes/Remote.cls":          "public class Remote {}",
		"classes/Remote.cls-meta.xml": greeterMeta,
		"package.xml":                 "<Package/>",
	})
	out, errOut, status := run("project", "retrieve", "start")
	if status != 0 {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	for _, want := range []string{"│ Created │ Remote      │ ApexClass │ force-app/main/default/classes/Remote.cls ", "│ Deleted │ GreeterTest │ ApexClass │ force-app/main/default/classes/GreeterTest.cls "} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(fake.options, "<members>Remote</members>") || strings.Contains(fake.options, "GreeterTest") {
		t.Errorf("retrieve request = %s", fake.options)
	}
	if _, err := os.Stat(filepath.Join(classes, "GreeterTest.cls")); !os.IsNotExist(err) {
		t.Error("the class deleted in the org still exists locally")
	}
	if got := readFile(t, filepath.Join(classes, "Remote.cls")); got != "public class Remote {}" {
		t.Errorf("retrieved class = %q", got)
	}
	// Both members are in sync, and the retrieved and deleted files are
	// not local changes.
	_, revisionFile := trackingPaths(t)
	revisions := readFile(t, revisionFile)
	if strings.Contains(revisions, `"lastRetrievedFromServer": null`) || !strings.Contains(revisions, `"serverMaxRevisionCounter": 3`) {
		t.Errorf("maxRevision.json =\n%s", revisions)
	}
	expectRun(t, 0, "No changes to deploy\n", "project", "deploy", "start")
}

func TestProjectDeployStartWarnsWhenTheOrgDoesNotReportSourceMembers(t *testing.T) {
	_, _, project := trackedDeploySetup(t)
	t.Setenv("SF_SOURCE_MEMBER_POLLING_TIMEOUT", "1")
	trackingPollInterval = 400 * time.Millisecond
	classes := filepath.Join(project, "force-app", "main", "default", "classes")
	os.WriteFile(filepath.Join(classes, "Greeter.cls"), []byte("public class Greeter { /* changed */ }"), 0o644)
	// The org reports no new revision after the deploy.
	out, errOut, status := run("project", "deploy", "start", "--json")
	if status != 0 {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	if !strings.Contains(out, `  "warnings": [
    "Polling for 1 SourceMembers timed out after 3 attempts (last 3 were empty).\n\nMissing SourceMembers:\n  - ApexClass: Greeter"
  ]`) {
		t.Errorf("json =\n%s", out)
	}

	os.WriteFile(filepath.Join(classes, "Greeter.cls"), []byte("public class Greeter { /* again */ }"), 0o644)
	out, errOut, status = run("project", "deploy", "start")
	if status != 0 || errOut != "Warning: Polling for 1 SourceMembers timed out after 3 attempts (last 3 were empty).\n\nMissing SourceMembers:\n  - ApexClass: Greeter\n" ||
		!strings.Contains(out, "   ▸ Members: 0/1 (0%)\n") {
		t.Errorf("status %d, stderr %q\n%s", status, errOut, out)
	}

	// With polling disabled the org is asked once and there is no warning.
	t.Setenv("SF_DISABLE_SOURCE_MEMBER_POLLING", "true")
	os.WriteFile(filepath.Join(classes, "Greeter.cls"), []byte("public class Greeter { /* third */ }"), 0o644)
	if _, errOut, status = run("project", "deploy", "start"); status != 0 || errOut != "" {
		t.Errorf("polling disabled: status %d, stderr %q", status, errOut)
	}
}

func TestExpectsSourceMemberFollowsTheRulesOfSf(t *testing.T) {
	for key, want := range map[string]bool{
		"ApexClass###Greeter":                           true,
		"CustomObject###Invoice__c":                     false,
		"CustomField###Invoice__c.Amount__c":            true,
		"CustomField###Account.Name":                    false,
		"CustomField###Invoice__c.Old_del__c":           false,
		"AccountSettings###Account":                     false,
		"WorkflowAlert###Case.Notify":                   false,
		"Profile###Admin":                               true,
		"Profile###Standard":                            false,
		"Profile###Custom: Sales Profile":               false,
		"Layout###Account-Account Layout":               true,
		"Layout###Account-Account (Marketing) Layout":   false,
		"CustomMetadata###Setting.One":                  true,
		"CustomMetadata###ns__Setting.One":              false,
		"NavigationMenu###SFDC_Default_Navigation_Site": false,
	} {
		if got := expectsSourceMember(key); got != want {
			t.Errorf("expectsSourceMember(%q) = %v, want %v", key, got, want)
		}
	}
}
