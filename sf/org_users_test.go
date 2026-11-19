package sf

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ForceCLI/force/lib"
)

// The layouts and messages in this file follow the sf 2.138.6 plugin
// source.  They were not compared with sf against an org.

func TestOrgListSobjectRecordCounts(t *testing.T) {
	org := loggedInFakeOrg(t)
	var query string
	org.handlers["/services/data/v67.0/limits/recordCount"] = func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		io.WriteString(w, `{"sObjects":[{"count":7,"name":"Contact"},{"count":3,"name":"Account"},{"count":0,"name":"Lead"}]}`)
	}
	expectRun(t, 0, `┌─────────┬──────────────┐
│ sObject │ Record Count │
├─────────┼──────────────┤
│ Account │ 3            │
│ Contact │ 7            │
└─────────┴──────────────┘

`, "org", "list", "sobject", "record-counts", "-s", "Account,Contact")
	if query != "sObjects=Account%2CContact" {
		t.Errorf("query = %s", query)
	}
	out, _, _ := run("force:limits:recordcounts:display", "--json")
	if !strings.Contains(out, "\"result\": [\n    {\n      \"count\": 3,\n      \"name\": \"Account\"\n    },") || !strings.Contains(out, `"name": "Lead"`) {
		t.Errorf("json:\n%s", out)
	}
}

func TestOrgAssignPermset(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.rawQueries["FROM User WHERE Username="] = `{"done":true,"totalSize":1,"records":[{"Id":"005000000000001AAA"}]}`
	org.rawQueries["FROM PermissionSet WHERE Name='Dreamhouse'"] = `{"done":true,"totalSize":1,"records":[{"Id":"0PS000000000001AAA"}]}`
	org.rawQueries["FROM PermissionSet WHERE Name='Admin' AND NamespacePrefix='acme'"] = `{"done":true,"totalSize":1,"records":[{"Id":"0PS000000000002AAA"}]}`
	org.rawQueries["FROM PermissionSet WHERE Name='Missing'"] = `{"done":true,"totalSize":0,"records":[]}`
	var created []map[string]string
	org.handlers["/services/data/v67.0/sobjects/PermissionSetAssignment"] = func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		created = append(created, body)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"0Pa000000000001AAA","success":true,"errors":[]}`)
	}
	expectRun(t, 0, `Permsets Assigned
┌────────────────┬───────────────────────────┐
│ Username       │ Permission Set Assignment │
├────────────────┼───────────────────────────┤
│ me@example.com │ Dreamhouse                │
│ me@example.com │ acme__Admin               │
└────────────────┴───────────────────────────┘

`, "org", "assign", "permset", "-n", "Dreamhouse", "-n", "acme__Admin")
	if len(created) != 2 || created[0]["AssigneeId"] != "005000000000001AAA" || created[0]["PermissionSetId"] != "0PS000000000001AAA" || created[1]["PermissionSetId"] != "0PS000000000002AAA" {
		t.Errorf("created = %v", created)
	}

	out, _, status := run("org", "assign", "permset", "--name", "Dreamhouse", "--name", "Missing", "-b", "dev")
	if status != 68 || !strings.HasPrefix(out, "Partial Success\n") || !strings.Contains(out, "\nFailures\n") ||
		!strings.Contains(out, `│ dev      │ Permission set "Missing" not found in target org. Do you need to push source? │`) {
		t.Errorf("partial: status %d\n%s", status, out)
	}
	out, _, status = run("org", "assign", "permset", "-n", "Missing", "--json")
	if status != 1 || !strings.Contains(out, `"successes": []`) || !strings.Contains(out, `"message": "Permission set \"Missing\" not found in target org. Do you need to push source?"`) || !strings.Contains(out, `"status": 1`) {
		t.Errorf("failure: status %d\n%s", status, out)
	}
}

func TestOrgAssignPermsetLicense(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.rawQueries["FROM User WHERE Username="] = `{"done":true,"totalSize":1,"records":[{"Id":"005000000000001AAA"}]}`
	org.rawQueries["FROM PermissionSetLicense"] = `{"done":true,"totalSize":1,"records":[{"Id":"0PL000000000001AAA"}]}`
	var created map[string]string
	org.handlers["/services/data/v67.0/sobjects/PermissionSetLicenseAssign"] = func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&created)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"2LA000000000001AAA","success":true,"errors":[]}`)
	}
	out, _, status := run("org", "assign", "permsetlicense", "-n", "DreamHouse")
	if status != 0 || !strings.HasPrefix(out, "Permset Licenses Assigned\n") || !strings.Contains(out, "│ me@example.com │ DreamHouse ") {
		t.Errorf("status %d\n%s", status, out)
	}
	if created["PermissionSetLicenseId"] != "0PL000000000001AAA" || created["AssigneeId"] != "005000000000001AAA" {
		t.Errorf("created = %v", created)
	}
}

func TestOrgGeneratePasswordStoresThePassword(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.rawQueries["FROM User WHERE Username="] = `{"done":true,"totalSize":1,"records":[{"Id":"005000000000001AAA"}]}`
	var sent map[string]string
	org.handlers["/services/data/v67.0/sobjects/User/005000000000001AAA/password"] = func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&sent)
		w.WriteHeader(http.StatusNoContent)
	}
	out, _, status := run("org", "generate", "password", "--json")
	var env struct {
		Result struct{ Username, Password string }
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil || status != 0 {
		t.Fatalf("status %d, %v\n%s", status, err, out)
	}
	if len(env.Result.Password) != 20 || sent["NewPassword"] != env.Result.Password || env.Result.Username != "me@example.com" {
		t.Errorf("result %+v, sent %v", env.Result, sent)
	}
	fields, _ := lib.ReadSFDXAuthFields("me@example.com")
	if fields.String("password") != env.Result.Password {
		t.Errorf("stored password = %q", fields.String("password"))
	}
	data, _ := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".sfdx", "me@example.com.json"))
	if strings.Contains(string(data), env.Result.Password) {
		t.Error("the password is stored without encryption")
	}
	out, _, _ = run("org", "auth", "show-user-password", "--json")
	if !strings.Contains(out, `"password": `) || !strings.Contains(out, env.Result.Password) {
		t.Errorf("show-user-password:\n%s", out)
	}

	out, errOut, status := run("org", "generate", "password", "-l", "8", "-c", "1")
	if status != 0 || !strings.HasPrefix(out, "Successfully set the password \"") ||
		!strings.HasSuffix(out, "You can see the password again by running \"sf org auth show-user-password -o me@example.com\".\n") ||
		!strings.Contains(errOut, "generating a password of length 20") || !strings.Contains(errOut, "generating a password of complexity 3") {
		t.Errorf("short password: status %d, stdout %q, stderr %q", status, out, errOut)
	}
}

func TestGeneratePasswordUsesTheCharacterSet(t *testing.T) {
	password, err := generatePassword(40, 0)
	if err != nil || len(password) != 40 || strings.ToLower(password) != password || strings.ContainsAny(password, "0123456789!@#") {
		t.Errorf("complexity 0 password = %q, err %v", password, err)
	}
	// A password of the highest complexity has a character of every
	// class, as an org requires, and lower case letters otherwise.
	for i := 0; i < 50; i++ {
		password, err := generatePassword(13, 5)
		if err != nil || len(password) != 13 || !strings.ContainsAny(password, passwordSymbols) || !strings.ContainsAny(password, passwordNumbers) ||
			!strings.ContainsAny(password, passwordUpper) || !strings.ContainsAny(password, passwordLower) {
			t.Fatalf("complexity 5 password = %q, err %v", password, err)
		}
		other := 0
		for _, character := range password {
			if !strings.ContainsRune(passwordLower, character) {
				other++
			}
		}
		if other != 3 {
			t.Fatalf("complexity 5 password %q has %d characters that are not lower case letters", password, other)
		}
	}
}

func TestOrgDeleteScratch(t *testing.T) {
	org := loggedInFakeOrg(t)
	_, errOut, status := run("org", "delete", "scratch", "--no-prompt")
	if status != 1 || errOut != "Error (UnknownScratchError): Unable to find a scratch org with username me@example.com.\n\n\n" {
		t.Errorf("not a scratch org: status %d, stderr %q", status, errOut)
	}

	writeTestAuth(t, "test-scratch@example.com", lib.SFDXAuthFields{"isScratch": true, "devHubUsername": "me@example.com", "instanceUrl": org.URL, "orgId": "00D000000000009"})
	lib.SetSFDXAlias("scratch", "test-scratch@example.com")
	org.rawQueries["FROM ActiveScratchOrg WHERE SignupUsername='test-scratch@example.com'"] = `{"done":true,"totalSize":1,"records":[{"Id":"00D000000000009AAA"}]}`
	deleted := false
	org.handlers["/services/data/v67.0/sobjects/ActiveScratchOrg/00D000000000009AAA"] = func(w http.ResponseWriter, r *http.Request) {
		deleted = r.Method == "DELETE"
		w.WriteHeader(http.StatusNoContent)
	}
	// sf prints nothing after it deleted the org.
	expectRun(t, 0, "", "org", "delete", "scratch", "-o", "scratch", "-p")
	if !deleted {
		t.Error("the ActiveScratchOrg record was not deleted")
	}
	if _, err := lib.ReadSFDXAuthFields("test-scratch@example.com"); err == nil {
		t.Error("the authorization file still exists")
	}
	if lib.ResolveSFDXUsername("scratch") != "scratch" {
		t.Error("the alias still exists")
	}
	if _, err := lib.ReadSFDXAuthFields("me@example.com"); err != nil {
		t.Errorf("the Dev Hub authorization was removed: %v", err)
	}

	// An org that the Dev Hub no longer has is reported as deleted.
	writeTestAuth(t, "test-gone@example.com", lib.SFDXAuthFields{"isScratch": true, "devHubUsername": "me@example.com", "instanceUrl": org.URL, "orgId": "00D000000000008"})
	org.rawQueries["FROM ActiveScratchOrg WHERE SignupUsername='test-gone@example.com'"] = `{"done":true,"totalSize":0,"records":[]}`
	expectRun(t, 0, "Successfully deleted scratch org test-gone@example.com.\n", "org", "delete", "scratch", "-o", "test-gone@example.com", "-p")
}

func TestOrgDeleteScratchNeedsAnOrg(t *testing.T) {
	useTempHome(t)
	_, errOut, status := run("org", "delete", "scratch")
	if status != 1 || errOut != "Error (MissingUsernameError): Unable to determine the username of the org to delete. Specify the username with the --target-org | -o flag.\n\n\n" {
		t.Errorf("status %d, stderr %q", status, errOut)
	}
}

func TestOrgListUsers(t *testing.T) {
	home := useTempHome(t)
	org := newFakeOrg(t)
	org.login(t, "--alias", "dev", "--set-default")
	org.queries["SELECT username, profileid, id FROM User"] = []map[string]interface{}{
		{"Username": "me@example.com", "ProfileId": "00e000000000001AAA", "Id": "005000000000001AAA"},
		{"Username": "other@example.com", "ProfileId": "00e000000000002AAA", "Id": "005000000000002AAA"},
	}
	org.queries["SELECT id, name FROM Profile"] = []map[string]interface{}{
		{"Id": "00e000000000001AAA", "Name": "System Administrator"},
		{"Id": "00e000000000002AAA", "Name": "Standard User"},
	}
	expectRun(t, 0, `Users in org 00D000000000000
┌─────────┬───────┬────────────────┬──────────────────────┬────────────────────┐
│ Default │ Alias │ Username       │ Profile Name         │ User Id            │
├─────────┼───────┼────────────────┼──────────────────────┼────────────────────┤
│ (A)     │ dev   │ me@example.com │ System Administrator │ 005000000000001AAA │
└─────────┴───────┴────────────────┴──────────────────────┴────────────────────┘

`, "org", "list", "users")

	// The users sf created for the org are listed in <orgId>.json.  One
	// that is not the target org has no marker.
	fields, err := lib.ReadSFDXAuthFields("me@example.com")
	if err != nil {
		t.Fatal(err)
	}
	fields["username"] = "other@example.com"
	if err := lib.WriteSFDXAuthFields(fields); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(home, ".sfdx", "00D000000000000.json"), []byte(`{"usernames":["me@example.com","other@example.com"]}`), 0o600)
	out, errOut, status := run("org", "list", "users", "--json")
	if status != 0 {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	for _, want := range []string{`    {
      "defaultMarker": "",
      "alias": "",
      "username": "other@example.com",
      "profileName": "Standard User",
      "orgId": "00D000000000000",
      "accessToken": "[REDACTED] Use 'sf org auth show-access-token' to view",
      "instanceUrl": "` + org.Server.URL + `/",
      "loginUrl": "` + org.Server.URL + `/",
      "userId": "005000000000002AAA"
    }`, `"defaultMarker": "(A)",
      "alias": "dev",`, "Secrets are now hidden from 'sf org list users' command output."} {
		if !strings.Contains(out, want) {
			t.Errorf("json lacks %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, fakeToken) {
		t.Error("the access token is in the output")
	}
}

func TestListFlagsAreSplitAtCommasOnlyWhereSfSplitsThem(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.queries["FROM PermissionSet WHERE Name='A,B'"] = nil
	// org assign permset takes a name with a comma as one name, without
	// a warning.
	out, errOut, status := run("org", "assign", "permset", "-n", "A,B")
	if status != 1 || errOut != "" || !strings.Contains(out, `Permission set "A,B" not found in target org.`) {
		t.Errorf("assign permset: status %d, stderr %q\n%s", status, errOut, out)
	}
	// org list sobject record-counts splits its list at commas and warns
	// that the form is deprecated.
	var requested string
	org.handlers["/services/data/v67.0/limits/recordCount"] = func(w http.ResponseWriter, r *http.Request) {
		requested = r.URL.Query().Get("sObjects")
		io.WriteString(w, `{"sObjects":[{"count":1,"name":"Account"}]}`)
	}
	_, errOut, status = run("org", "list", "sobject", "record-counts", "-s", "Account,User")
	if status != 0 || requested != "Account,User" || errOut != "Warning: "+commaListWarning+"\n" {
		t.Errorf("record counts: status %d, sObjects %q, stderr %q", status, requested, errOut)
	}
	out, _, _ = run("org", "list", "sobject", "record-counts", "-s", "Account,User", "--json")
	if !strings.Contains(out, "\"warnings\": [\n    \""+commaListWarning+"\"") {
		t.Errorf("record counts json:\n%s", out)
	}
}

func TestOrgGeneratePasswordWarnsAndNeedsALocalUser(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/data/v67.0/sobjects/User/005000000000001AAA/password"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	const warning = "This command has generated and displayed a password. Avoid sharing or logging this output as it contains a sensitive credential."
	out, errOut, status := run("org", "generate", "password")
	if status != 0 || errOut != "Warning: "+warning+"\n" || !strings.HasPrefix(out, "Successfully set the password ") {
		t.Errorf("status %d, stdout %q, stderr %q", status, out, errOut)
	}
	out, _, status = run("org", "generate", "password", "--json")
	if status != 0 || !strings.Contains(out, "\"warnings\": [\n    \""+warning+"\"") {
		t.Errorf("json: status %d\n%s", status, out)
	}
	_, errOut, status = run("org", "generate", "password", "-b", "nobody@example.com")
	if status != 1 || !strings.HasPrefix(errOut, "Error (NamedOrgNotFoundError): No authorization information found for nobody@example.com.\n") {
		t.Errorf("unknown user: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("org", "generate", "password", "-l", "5000")
	if status != 2 || !strings.Contains(errOut, "Expected an integer less than or equal to 1000 but received: 5000") {
		t.Errorf("length: status %d, stderr %q", status, errOut)
	}
}
