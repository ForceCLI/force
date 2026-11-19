package sf

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlagsDirSuppliesFlagValues(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.rawQueries["FROM Account"] = `{"done":true,"totalSize":1,"records":[{"Name":"Acme"}]}`
	org.rawQueries["FROM Contact"] = `{"done":true,"totalSize":1,"records":[{"LastName":"Lovelace"}]}`
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "query"), []byte("SELECT Name FROM Account\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "result-format"), []byte("csv"), 0o644)
	os.WriteFile(filepath.Join(dir, "json"), nil, 0o644)

	out, _, status := run("data", "query", "--flags-dir", dir)
	if status != 0 || !strings.Contains(out, `"Name": "Acme"`) || !strings.HasPrefix(out, "{\n  \"status\": 0,") {
		t.Errorf("flags from files: status %d\n%s", status, out)
	}
	// A flag on the command line is used in place of the file, also when
	// it is given by its short name.
	out, _, status = run("data", "query", "--flags-dir="+dir, "-q", "SELECT LastName FROM Contact")
	if status != 0 || !strings.Contains(out, `"LastName": "Lovelace"`) || strings.Contains(out, "Acme") {
		t.Errorf("short flag on the command line: status %d\n%s", status, out)
	}
	out, _, status = run("data", "query", "--flags-dir", dir, "--query", "SELECT LastName FROM Contact")
	if status != 0 || !strings.Contains(out, `"LastName": "Lovelace"`) || strings.Contains(out, "Acme") {
		t.Errorf("command line flag: status %d\n%s", status, out)
	}

	_, errOut, status := run("data", "query", "--flags-dir", filepath.Join(dir, "nosuch"))
	if status != 2 || !strings.Contains(errOut, "No directory found at") {
		t.Errorf("missing directory: status %d, stderr %q", status, errOut)
	}
}

func TestExpandFlagsDirRepeatsMultiLineValues(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "metadata"), []byte("ApexClass:A\nApexClass:B\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "w"), []byte("10"), 0o644)
	got, err := expandFlagsDir(nil, []string{"project", "deploy", "start", "--flags-dir", dir, "--", "tail"})
	if err != nil {
		t.Fatal(err)
	}
	want := "project deploy start --metadata=ApexClass:A --metadata=ApexClass:B -w=10 -- tail"
	if strings.Join(got, " ") != want {
		t.Errorf("args = %v\nwant   %s", got, want)
	}
}

func TestDeployAddsDestructiveChanges(t *testing.T) {
	_, fake, _ := deploySetup(t)
	home := os.Getenv("HOME")
	manifest := filepath.Join(home, "package.xml")
	pre := filepath.Join(home, "pre.xml")
	post := filepath.Join(home, "post.xml")
	os.WriteFile(manifest, []byte(`<Package xmlns="http://soap.sforce.com/2006/04/metadata"><types><members>Greeter</members><name>ApexClass</name></types><version>67.0</version></Package>`), 0o644)
	os.WriteFile(pre, []byte("<Package><!-- pre --></Package>"), 0o644)
	os.WriteFile(post, []byte("<Package><!-- post --></Package>"), 0o644)
	if _, errOut, status := run("project", "deploy", "start", "-x", manifest, "--pre-destructive-changes", pre, "--post-destructive-changes", post, "--purge-on-delete", "--json"); status != 0 {
		t.Fatalf("status %d %s", status, errOut)
	}
	if fake.deployed["destructiveChangesPre.xml"] != "<Package><!-- pre --></Package>" || fake.deployed["destructiveChangesPost.xml"] != "<Package><!-- post --></Package>" {
		t.Errorf("deployed files = %v", keys(fake.deployed))
	}
	if !strings.Contains(fake.options, "<purgeOnDelete>true</purgeOnDelete>") {
		t.Error("purgeOnDelete is not set")
	}
	_, errOut, status := run("project", "deploy", "start", "-d", "force-app", "--post-destructive-changes", post)
	if status != 2 || !strings.Contains(errOut, "All of the following must be provided when using --post-destructive-changes: --manifest") {
		t.Errorf("without a manifest: status %d, stderr %q", status, errOut)
	}
}

func TestApiRequestRestFromFile(t *testing.T) {
	org := loggedInFakeOrg(t)
	var method, body, custom string
	org.handlers["/services/data/v67.0/sobjects/Account"] = func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		method, body, custom = r.Method, string(data), r.Header.Get("X-Custom")
		io.WriteString(w, `{"id":"001000000000001AAA"}`)
	}
	file := filepath.Join(t.TempDir(), "request.json")
	os.WriteFile(file, []byte(`{
  "url": {"raw": "/services/data/v67.0/sobjects/Account"},
  "method": "POST",
  "header": [{"key": "X-Custom", "value": "from-file"}, {"key": "X-Off", "value": "no", "disabled": true}],
  "body": {"mode": "raw", "raw": {"Name": "FromFile"}}
}`), 0o644)
	out, _, status := run("api", "request", "rest", "--file", file)
	if status != 0 || method != "POST" || body != `{"Name":"FromFile"}` || custom != "from-file" || !strings.Contains(out, "001000000000001AAA") {
		t.Errorf("status %d, method %s, body %s, header %s\n%s", status, method, body, custom, out)
	}
	// Flags replace the values in the file.
	if _, _, status := run("api", "request", "rest", "-f", file, "-X", "PATCH", "-H", "X-Custom: from-flag", "-b", `{"Name":"FromFlag"}`); status != 0 || method != "PATCH" || custom != "from-flag" || body != `{"Name":"FromFlag"}` {
		t.Errorf("flags: status %d, method %s, body %s, header %s", status, method, body, custom)
	}
	os.WriteFile(file, []byte(`{"url":"/x","method":"POST","body":{"mode":"formdata","formdata":[]}}`), 0o644)
	_, errOut, status := run("api", "request", "rest", "--file", file)
	if status != 1 || !strings.Contains(errOut, `The body mode "formdata" in --file is not supported`) {
		t.Errorf("formdata: status %d, stderr %q", status, errOut)
	}
}

func TestAutocompletePrintsInstructionsAndScripts(t *testing.T) {
	out, _, status := run("autocomplete", "bash")
	if status != 0 || !strings.Contains(out, "source <(sf autocomplete script bash)") {
		t.Errorf("instructions: status %d\n%s", status, out)
	}
	out, _, status = run("autocomplete", "script", "bash")
	if status != 0 || !strings.Contains(out, "bash completion V2 for sf") {
		t.Errorf("script: status %d\n%s", status, out[:min(len(out), 200)])
	}
	_, errOut, status := run("autocomplete", "tcsh")
	if status != 1 || !strings.Contains(errOut, "tcsh is not a supported shell for autocomplete") {
		t.Errorf("unsupported shell: status %d, stderr %q", status, errOut)
	}
}
