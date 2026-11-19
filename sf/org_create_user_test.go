package sf

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ForceCLI/force/lib"
)

// userCreationOrg is a fake scratch org that creates users.  It returns the
// requests it received for the SCIM API and for the update of the user.
func userCreationOrg(t *testing.T) (*fakeOrg, *[]string) {
	t.Helper()
	org := loggedInFakeOrg(t)
	if err := lib.UpdateSFDXAuthFields("me@example.com", lib.SFDXAuthFields{"isScratch": true, "refreshToken": "admin-refresh", "clientId": "PlatformCLI"}); err != nil {
		t.Fatal(err)
	}
	org.queries["FROM User WHERE Username"] = []map[string]interface{}{{
		"Id": "005000000000001AAA", "Username": "me@example.com", "LastName": "Admin", "Alias": "adm", "TimeZoneSidKey": "America/Los_Angeles",
		"LocaleSidKey": "en_US", "EmailEncodingKey": "UTF-8", "ProfileId": "00e000000000001AAA", "LanguageLocaleKey": "en_US", "Email": "admin@example.com",
	}}
	org.queries["FROM Profile WHERE name='Standard User'"] = []map[string]interface{}{{"Id": "00e000000000002AAA"}}
	org.queries["FROM profile WHERE name='Chatter Free User'"] = []map[string]interface{}{{"Id": "00e000000000003AAA"}}
	org.queries["FROM profile WHERE name='Missing'"] = nil
	org.queries["FROM PermissionSet WHERE Name='Missing'"] = nil
	requests := &[]string{}
	org.handlers["/services/scim/v1/Users"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*requests = append(*requests, "scim "+r.Header.Get("auto-approve-user")+" "+string(body))
		if strings.Contains(string(body), `"username":"taken@example.com"`) {
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"Errors":[{"description":"Resource already exists:DUPLICATE_USERNAME:Duplicate Username."}]}`))
			return
		}
		w.Header().Set("auto-approve-user", "new-user-refresh")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"005000000000009AAA"}`))
	}
	org.handlers["/services/data/v67.0/sobjects/User/005000000000009AAA"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*requests = append(*requests, r.Method+" "+string(body))
		w.WriteHeader(http.StatusNoContent)
	}
	org.handlers["/services/data/v67.0/sobjects/User/describe"] = func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"User","fields":[]}`))
	}
	org.handlers["/services/data/v67.0/sobjects/User/005000000000009AAA/password"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	return org, requests
}

func TestOrgCreateUserCreatesAndAuthorizesAUser(t *testing.T) {
	org, requests := userCreationOrg(t)
	home := os.Getenv("HOME")
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "orgId": "00D000000000000",
    "permissionSetAssignments": [],
    "fields": {
      "id": "005000000000009AAA",
      "username": "new@example.com",
      "alias": "adm",
      "email": "admin@example.com",
      "emailencodingkey": "UTF-8",
      "languagelocalekey": "en_US",
      "localesidkey": "en_US",
      "profileid": "00e000000000003AAA",
      "lastname": "Admin",
      "timezonesidkey": "America/Los_Angeles",
      "profilename": "Chatter Free User"
    }
  },
  "warnings": []
}
`, "org", "create", "user", "username=new@example.com", "profileName=Chatter Free User", "-a", "newbie", "--json")
	want := []string{
		`scim true {"emails":["admin@example.com"],"entitlements":[{"value":"00e000000000003AAA"}],"name":{"familyName":"Admin"},"nickName":"new@example.com","username":"new@example.com"}`,
		`PATCH {"Alias":"adm","EmailEncodingKey":"UTF-8","LanguageLocaleKey":"en_US","LocaleSidKey":"en_US","TimeZoneSidKey":"America/Los_Angeles"}`,
	}
	if strings.Join(*requests, "\n") != strings.Join(want, "\n") {
		t.Errorf("requests =\n%s\nwant\n%s", strings.Join(*requests, "\n"), strings.Join(want, "\n"))
	}
	// The refresh token the org gave is exchanged with the client of the
	// target org.
	if len(org.refreshRequests) != 1 || org.refreshRequests[0].Get("refresh_token") != "new-user-refresh" || org.refreshRequests[0].Get("client_id") != "PlatformCLI" {
		t.Errorf("refresh requests = %v", org.refreshRequests)
	}
	fields, order, err := lib.ReadSFDXAuthFieldsOrdered("new@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, " ") != "orgId username accessToken instanceUrl loginUrl refreshToken clientId isDevHub instanceApiVersion instanceApiVersionLastRetrieved userId userProfileName" {
		t.Errorf("auth file keys = %v", order)
	}
	if fields.String("refreshToken") != "new-user-refresh" || fields.String("accessToken") != fakeToken || fields.String("userId") != "005000000000009AAA" || fields.String("userProfileName") != "Chatter Free User" || fields["isDevHub"] != false {
		t.Errorf("auth file = %v", fields)
	}
	if got := readFile(t, filepath.Join(home, ".sfdx", "00D000000000000.json")); got != "{\n  \"usernames\": [\n    \"me@example.com\",\n    \"new@example.com\"\n  ]\n}" {
		t.Errorf("org users file = %s", got)
	}
	if lib.ResolveSFDXUsername("newbie") != "new@example.com" {
		t.Error("the alias was not set")
	}
	// The new user is listed with the users of the org.
	org.queries["SELECT username, profileid, id FROM User"] = []map[string]interface{}{{"Username": "new@example.com", "ProfileId": "00e000000000003AAA", "Id": "005000000000009AAA"}}
	org.queries["SELECT id, name FROM Profile"] = []map[string]interface{}{{"Id": "00e000000000003AAA", "Name": "Chatter Free User"}}
	out, _, _ := run("org", "list", "users")
	if !strings.Contains(out, "│         │ newbie │ new@example.com │") {
		t.Errorf("org list users =\n%s", out)
	}
}

func TestOrgCreateUserReportsPartialSuccess(t *testing.T) {
	_, requests := userCreationOrg(t)
	definition := filepath.Join(os.Getenv("HOME"), "user.json")
	os.WriteFile(definition, []byte(`{"Username":"file@example.com","LastName":"Tester","Title":"Reviewer","generatePassword":true,"permsets":["Missing"]}`), 0o644)
	out, errOut, status := run("org", "create", "user", "-f", definition, "-s", "email=other@example.com")
	want := `=== Partial Success

Successfully created user "file@example.com.00d000000000000" with ID 005000000000009AAA for org 00D000000000000.

See more details about this user by running "sf org user display -o file@example.com.00d000000000000".

=== Failures

┌───────────────────────────┬───────────────────────────────────────────────────────────────────────────────┐
│ Action                    │ Error Message                                                                 │
├───────────────────────────┼───────────────────────────────────────────────────────────────────────────────┤
│ Permission Set Assignment │ Permission set "Missing" not found in target org. Do you need to push source? │
└───────────────────────────┴───────────────────────────────────────────────────────────────────────────────┘

`
	if status != 68 || out != want || errOut != "Warning: This command has generated and displayed a password. Avoid sharing or logging this output as it contains a sensitive credential.\n" {
		t.Errorf("status %d, stderr %q\n%s", status, errOut, out)
	}
	if len(*requests) != 2 || !strings.Contains((*requests)[0], `"emails":["other@example.com"]`) || !strings.Contains((*requests)[0], `"familyName":"Tester"`) ||
		!strings.Contains((*requests)[1], `"Title":"Reviewer"`) || strings.Contains((*requests)[1], "ermsets") || strings.Contains((*requests)[1], "assword") {
		t.Errorf("requests = %v", *requests)
	}
	fields, err := lib.ReadSFDXAuthFields("file@example.com.00d000000000000")
	if err != nil || len(fields.String("password")) != 13 {
		t.Errorf("stored password has %d characters, err %v", len(fields.String("password")), err)
	}
}

func TestOrgCreateUserErrors(t *testing.T) {
	_, requests := userCreationOrg(t)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"profileName=Missing"}, "Error (SingleRecordQuery_NoRecords): No record found for SELECT id FROM profile WHERE name='Missing'\n\n\n"},
		{[]string{"roleDeveloperName=bad__name"}, "Error (InvalidRoleDeveloperNameError): Invalid roleDeveloperName: \"bad__name\". Must start with a letter and contain only alphanumeric characters or single underscores, with no double or final underscores.\n\n\n"},
		{[]string{"username=taken@example.com"}, "Error (duplicateUsername): The username \"taken@example.com\" already exists in this or another Salesforce org. Usernames must be unique across all Salesforce orgs. Try using the --set-unique-username flag to force a unique username by appending the org ID.\n\n\n"},
	} {
		if _, errOut, status := run(append([]string{"org", "create", "user"}, c.args...)...); status != 1 || errOut != c.want {
			t.Errorf("%v: status %d, stderr %q", c.args, status, errOut)
		}
	}
	if len(*requests) != 1 {
		t.Errorf("requests = %v", *requests)
	}
	// An org that is not a scratch org is refused.
	if err := lib.UpdateSFDXAuthFields("me@example.com", lib.SFDXAuthFields{"isScratch": false}); err != nil {
		t.Fatal(err)
	}
	if _, errOut, status := run("org", "create", "user"); status != 1 || errOut != "Error (NonScratchOrgError): This command works with only scratch orgs.\n\n\n" {
		t.Errorf("not a scratch org: status %d, stderr %q", status, errOut)
	}
}
