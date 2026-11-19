package sf

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The export output in this file was captured from sf 2.138.6.  The import
// and file upload output follows the sf plugin source.

func serveTreeOrg(org *fakeOrg) {
	org.rawQueries["FROM Account"] = `{"done":true,"totalSize":1,"records":[{"attributes":{"type":"Account","url":"/a"},"Id":"001000000000001AAA","Name":"TreeCo","Phone":null,"ParentId":null,"Contacts":{"totalSize":2,"done":true,"records":[` +
		`{"attributes":{"type":"Contact","url":"/c1"},"Id":"003000000000001AAA","LastName":"Leaf"},` +
		`{"attributes":{"type":"Contact","url":"/c2"},"Id":"003000000000002AAA","LastName":"Branch","ReportsToId":"003000000000001AAA"}]}}]}`
	org.handlers["/services/data/v67.0/sobjects/Account/describe"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"name":"Account","fields":[{"name":"Id","type":"id"},{"name":"ParentId","type":"reference","referenceTo":["Account"]}],"childRelationships":[{"childSObject":"Contact","field":"AccountId","relationshipName":"Contacts"}]}`)
	}
	org.handlers["/services/data/v67.0/sobjects/Contact/describe"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"name":"Contact","fields":[{"name":"AccountId","type":"reference","referenceTo":["Account"]},{"name":"ReportsToId","type":"reference","referenceTo":["Contact"]}],"childRelationships":[]}`)
	}
}

func TestDataExportTree(t *testing.T) {
	org := loggedInFakeOrg(t)
	serveTreeOrg(org)
	dir := t.TempDir()
	query := "SELECT Id, Name, Phone, ParentId, (SELECT Id, LastName, ReportsToId FROM Contacts) FROM Account"
	expectRun(t, 0, "wrote 1 records to "+filepath.Join(dir, "Account-Contact.json")+"\n", "data", "export", "tree", "-q", query, "-d", dir)
	written, _ := os.ReadFile(filepath.Join(dir, "Account-Contact.json"))
	want := `{
    "records": [
        {
            "attributes": {
                "type": "Account",
                "referenceId": "AccountRef1"
            },
            "Name": "TreeCo",
            "Contacts": {
                "records": [
                    {
                        "attributes": {
                            "type": "Contact",
                            "referenceId": "ContactRef1"
                        },
                        "LastName": "Leaf"
                    },
                    {
                        "attributes": {
                            "type": "Contact",
                            "referenceId": "ContactRef2"
                        },
                        "LastName": "Branch",
                        "ReportsToId": "@ContactRef1"
                    }
                ]
            }
        }
    ]
}`
	if string(written) != want {
		t.Errorf("Account-Contact.json =\n%s\nwant\n%s", written, want)
	}
}

func TestDataExportTreeWithPlan(t *testing.T) {
	org := loggedInFakeOrg(t)
	serveTreeOrg(org)
	dir := t.TempDir()
	out, _, status := run("data", "export", "tree", "--query", "SELECT Id, Name FROM Account", "--output-dir", dir, "--plan", "--prefix", "pre")
	want := "wrote 1 records to " + filepath.Join(dir, "pre-Account.json") + "\nwrote 2 records to " + filepath.Join(dir, "pre-Contact.json") + "\n"
	if status != 0 || out != want {
		t.Errorf("status %d, stdout %q", status, out)
	}
	plan, _ := os.ReadFile(filepath.Join(dir, "pre-Account-Contact-plan.json"))
	wantPlan := `[
    {
        "sobject": "Account",
        "files": [
            "pre-Account.json"
        ]
    },
    {
        "sobject": "Contact",
        "files": [
            "pre-Contact.json"
        ]
    }
]`
	if string(plan) != wantPlan {
		t.Errorf("plan =\n%s", plan)
	}
	contacts, _ := os.ReadFile(filepath.Join(dir, "pre-Contact.json"))
	// Each child gets the lookup to its parent as a reference.
	if strings.Count(string(contacts), `"AccountId": "@AccountRef1"`) != 2 || !strings.Contains(string(contacts), `"ReportsToId": "@ContactRef1"`) {
		t.Errorf("pre-Contact.json =\n%s", contacts)
	}
	accounts, _ := os.ReadFile(filepath.Join(dir, "pre-Account.json"))
	if strings.Contains(string(accounts), "Contacts") || strings.Contains(string(accounts), `"Id"`) {
		t.Errorf("pre-Account.json keeps children or the Id:\n%s", accounts)
	}

	_, errOut, status := run("data", "export", "tree", "-q", "DELETE FROM Account")
	if status != 1 || !strings.Contains(errOut, "Error (soqlInvalid): Invalid SOQL query: DELETE FROM Account") {
		t.Errorf("invalid query: status %d, stderr %q", status, errOut)
	}
}

func TestDataImportTreeResolvesReferencesAcrossFiles(t *testing.T) {
	org := loggedInFakeOrg(t)
	var contactRequest string
	org.handlers["/services/data/v67.0/composite/tree/Account"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"hasErrors":false,"results":[{"referenceId":"AccountRef1","id":"001000000000009AAA"}]}`)
	}
	org.handlers["/services/data/v67.0/composite/tree/Contact"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		contactRequest = string(body)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"hasErrors":false,"results":[{"referenceId":"ContactRef1","id":"003000000000009AAA"}]}`)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Account.json"), []byte(`{"records":[{"attributes":{"type":"Account","referenceId":"AccountRef1"},"Name":"TreeCo"}]}`), 0o644)
	os.WriteFile(filepath.Join(dir, "Contact.json"), []byte(`{"records":[{"attributes":{"type":"Contact","referenceId":"ContactRef1"},"LastName":"Leaf","AccountId":"@AccountRef1"}]}`), 0o644)
	plan := filepath.Join(dir, "plan.json")
	os.WriteFile(plan, []byte(`[{"sobject":"Account","files":["Account.json"]},{"sobject":"Contact","files":["Contact.json"]}]`), 0o644)

	expectRun(t, 0, `Import Results
┌──────────────┬─────────┬────────────────────┐
│ Reference ID │ Type    │ ID                 │
├──────────────┼─────────┼────────────────────┤
│ AccountRef1  │ Account │ 001000000000009AAA │
│ ContactRef1  │ Contact │ 003000000000009AAA │
└──────────────┴─────────┴────────────────────┘

`, "data", "import", "tree", "-p", plan)
	var request struct {
		Records []map[string]interface{} `json:"records"`
	}
	json.Unmarshal([]byte(contactRequest), &request)
	if len(request.Records) != 1 || request.Records[0]["AccountId"] != "001000000000009AAA" {
		t.Errorf("the reference to the account was not replaced by its Id: %s", contactRequest)
	}

	out, _, status := run("data", "import", "tree", "--files", filepath.Join(dir, "Account.json"), "--json")
	if status != 0 || !strings.Contains(out, "\"refId\": \"AccountRef1\",\n      \"type\": \"Account\",\n      \"id\": \"001000000000009AAA\"") {
		t.Errorf("--files json: status %d\n%s", status, out)
	}

	org.handlers["/services/data/v67.0/composite/tree/Account"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"hasErrors":true,"results":[{"referenceId":"AccountRef1","errors":[{"statusCode":"REQUIRED_FIELD_MISSING","message":"Required fields are missing: [Name]"}]}]}`)
	}
	_, errOut, status := run("data", "import", "tree", "-f", filepath.Join(dir, "Account.json"))
	if status != 1 || !strings.Contains(errOut, "Error (dataImportFailed): Import failed from file:") || !strings.Contains(errOut, "REQUIRED_FIELD_MISSING") {
		t.Errorf("failed import: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("data", "import", "tree")
	if status != 2 || !strings.Contains(errOut, "Exactly one of the following must be provided: --files, --plan") {
		t.Errorf("no input: status %d, stderr %q", status, errOut)
	}
}

func TestDataCreateFile(t *testing.T) {
	org := loggedInFakeOrg(t)
	var request map[string]string
	org.handlers["/services/data/v67.0/sobjects/ContentVersion"] = func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&request)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"068000000000001AAA","success":true,"errors":[]}`)
	}
	org.rawQueries["FROM ContentVersion"] = `{"done":true,"totalSize":1,"records":[{"attributes":{"type":"ContentVersion"},"Id":"068000000000001AAA","ContentDocumentId":"069000000000001AAA","FileExtension":"txt","Title":"Greeting"}]}`
	file := filepath.Join(t.TempDir(), "hello.txt")
	os.WriteFile(file, []byte("hello\n"), 0o644)
	expectRun(t, 0, "Created file with ContentDocumentId 069000000000001AAA.\nFile attached to record with ID 001000000000001AAA.\n",
		"data", "create", "file", "-f", file, "-t", "Greeting", "-i", "001000000000001AAA")
	if request["Title"] != "Greeting" || request["PathOnClient"] != "hello.txt" || request["VersionData"] != "aGVsbG8K" || request["FirstPublishLocationId"] != "001000000000001AAA" {
		t.Errorf("request = %v", request)
	}
	if _, errOut, status := run("data", "create", "file", "-f", "missing.txt"); status != 2 || errOut != "Error (2): Parsing --file \n\tNo file found at missing.txt\nSee more help with --help\n\n\n" {
		t.Errorf("missing file: status %d, stderr %q", status, errOut)
	}
	out, _, _ := run("data", "create", "file", "--file", file, "--json")
	// The result is the record as the org returns it, with its attributes.
	if !strings.Contains(out, `"ContentDocumentId": "069000000000001AAA"`) || !strings.Contains(out, "\"result\": {\n    \"attributes\": {") || request["Title"] != "hello.txt" {
		t.Errorf("json: %s, title %s", out, request["Title"])
	}
}

func TestDataImportTreeListsTheErrorsOfTheRecords(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/data/v67.0/composite/tree/Contact"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"hasErrors":true,"results":[{"referenceId":"BadRef1","errors":[{"statusCode":"REQUIRED_FIELD_MISSING","message":"Required fields are missing: [LastName]","fields":["LastName"]}]}]}`)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "Bad.json")
	os.WriteFile(file, []byte(`{"records":[{"attributes":{"type":"Contact","referenceId":"BadRef1"},"FirstName":"NoLastName"}]}`), 0o644)
	out, errOut, status := run("data", "import", "tree", "-f", file)
	want := `Tree import errors
┌──────────────┬────────────────────────┬─────────────────────────────────────────┬──────────┐
│ Reference ID │ Status Code            │ Error Message                           │ Fields   │
├──────────────┼────────────────────────┼─────────────────────────────────────────┼──────────┤
│ BadRef1      │ REQUIRED_FIELD_MISSING │ Required fields are missing: [LastName] │ LastName │
└──────────────┴────────────────────────┴─────────────────────────────────────────┴──────────┘

`
	if status != 1 || out != want || errOut != "Error (SfError): Data Import failed\n\n\n" {
		t.Errorf("status %d, stderr %q\n%s", status, errOut, out)
	}
	out, _, status = run("data", "import", "tree", "-f", file, "--json")
	if status != 1 || !strings.Contains(out, `"name": "SfError"`) || !strings.Contains(out, `  "data": [
    {
      "referenceId": "BadRef1",
      "StatusCode": "REQUIRED_FIELD_MISSING",
      "Message": "Required fields are missing: [LastName]",
      "fields": "LastName"
    }
  ],`) {
		t.Errorf("json: status %d\n%s", status, out)
	}

	for _, c := range []struct {
		args   []string
		status int
		want   string
	}{
		{[]string{"-f", "nope.json"}, 1, "Error (ENOENT): ENOENT: no such file or directory, open 'nope.json'\n\n\n"},
		{[]string{"-f", file, "-p", "plan.json"}, 2, "Error (2): The following errors occurred:\n  --files cannot also be provided when using --plan\n  --plan cannot also be provided when using --files\nSee more help with --help\n\n\n"},
	} {
		if _, errOut, status := run(append([]string{"data", "import", "tree"}, c.args...)...); status != c.status || errOut != c.want {
			t.Errorf("%v: status %d, stderr %q", c.args, status, errOut)
		}
	}
}

func TestRecordIDComesFromTheRecordURL(t *testing.T) {
	record := orderedFields("attributes", orderedFields("type", "Account", "url", "/services/data/v67.0/sobjects/Account/001000000000001AAA"), "Name", "Acme")
	if got := recordID(record); got != "001000000000001AAA" {
		t.Errorf("recordID = %q", got)
	}
	record.Set("Id", "001000000000002AAA")
	if got := recordID(record); got != "001000000000002AAA" {
		t.Errorf("recordID with an Id field = %q", got)
	}
}
