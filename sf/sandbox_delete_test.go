package sf

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrgDeleteSandbox(t *testing.T) {
	useTempHome(t)
	org := newFakeOrg(t)
	org.login(t, "--alias", "prod")
	home := os.Getenv("HOME")

	_, errOut, status := run("org", "delete", "sandbox", "-o", "prod", "--no-prompt")
	if status != 1 || errOut != "Error (UnknownSandboxError): The org with username: me@example.com is not known by the CLI to be a sandbox\n\n\nTry this:\n\nRe-authenticate the sandbox with the CLI and try again.\nEnsure the CLI has authenticated with the sandbox's production org.\n\n" {
		t.Errorf("not a sandbox: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("org", "delete", "sandbox", "-o", "nobody", "--no-prompt")
	if status == 0 || !strings.Contains(errOut, "Ensure the alias or username for the nobody org is correct.") {
		t.Errorf("unknown org: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("org", "delete", "sandbox", "--no-prompt")
	if status != 1 || !strings.Contains(errOut, "Error (MissingUsernameError): Unable to determine the username of the org to delete.") {
		t.Errorf("no org: status %d, stderr %q", status, errOut)
	}

	// A sandbox is an authorization with a <org id>.sandbox.json file that
	// names its production org.
	authFile := filepath.Join(home, ".sfdx", "me@example.com.json")
	auth, err := os.ReadFile(authFile)
	if err != nil {
		t.Fatal(err)
	}
	sandboxAuth := filepath.Join(home, ".sfdx", "me@example.com.dev1.json")
	os.WriteFile(sandboxAuth, []byte(strings.NewReplacer(`"me@example.com"`, `"me@example.com.dev1"`, `"00D000000000000`, `"00D000000000009`).Replace(string(auth))), 0o600)
	sandbox, err := loadOrg("me@example.com.dev1")
	if err != nil {
		t.Fatal(err)
	}
	orgID := sandbox.Fields.String("orgId")
	configFile := filepath.Join(home, ".sfdx", orgID+".sandbox.json")
	writeConfig := func(infoID string) {
		os.WriteFile(sandboxAuth, []byte(strings.NewReplacer(`"me@example.com"`, `"me@example.com.dev1"`, `"00D000000000000`, `"00D000000000009`).Replace(string(auth))), 0o600)
		os.WriteFile(configFile, []byte(fmt.Sprintf(`{"sandboxUsername":"me@example.com.dev1","sandboxOrgId":"%s","prodOrgUsername":"me@example.com","sandboxName":"dev1","sandboxInfoId":"%s"}`, orgID, infoID)), 0o600)
	}
	var deleted []string
	org.handlers["/services/data/v67.0/tooling/sobjects/SandboxInfo/0GQ000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		deleted = append(deleted, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}

	writeConfig("0GQ000000000001AAA")
	// Without --no-prompt the command asks, and an empty answer is "no".
	if out, _, status := run("org", "delete", "sandbox", "-o", "me@example.com.dev1"); status != 0 || out != "" || len(deleted) != 0 {
		t.Errorf("declined: status %d, stdout %q, requests %v", status, out, deleted)
	}
	expectRun(t, 0, "Successfully marked sandbox me@example.com.dev1 for deletion.\n", "org", "delete", "sandbox", "-o", "me@example.com.dev1", "--no-prompt")
	if len(deleted) != 1 || deleted[0] != "DELETE" {
		t.Errorf("requests = %v", deleted)
	}
	for _, file := range []string{sandboxAuth, configFile} {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("%s still exists", file)
		}
	}

	// Without the SandboxInfo Id in the file, the production org is asked.
	writeConfig("")
	var queries []string
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query().Get("q"))
		io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"SandboxInfoId":"0GQ000000000001AAA"}]}`)
	}
	deleted = nil
	out, _, status := run("org", "delete", "sandbox", "-o", "me@example.com.dev1", "--no-prompt", "--json")
	if status != 0 || len(deleted) != 1 || !strings.Contains(out, `"username": "me@example.com.dev1"`) || !strings.Contains(out, `"orgId": "`+orgID+`"`) ||
		len(queries) != 1 || queries[0] != "SELECT SandboxInfoId FROM SandboxProcess WHERE SandboxName ='dev1' AND Status NOT IN ('D', 'E')" {
		t.Errorf("lookup: status %d, requests %v, queries %q\n%s", status, deleted, queries, out)
	}
}
