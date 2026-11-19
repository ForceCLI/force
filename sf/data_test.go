package sf

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The output formats in this file were captured from sf 2.138.6.

const accountQueryResponse = `{"done":true,"totalSize":2,"records":[` +
	`{"Name":"Acme","Phone":"555","Owner":{"Username":"admin@example.com","attributes":{"type":"User","url":"/services/data/v67.0/sobjects/User/005000000000001AAA"}},"attributes":{"type":"Account","url":"/services/data/v67.0/sobjects/Account/001000000000001AAA"}},` +
	`{"Name":"Beta, Inc.","Phone":null,"Owner":{"Username":"admin@example.com","attributes":{"type":"User","url":"/services/data/v67.0/sobjects/User/005000000000001AAA"}},"attributes":{"type":"Account","url":"/services/data/v67.0/sobjects/Account/001000000000002AAA"}}]}`

func loggedInFakeOrg(t *testing.T) *fakeOrg {
	t.Helper()
	useTempHome(t)
	org := newFakeOrg(t)
	org.login(t, "--alias", "dev", "--set-default")
	org.requests = nil
	return org
}

func TestDataQueryHumanOutput(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.rawQueries["FROM Account"] = accountQueryResponse
	out, errOut, status := run("data", "query", "-q", "SELECT Name, Phone, Owner.Username FROM Account")
	want := `┌────────────┬───────┬───────────────────┐
│ NAME       │ PHONE │ OWNER.USERNAME    │
├────────────┼───────┼───────────────────┤
│ Acme       │ 555   │ admin@example.com │
│ Beta, Inc. │ null  │ admin@example.com │
└────────────┴───────┴───────────────────┘

Total number of records retrieved: 2.
`
	if status != 0 || out != want {
		t.Errorf("status %d, stdout\n%s\nwant\n%s", status, out, want)
	}
	if errOut != "Querying Data... done\n" {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestDataQueryJSONKeepsFieldOrder(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.rawQueries["FROM Account"] = `{"done":true,"totalSize":1,"records":[{"Phone":"555","Name":"Acme","attributes":{"type":"Account","url":"/x"}}]}`
	want := `{
  "status": 0,
  "result": {
    "records": [
      {
        "Phone": "555",
        "Name": "Acme",
        "attributes": {
          "type": "Account",
          "url": "/x"
        }
      }
    ],
    "totalSize": 1,
    "done": true
  },
  "warnings": []
}
`
	expectRun(t, 0, want, "data", "query", "--query", "SELECT Phone, Name FROM Account", "--json")
	deprecated := strings.Replace(want, `"warnings": []`, `"warnings": [
    "The \"force data soql query\" command has been deprecated. Use \"data query\" instead."
  ]`, 1)
	expectRun(t, 0, deprecated, "force:data:soql:query", "-q", "SELECT Phone, Name FROM Account", "--json")

	// --result-format json prints the envelope without "warnings" and no
	// progress line.
	out, errOut, _ := run("data", "query", "-q", "SELECT Phone, Name FROM Account", "-r", "json")
	if strings.Contains(out, "warnings") || !strings.HasPrefix(out, "{\n  \"status\": 0,\n  \"result\": {") || errOut != "" {
		t.Errorf("-r json: stdout %s stderr %q", out, errOut)
	}
}

func TestDataQueryCSV(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.rawQueries["FROM Account"] = accountQueryResponse
	expectRun(t, 0, "Name,Phone,Owner.Username\nAcme,555,admin@example.com\n\"Beta, Inc.\",,admin@example.com\n",
		"data", "query", "-q", "SELECT Name, Phone, Owner.Username FROM Account", "-r", "csv")
}

func TestDataQueryFollowsNextRecordsUrl(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.rawQueries["FROM Account"] = `{"done":false,"totalSize":2,"nextRecordsUrl":"/services/data/v67.0/query/01g-2000","records":[{"Name":"One"}]}`
	org.handlers["/services/data/v67.0/query/01g-2000"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"done":true,"totalSize":2,"records":[{"Name":"Two"}]}`)
	}
	expectRun(t, 0, "Name\nOne\nTwo\n", "data", "query", "-q", "SELECT Name FROM Account", "-r", "csv")
}

func TestDataQueryCountAndEmptyResults(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.rawQueries["COUNT()"] = `{"done":true,"totalSize":7,"records":[]}`
	org.rawQueries["nosuch"] = `{"done":true,"totalSize":0,"records":[]}`
	// Against an org, sf prints no table for a count: an empty line and
	// the total.
	expectRun(t, 0, "\nTotal number of records retrieved: 7.\n", "data", "query", "-q", "SELECT COUNT() FROM Account")
	expectRun(t, 0, "\nTotal number of records retrieved: 0.\n", "data", "query", "-q", "SELECT Id FROM Account WHERE Name = 'nosuch'")
}

func TestDataQueryUsesToolingAndQueryAll(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"done":true,"totalSize":0,"records":[]}`)
	}
	org.handlers["/services/data/v67.0/queryAll"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"done":true,"totalSize":0,"records":[]}`)
	}
	if _, errOut, status := run("data", "query", "-q", "SELECT Id FROM ApexClass", "--use-tooling-api", "--json"); status != 0 {
		t.Errorf("tooling: status %d %s", status, errOut)
	}
	if _, errOut, status := run("data", "query", "-q", "SELECT Id FROM Account", "--all-rows", "--json"); status != 0 {
		t.Errorf("all rows: status %d %s", status, errOut)
	}
}

func TestDataQueryErrors(t *testing.T) {
	loggedInFakeOrg(t)
	_, errOut, status := run("data", "query")
	want := "Error (2): The following error occurred:\n  Exactly one of the following must be provided: --file, --query\nSee more help with --help\n\n\n"
	if status != 2 || errOut != want {
		t.Errorf("no query: status %d, stderr %q", status, errOut)
	}

	// The fake org answers an unknown query with an INVALID_TYPE error.
	out, _, status := run("data", "query", "-q", "SELECT Id FROM Bogus", "--json")
	var env map[string]interface{}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	data, _ := env["data"].(map[string]interface{})
	if status != 1 || env["name"] != "INVALID_TYPE" || env["code"] != "1" || env["context"] != "DataSoqlQueryCommand" || data["errorCode"] != "INVALID_TYPE" {
		t.Errorf("query error: status %d, envelope %v", status, env)
	}
	_, errOut, _ = run("data", "query", "-q", "SELECT Id FROM Bogus")
	if !strings.HasPrefix(errOut, "Querying Data... done\nError (1): sObject type is not supported") {
		t.Errorf("human query error = %q", errOut)
	}
}

func TestDataQueryReadsQueryFromFileAndWritesOutputFile(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.rawQueries["FROM Account"] = accountQueryResponse
	dir := t.TempDir()
	queryFile := filepath.Join(dir, "query.soql")
	outputFile := filepath.Join(dir, "out.csv")
	if err := os.WriteFile(queryFile, []byte("SELECT Name FROM Account\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectRun(t, 0, "2 records written to "+outputFile+"\n", "data", "query", "--file", queryFile, "-r", "csv", "--output-file", outputFile)
	written, _ := os.ReadFile(outputFile)
	if !strings.HasPrefix(string(written), "Name,Phone,Owner.Username\nAcme,") {
		t.Errorf("output file = %q", written)
	}
}

func TestParseKeyValueSequence(t *testing.T) {
	cases := map[string][]string{
		"Name=Acme Phone=555":             {"Name=Acme", "Phone=555"},
		"Name='Beta Corp' Phone=555":      {"Name=Beta Corp", "Phone=555"},
		`Name="Gamma Inc" Active__c=true`: {"Name=Gamma Inc", "Active__c=true"},
		"Name=O'Brien":                    {"Name=O'Brien"},
		`  Description="it's here"  `:     {"Description=it's here"},
		"Formula__c=a=b":                  {"Formula__c=a=b"},
	}
	for text, want := range cases {
		if got := parseKeyValueSequence(text); !reflect.DeepEqual(got, want) {
			t.Errorf("parseKeyValueSequence(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestParseValuesConvertsBooleansAndJSON(t *testing.T) {
	values, err := parseValues(`Name=Acme Active__c=TRUE Closed__c=false Address__c='{"city":"Reno"}' Count__c=5`)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := encodeJSON(values)
	want := `{"Name":"Acme","Active__c":true,"Closed__c":false,"Address__c":{"city":"Reno"},"Count__c":"5"}`
	if encoded != want {
		t.Errorf("values = %s, want %s", encoded, want)
	}
	if _, err := parseValues("novalue"); err == nil {
		t.Error("a pair without = is accepted")
	}
}

func TestDataRecordCommands(t *testing.T) {
	org := loggedInFakeOrg(t)
	var created, patched string
	var deleted bool
	org.handlers["/services/data/v67.0/sobjects/Account"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		created = string(body)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"001000000000001AAA","success":true,"errors":[]}`)
	}
	org.handlers["/services/data/v67.0/sobjects/Account/001000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			body, _ := io.ReadAll(r.Body)
			patched = string(body)
			w.WriteHeader(http.StatusNoContent)
		case "DELETE":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			io.WriteString(w, `{"attributes":{"type":"Account","url":"/x"},"Id":"001000000000001AAA","Name":"Acme","Phone":null,"IsDeleted":false}`)
		}
	}
	org.rawQueries["FROM Account WHERE Name = 'Acme'"] = `{"done":true,"totalSize":1,"records":[{"Id":"001000000000001AAA"}]}`
	org.rawQueries["FROM Account WHERE Name = 'Dup'"] = `{"done":true,"totalSize":2,"records":[{"Id":"001000000000001AAA"},{"Id":"001000000000002AAA"}]}`
	org.rawQueries["FROM Account WHERE Name = 'None'"] = `{"done":true,"totalSize":0,"records":[]}`

	out, errOut, status := run("data", "create", "record", "-s", "Account", "-v", "Name='Acme Corp' Phone=555")
	if status != 0 || out != "Successfully created record: 001000000000001AAA.\n" || errOut != "Creating record for Account... done\n" {
		t.Errorf("create: status %d, stdout %q, stderr %q", status, out, errOut)
	}
	if created != `{"Name":"Acme Corp","Phone":"555"}` {
		t.Errorf("create request body = %s", created)
	}
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "id": "001000000000001AAA",
    "success": true,
    "errors": []
  },
  "warnings": []
}
`, "data", "create", "record", "--sobject", "Account", "--values", "Name=Acme", "--json")

	out, errOut, status = run("data", "get", "record", "-s", "Account", "-w", "Name=Acme")
	want := "attributes:\n  type: Account\n  url: /x\nId: 001000000000001AAA\nName: Acme\nPhone: null\nIsDeleted: false\n"
	if status != 0 || out != want || errOut != "Getting Record... done\n" {
		t.Errorf("get: status %d, stdout %q, stderr %q", status, out, errOut)
	}

	out, errOut, status = run("data", "update", "record", "-s", "Account", "-i", "001000000000001AAA", "-v", "Phone=777")
	if status != 0 || out != "Successfully updated record: 001000000000001AAA.\n" || errOut != "Updating Record... Success\n" || patched != `{"Phone":"777"}` {
		t.Errorf("update: status %d, stdout %q, stderr %q, body %s", status, out, errOut, patched)
	}

	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "id": "001000000000001AAA",
    "success": true,
    "errors": []
  },
  "warnings": []
}
`, "data", "delete", "record", "-s", "Account", "-w", "Name=Acme", "--json")
	if !deleted {
		t.Error("no DELETE request was made")
	}

	_, errOut, status = run("data", "get", "record", "-s", "Account", "-w", "Name=Dup")
	if status != 1 || errOut != "Getting Record... failed\nError (DataRecordGetMultipleRecords): Name=Dup is not a unique qualifier for Account; 2 records were retrieved.\n\n\n" {
		t.Errorf("multiple matches: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("data", "delete", "record", "-s", "Account", "-w", "Name=None")
	if status != 1 || errOut != "Deleting Record... failed\nError (DataRecordGetNoRecord): No matching record found.\n\n\n" {
		t.Errorf("no match: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("data", "get", "record", "-s", "Account", "-i", "001000000000009AAA")
	if status != 1 || errOut != "Getting Record... failed\nError (NOT_FOUND): The requested resource does not exist\n\n\n" {
		t.Errorf("not found: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("data", "get", "record", "-s", "Account")
	if status != 2 || !strings.Contains(errOut, "Exactly one of the following must be provided: --where, --record-id") {
		t.Errorf("no id: status %d, stderr %q", status, errOut)
	}
}

func TestRecordCommandsReportAPIErrorsAsSfDoes(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/data/v67.0/sobjects/Contact"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `[{"message":"Required fields are missing: [LastName]","errorCode":"REQUIRED_FIELD_MISSING","fields":["LastName"]}]`)
	}
	// A create or update error keeps the exit status as its label, and
	// its data is the error of the API.
	_, errOut, status := run("data", "create", "record", "-s", "Contact", "-v", "FirstName=x")
	if status != 1 || errOut != "Creating record for Contact... Error\nError (1): Required fields are missing: [LastName]\n\n\n" {
		t.Errorf("create: status %d, stderr %q", status, errOut)
	}
	out, _, _ := run("data", "create", "record", "-s", "Contact", "-v", "FirstName=x", "--json")
	for _, want := range []string{`"name": "REQUIRED_FIELD_MISSING"`, `"code": "1"`, `  "data": {
    "message": "Required fields are missing: [LastName]",
    "errorCode": "REQUIRED_FIELD_MISSING",
    "fields": [
      "LastName"
    ]
  },`} {
		if !strings.Contains(out, want) {
			t.Errorf("create json lacks %s:\n%s", want, out)
		}
	}
}
