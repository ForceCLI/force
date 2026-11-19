package sf

import (
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ForceCLI/force/lib"
)

// The output of these commands, maxRevision.json and the tracking repository
// were compared with what sf 2.138.6 writes, and sf read the files this mode
// wrote: after a reset by this mode, "sf project deploy preview" reported no
// changes, and reported later edits to the project.

const sourceMemberDescribe = "/services/data/v67.0/tooling/sobjects/SourceMember/describe"

// trackingProject makes the temporary home directory a project with two
// classes, logs in to a fake org that has five source members, and returns
// the org.
func trackingProject(t *testing.T) *fakeOrg {
	t.Helper()
	org := loggedInFakeOrg(t)
	os.WriteFile("sfdx-project.json", []byte(`{"packageDirectories":[{"path":"force-app","default":true}]}`), 0o644)
	classes := filepath.Join("force-app", "main", "default", "classes")
	os.MkdirAll(classes, 0o755)
	for name, content := range map[string]string{"Greeter.cls": "public class Greeter {}\n", "Greeter.cls-meta.xml": "<ApexClass/>\n", "GreeterTest.cls": "@IsTest class GreeterTest {}\n", "GreeterTest.cls-meta.xml": "<ApexClass/>\n"} {
		if err := os.WriteFile(filepath.Join(classes, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	org.handlers[sourceMemberDescribe] = func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"name":"SourceMember"}`)) }
	member := func(memberType, name string, revision int, obsolete bool) map[string]interface{} {
		return map[string]interface{}{"MemberIdOrName": "id", "MemberType": memberType, "MemberName": name, "IsNameObsolete": obsolete, "RevisionCounter": revision, "IsNewMember": true, "ChangedBy": "005", "LastModifiedDate": "2026-11-01T10:00:00.000+0000"}
	}
	all := []map[string]interface{}{
		member("ApexClass", "Greeter", 1, false), member("ApexClass", "GreeterTest", 2, false), member("ApexClass", "OnlyInOrg", 3, false),
		member("CustomObject", "Invoice__c", 4, false), member("Layout", "Account-My Layout (1)", 5, true),
	}
	org.queries["RevisionCounter > 0"] = all
	org.queries["RevisionCounter <= 3"] = all[:3]
	org.queries["RevisionCounter <= 0"] = nil
	return org
}

func trackingPaths(t *testing.T) (gitDir, revisionFile string) {
	t.Helper()
	root, _ := os.Getwd()
	base := filepath.Join(root, ".sf", "orgs", "00D000000000000")
	return filepath.Join(base, "localSourceTracking"), filepath.Join(base, "maxRevision.json")
}

func TestGitObjectIDsAreThoseOfGit(t *testing.T) {
	for _, c := range []struct{ kind, data, want string }{
		{"blob", "hello\n", "ce013625030ba8dba906f756967f9e9ca394464a"},
		{"blob", "", "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
		{"tree", "", "4b825dc642cb6eb9a060e54bf8d69288fbee4904"},
	} {
		if id := gitObjectID(c.kind, []byte(c.data)); hex.EncodeToString(id[:]) != c.want {
			t.Errorf("%s %q = %x, want %s", c.kind, c.data, id, c.want)
		}
	}
}

func TestShadowRepoCommitsAndReadsTrackedFiles(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".sf", "orgs", "x", "localSourceTracking")
	repo, err := openShadowRepo(gitDir, root)
	if err != nil {
		t.Fatal(err)
	}
	if head := readFile(t, filepath.Join(gitDir, "HEAD")); head != "ref: refs/heads/main\n" {
		t.Errorf("HEAD = %q", head)
	}
	if files, err := repo.trackedFiles(); err != nil || len(files) != 0 {
		t.Errorf("a new repository tracks %v, err %v", files, err)
	}
	files := map[string][20]byte{}
	// The directory a sorts after the file a.txt and before the file a0,
	// because git compares a directory as its name followed by a slash.
	for _, name := range []string{"a/b/one.cls", "a/two.cls", "a.txt", "a0", "top.cls"} {
		id, err := repo.writeObject("blob", []byte(name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = id
	}
	first, err := repo.commit(files, "first", time.Unix(1795098713, 0))
	if err != nil {
		t.Fatal(err)
	}
	read, err := repo.trackedFiles()
	if err != nil || len(read) != len(files) {
		t.Fatalf("tracked files = %v, err %v", read, err)
	}
	for name, id := range files {
		if read[name] != id {
			t.Errorf("%s = %x, want %x", name, read[name], id)
		}
	}
	// The tree id is the one git write-tree reports for the same files.
	_, commit, _ := repo.readObject(first)
	if !strings.HasPrefix(string(commit), "tree 0e6a71475432684f69bd809f5eca9b8c8c980d91\nauthor sfdx source tracking <> 1795098713 ") || !strings.HasSuffix(string(commit), "\n\nfirst\n") {
		t.Errorf("commit =\n%s", commit)
	}

	delete(files, "a0")
	second, err := repo.commit(files, "second", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, commit, _ = repo.readObject(second)
	if !strings.Contains(string(commit), "\nparent "+hex.EncodeToString(first[:])+"\n") {
		t.Errorf("the second commit has no parent:\n%s", commit)
	}
	if read, _ := repo.trackedFiles(); len(read) != 4 {
		t.Errorf("tracked files after a removal = %v", read)
	}
	// The index lists the files in order and ends with its checksum.
	index, _ := os.ReadFile(filepath.Join(gitDir, "index"))
	if string(index[:4]) != "DIRC" || index[7] != 2 || index[11] != 4 || (len(index)-12-20)%8 != 0 {
		t.Errorf("index header = %v, length %d", index[:12], len(index))
	}
	if a, b := strings.Index(string(index), "a.txt"), strings.Index(string(index), "a/b/one.cls"); a < 0 || b < a {
		t.Errorf("index entries are not in path order")
	}
}

func TestOrgEnableAndDisableTracking(t *testing.T) {
	org := trackingProject(t)
	expectRun(t, 0, "Enabled source tracking for me@example.com.\n", "org", "enable", "tracking", "-o", "dev")
	fields, _ := lib.ReadSFDXAuthFields("me@example.com")
	if fields["tracksSource"] != true {
		t.Errorf("tracksSource = %v", fields["tracksSource"])
	}
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "tracksSource": false,
    "username": "me@example.com"
  },
  "warnings": []
}
`, "org", "disable", "tracking", "-o", "dev", "--json")
	fields, _ = lib.ReadSFDXAuthFields("me@example.com")
	if fields["tracksSource"] != false {
		t.Errorf("tracksSource = %v", fields["tracksSource"])
	}

	org.handlers[sourceMemberDescribe] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`[{"errorCode":"NOT_FOUND","message":"The requested resource does not exist"}]`))
	}
	_, errOut, status := run("org", "enable", "tracking", "-o", "dev")
	if status != 1 || !strings.HasPrefix(errOut, "Error (TrackingNotAvailableError): You can't enable source tracking on this org because the SourceMember Tooling API object isn't available") || !strings.Contains(errOut, "Try this:") {
		t.Errorf("org without SourceMember: status %d, stderr %q", status, errOut)
	}
	if fields, _ := lib.ReadSFDXAuthFields("me@example.com"); fields["tracksSource"] != false {
		t.Errorf("tracksSource changed after a failed enable: %v", fields["tracksSource"])
	}
}

func TestProjectResetTracking(t *testing.T) {
	trackingProject(t)
	gitDir, revisionFile := trackingPaths(t)
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "sourceMembersSynced": 5,
    "localPathsSynced": 4
  },
  "warnings": []
}
`, "project", "reset", "tracking", "-o", "dev", "-p", "--json")
	want := `{
    "fileVersion": 0,
    "serverMaxRevisionCounter": 5,
    "sourceMembers": {
        "ApexClass__Greeter": {
            "memberType": "ApexClass",
            "serverRevisionCounter": 1,
            "lastRetrievedFromServer": 1,
            "isNameObsolete": false
        },
        "ApexClass__GreeterTest": {
            "memberType": "ApexClass",
            "serverRevisionCounter": 2,
            "lastRetrievedFromServer": 2,
            "isNameObsolete": false
        },
        "ApexClass__OnlyInOrg": {
            "memberType": "ApexClass",
            "serverRevisionCounter": 3,
            "lastRetrievedFromServer": 3,
            "isNameObsolete": false
        },
        "CustomObject__Invoice__c": {
            "memberType": "CustomObject",
            "serverRevisionCounter": 4,
            "lastRetrievedFromServer": 4,
            "isNameObsolete": false
        },
        "Layout__Account-My Layout (1)": {
            "memberType": "Layout",
            "serverRevisionCounter": 5,
            "lastRetrievedFromServer": 5,
            "isNameObsolete": true
        }
    }
}`
	if got := readFile(t, revisionFile); got != want {
		t.Errorf("maxRevision.json =\n%s", got)
	}
	// The file is read back to the same members.
	remote, err := loadRemoteTracking(revisionFile)
	if err != nil || remote.maxRevision != 5 || len(remote.keys) != 5 || !remote.members["Layout###Account-My Layout (1)"].IsNameObsolete || !remote.members["CustomObject###Invoice__c"].inSync() {
		t.Errorf("read back: %+v, err %v", remote, err)
	}

	root, _ := os.Getwd()
	repo, _ := openShadowRepo(gitDir, root)
	tracked, err := repo.trackedFiles()
	var names []string
	for name := range tracked {
		names = append(names, name)
	}
	sort.Strings(names)
	if err != nil || strings.Join(names, " ") != "force-app/main/default/classes/Greeter.cls force-app/main/default/classes/Greeter.cls-meta.xml force-app/main/default/classes/GreeterTest.cls force-app/main/default/classes/GreeterTest.cls-meta.xml" {
		t.Errorf("tracked files = %v, err %v", names, err)
	}

	// A second reset finds the files in sync. With a revision, only the
	// members up to it are recorded, and revision 0 leaves the file as it
	// is, because sf does not write a file without members.
	expectRun(t, 0, "Reset local tracking files.\n", "project", "reset", "tracking", "-o", "dev", "-p")
	out, _, _ := run("project", "reset", "tracking", "-o", "dev", "-p", "--json")
	if !strings.Contains(out, `"localPathsSynced": 0`) {
		t.Errorf("second reset = %s", out)
	}
	expectRun(t, 0, "Reset local tracking files to revision 3.\n", "project", "reset", "tracking", "-o", "dev", "-p", "-r", "3")
	if got := readFile(t, revisionFile); !strings.Contains(got, `"serverMaxRevisionCounter": 3`) || strings.Contains(got, "Invoice__c") {
		t.Errorf("maxRevision.json after -r 3 =\n%s", got)
	}
	out, _, _ = run("project", "reset", "tracking", "-o", "dev", "-p", "-r", "0", "--json")
	if !strings.Contains(out, `"sourceMembersSynced": 0`) || !strings.Contains(readFile(t, revisionFile), `"serverMaxRevisionCounter": 3`) {
		t.Errorf("reset to revision 0 = %s", out)
	}
	_, errOut, status := run("project", "reset", "tracking", "-o", "dev", "-p", "-r", "-1")
	if status != 2 || !strings.Contains(errOut, "Expected an integer greater than or equal to 0 but received: -1") {
		t.Errorf("negative revision: status %d, stderr %q", status, errOut)
	}
}

func TestLocalStatusReportsChangesSinceTheLastCommit(t *testing.T) {
	org := trackingProject(t)
	_ = org
	if _, errOut, status := run("project", "reset", "tracking", "-o", "dev", "-p"); status != 0 {
		t.Fatalf("reset: status %d %s", status, errOut)
	}
	classes := filepath.Join("force-app", "main", "default", "classes")
	os.WriteFile(filepath.Join(classes, "Greeter.cls"), []byte("public class Greeter { /* changed */ }\n"), 0o644)
	os.WriteFile(filepath.Join(classes, "Added.cls"), []byte("public class Added {}\n"), 0o644)
	os.Remove(filepath.Join(classes, "GreeterTest.cls"))
	// Files sf does not track.
	os.MkdirAll(filepath.Join("force-app", "main", "default", "lwc", "c", "__tests__"), 0o755)
	os.WriteFile(filepath.Join("force-app", "main", "default", "lwc", "c", "__tests__", "c.test.js"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join("force-app", ".hidden"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join("force-app", "main", ".gitignore"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join("force-app", "node_modules", "m"), 0o755)
	os.WriteFile(filepath.Join("force-app", "node_modules", "m", "index.js"), []byte("x"), 0o644)
	os.WriteFile("outside.cls", []byte("x"), 0o644)

	project, err := loadProject()
	if err != nil {
		t.Fatal(err)
	}
	tracking := &sourceTracking{project: project, orgID: "00D000000000000"}
	repo, _ := openShadowRepo(tracking.gitDir(), project.Root)
	changes, err := tracking.localStatus(repo)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "force-app/main/default/classes/"
	if strings.Join(changes.added, " ") != prefix+"Added.cls" || strings.Join(changes.modified, " ") != prefix+"Greeter.cls" || strings.Join(changes.deleted, " ") != prefix+"GreeterTest.cls" {
		t.Errorf("added %v, modified %v, deleted %v", changes.added, changes.modified, changes.deleted)
	}
}

func TestProjectDeleteTracking(t *testing.T) {
	org := trackingProject(t)
	gitDir, revisionFile := trackingPaths(t)
	run("project", "reset", "tracking", "-o", "dev", "-p")
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "clearedFiles": [
      "`+gitDir+`",
      "`+revisionFile+`"
    ]
  },
  "warnings": []
}
`, "project", "delete", "tracking", "-o", "dev", "-p", "--json")
	for _, path := range []string{gitDir, revisionFile} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s was not deleted", path)
		}
	}
	// Deleting again succeeds, and does not need an org with tracking.
	org.handlers[sourceMemberDescribe] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`[{"errorCode":"NOT_FOUND","message":"The requested resource does not exist"}]`))
	}
	expectRun(t, 0, "Cleared local tracking files.\n", "project", "delete", "tracking", "-o", "dev", "-p")
	_, errOut, status := run("project", "reset", "tracking", "-o", "dev", "-p")
	if status != 1 || errOut != "Error (NonSourceTrackedOrgError): This org does not have source tracking.\n\n\n" {
		t.Errorf("reset for an org without tracking: status %d, stderr %q", status, errOut)
	}
	out, errOut, status := run("force:source:tracking:clear", "-o", "dev", "--noprompt")
	if status != 0 || out != "Cleared local tracking files.\n" || errOut != "Warning: The \"force source tracking clear\" command has been deprecated. Use \"project delete tracking\" instead.\nWarning: The \"--noprompt\" flag has been deprecated. Use \"--no-prompt | -p\" instead.\n" {
		t.Errorf("legacy id: status %d, stdout %q, stderr %q", status, out, errOut)
	}
}
