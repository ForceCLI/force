package sf

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// The layouts and messages in this file follow the sf 2.138.6 plugin
// source.  They were not compared with sf against a Dev Hub.

func TestPackageInstallAndUninstallReports(t *testing.T) {
	org := devHubSetup(t)
	org.handlers["/services/data/v67.0/tooling/sobjects/PackageInstallRequest/0Hf000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"Id":"0Hf000000000001AAA","Status":"SUCCESS","SubscriberPackageVersionKey":"04t000000000001AAA","Errors":null}`)
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/SubscriberPackageVersionUninstallRequest/06y000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"Id":"06y000000000001AAA","Status":"InProgress","SubscriberPackageVersionId":"04t000000000001AAA"}`)
	}
	expectRun(t, 0, "Successfully installed package [04t000000000001AAA]\n", "package", "install", "report", "-i", "0Hf000000000001AAA")
	expectRun(t, 0, "PackageUninstallRequest is currently InProgress.\nYou can continue to query the status using sf package uninstall report -i 06y000000000001AAA -o me@example.com\n",
		"package", "uninstall", "report", "--request-id", "06y000000000001AAA")
	out, _, status := run("force:package:install:report", "-i", "0Hf000000000001AAA", "--json")
	if status != 0 || !strings.Contains(out, `"Status": "SUCCESS"`) {
		t.Errorf("json: status %d\n%s", status, out)
	}
}

func TestPackageUpdateAndDelete(t *testing.T) {
	org := devHubSetup(t)
	var packagePatch, versionPatch string
	org.handlers["/services/data/v67.0/tooling/sobjects/Package2/0Ho000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		packagePatch = string(body)
		w.WriteHeader(http.StatusNoContent)
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/Package2Version/05i000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		versionPatch = string(body)
		w.WriteHeader(http.StatusNoContent)
	}
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"done":true,"totalSize":1,"records":[`+packageVersionRecord+`]}`)
	}

	expectRun(t, 0, "Successfully updated the package. 0Ho000000000001AAA\n", "package", "update", "-p", "billing", "-n", "Billing 2", "-d", "")
	if packagePatch != `{"Name":"Billing 2","Description":""}` {
		t.Errorf("package update = %s", packagePatch)
	}
	expectRun(t, 0, "Successfully deleted the package with ID: 0Ho000000000001AAA\n", "package", "delete", "-p", "billing", "--no-prompt")
	if packagePatch != `{"IsDeprecated":true}` {
		t.Errorf("package delete = %s", packagePatch)
	}
	_, errOut, status := run("package", "delete", "-p", "billing")
	if status != 1 || !strings.Contains(errOut, "Deleted packages can't be recovered.") || !strings.Contains(errOut, "The request to delete this package was canceled") {
		t.Errorf("declined delete: status %d, stderr %q", status, errOut)
	}

	expectRun(t, 0, "Successfully updated the package version. 05i000000000001AAA\n", "package", "version", "update", "-p", "billing@1.0.0-1", "-a", "Summer", "-t", "rc1")
	if versionPatch != `{"Name":"Summer","Tag":"rc1"}` {
		t.Errorf("version update = %s", versionPatch)
	}
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "id": "05i000000000001AAA",
    "success": true,
    "errors": []
  },
  "warnings": []
}
`, "package", "version", "delete", "-p", "04t000000000001AAA", "--json")
	if versionPatch != `{"IsDeprecated":true}` {
		t.Errorf("version delete = %s", versionPatch)
	}
}

func TestPackageVersionCreateRequests(t *testing.T) {
	org := devHubSetup(t)
	var soql string
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if strings.Contains(query, "Package2VersionCreateRequestError") {
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Message":"Apex compile failure"}]}`)
			return
		}
		soql = query
		if strings.Contains(query, "Status = 'Error'") || strings.Contains(query, "08c000000000002AAA") {
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"08c000000000002AAA","Status":"Error","Package2Id":"0Ho000000000001AAA","Package2":{"Name":"Billing"},"Package2VersionId":null,"Package2Version":null,"Tag":null,"Branch":null,"CreatedDate":"2026-01-02T03:04:05.000+0000","CreatedById":"005000000000001AAA"}]}`)
			return
		}
		io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"08c000000000001AAA","Status":"Success","Package2Id":"0Ho000000000001AAA","Package2":{"Name":"Billing"},"Package2VersionId":"05i000000000001AAA","Package2Version":{"SubscriberPackageVersionId":"04t000000000001AAA","HasMetadataRemoved":false,"MajorVersion":1,"MinorVersion":2,"PatchVersion":0,"BuildNumber":3,"CodeCoverage":{"apexCodeCoveragePercentage":96},"HasPassedCodeCoverageCheck":true,"ConvertedFromVersionId":null},"Tag":"rc1","Branch":null,"CreatedDate":"2026-01-02T03:04:05.000+0000","CreatedById":"005000000000001AAA"}]}`)
	}

	expectRun(t, 0, tableText(t, "Package Version Create Request", [][]string{
		{"ID", "08c000000000001AAA"}, {"Status", "Success"}, {"Package Id", "0Ho000000000001AAA"}, {"Package Version Id", "05i000000000001AAA"},
		{"Subscriber Package Version Id", "04t000000000001AAA"}, {"Tag", "rc1"}, {"Branch", ""}, {"Created Date", "2026-01-02 03:04"},
		{"Installation URL", "https://login.salesforce.com/packaging/installPackage.apexp?p0=04t000000000001AAA"}, {"Created By", "005000000000001AAA"},
	}), "package", "version", "create", "report", "-i", "08c000000000001AAA")

	out, errOut, status := run("package", "version", "create", "report", "-i", "08c000000000002AAA")
	if status != 0 || !strings.Contains(out, "│ Status                        │ Error ") || !strings.Contains(out, "\n=== Errors\n") || !strings.Contains(errOut, "Warning: (1) Apex compile failure") {
		t.Errorf("error report: status %d, stderr %q\n%s", status, errOut, out)
	}
	out, _, _ = run("package", "version", "create", "report", "-i", "08c000000000002AAA", "--json")
	if !strings.Contains(out, "\"Error\": [\n        \"Apex compile failure\"\n      ]") || !strings.Contains(out, `"Package2Name": "Billing"`) {
		t.Errorf("error report json:\n%s", out)
	}

	out, _, status = run("package", "version", "create", "list", "-c", "7")
	if status != 0 || !strings.HasPrefix(out, "Package Version Create Requests  [1]\n") ||
		!strings.Contains(out, "│ Id                 │ Status  │ Package Id         │ Package Version Id │ Subscriber Package Version Id │ Tag │ Branch │ Created Date     │ Created By         │") {
		t.Errorf("list: status %d\n%s", status, out)
	}
	if !strings.Contains(soql, "CreatedDate = LAST_N_DAYS:7") {
		t.Errorf("list query = %s", soql)
	}
	if !strings.HasSuffix(soql, " ORDER BY CreatedDate desc") {
		t.Errorf("list query order = %s", soql)
	}
	out, _, _ = run("package", "version", "create", "list", "--json")
	if !strings.Contains(out, `"CodeCoverage": 96,
      "VersionNumber": "1.2.0.3",
      "HasPassedCodeCoverageCheck": true,
      "CreatedBy": "005000000000001AAA",
      "ConvertedFromVersionId": null`) {
		t.Errorf("list json:\n%s", out)
	}
	// sf reads the error messages for the report of one request only.
	out, _, status = run("package", "version", "create", "list", "-s", "Error", "--json")
	if status != 0 || !strings.Contains(soql, "Status = 'Error'") || !strings.Contains(out, `"Error": [],`) || !strings.Contains(out, `"VersionNumber": null`) ||
		!strings.Contains(out, `"ConvertedFromVersionId": "ID Unavailable"`) {
		t.Errorf("status filter: status %d, query %s\n%s", status, soql, out)
	}
}

// tableText returns the output of printTable for a Name/Value table.
func tableText(t *testing.T, title string, rows [][]string) string {
	t.Helper()
	var b strings.Builder
	printTable(&b, title, []string{"Name", "Value"}, rows)
	return b.String()
}

func TestApiRequestGraphql(t *testing.T) {
	org := loggedInFakeOrg(t)
	var payload string
	response := `{"data":{"uiapi":{"query":{"Account":{"edges":[]}}}},"errors":[]}`
	org.handlers["/services/data/v67.0/graphql"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		payload = string(body)
		io.WriteString(w, response)
	}
	out, errOut, status := run("api", "request", "graphql", "--body", "query { uiapi { query { Account { edges { node { Id } } } } } }")
	if status != 0 || !strings.HasPrefix(out, "{\n  \"data\": {\n    \"uiapi\": {") || !strings.HasPrefix(errOut, "Warning: This command is currently in beta.") {
		t.Errorf("status %d, stdout %s, stderr %q", status, out, errOut)
	}
	if payload != `{"query":"query { uiapi { query { Account { edges { node { Id } } } } } }"}` {
		t.Errorf("payload = %s", payload)
	}
	response = `{"data":null,"errors":[{"message":"Validation error"}]}`
	// Like sf, a response with status 200 that reports errors exits with 0.
	if _, _, status := run("api", "request", "graphql", "--body", "query { nope }"); status != 0 {
		t.Errorf("a response with errors exits with status %d, want 0", status)
	}
}

func TestPackage1VersionListAndDisplay(t *testing.T) {
	org := loggedInFakeOrg(t)
	records := `[]`
	var query string
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query().Get("q")
		w.Write([]byte(`{"done":true,"totalSize":1,"records":` + records + `}`))
	}
	_, errOut, status := run("package1", "version", "list")
	if status != 0 || errOut != "Warning: No Results Found\n" {
		t.Errorf("empty list: status %d, stderr %q", status, errOut)
	}
	expectRun(t, 0, "{\n  \"status\": 0,\n  \"result\": [],\n  \"warnings\": [\n    \"No Results Found\"\n  ]\n}\n", "package1", "version", "list", "--json")
	_, errOut, status = run("package1", "version", "display", "-i", "04t000000000001AAA")
	if status != 0 || errOut != "Warning: No results found\n" {
		t.Errorf("empty display: status %d, stderr %q", status, errOut)
	}

	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"list", "-i", "033000000000001"}, "Error (InvalidIdLengthError): Parsing --package-id \n\tThe id must be 18 characters.\nSee more help with --help\n\n\n"},
		{[]string{"list", "-i", "04t000000000001AAA"}, "Error (InvalidPrefixError): Parsing --package-id \n\tThe id must begin with 033.\nSee more help with --help\n\n\n"},
		{[]string{"display"}, "Error (2): The following error occurred:\n  Missing required flag package-version-id\nSee more help with --help\n\n\n"},
		{[]string{"display", "-i", "033000000000001AAA"}, "Error (InvalidPrefixError): Parsing --package-version-id \n\tThe id must begin with 04t.\nSee more help with --help\n\n\n"},
		{[]string{"display", "-i", "04t0000"}, "Error (InvalidIdLengthError): Parsing --package-version-id \n\tThe id must be 15 or 18 characters.\nSee more help with --help\n\n\n"},
	} {
		if _, errOut, status := run(append([]string{"package1", "version"}, c.args...)...); status != 2 || errOut != c.want {
			t.Errorf("%v: status %d, stderr %q", c.args, status, errOut)
		}
	}

	// The table of a result follows the plugin source; the org used for
	// the comparison with sf had no first-generation package.
	records = `[{"Id":"04t000000000001AAA","MetadataPackageId":"033000000000001AAA","Name":"Spring","ReleaseState":"Released","MajorVersion":1,"MinorVersion":2,"PatchVersion":0,"BuildNumber":3}]`
	expectRun(t, 0, `┌─────────────────────────────┬─────────────────────┬────────┬───────────────┬─────────┬──────────────┐
│ Metadata Package Version Id │ Metadata Package Id │ Name   │ Release State │ Version │ Build Number │
├─────────────────────────────┼─────────────────────┼────────┼───────────────┼─────────┼──────────────┤
│ 04t000000000001AAA          │ 033000000000001AAA  │ Spring │ Released      │ 1.2.0   │ 3            │
└─────────────────────────────┴─────────────────────┴────────┴───────────────┴─────────┴──────────────┘

`, "package1", "version", "list", "-i", "033000000000001AAA")
	if !strings.Contains(query, "WHERE MetadataPackageId = '033000000000001AAA' ORDER BY MetadataPackageId, MajorVersion") {
		t.Errorf("query = %s", query)
	}
	out, _, status := run("package1", "version", "display", "-i", "04t000000000001AAA", "--json")
	if status != 0 || !strings.Contains(out, `    {
      "MetadataPackageVersionId": "04t000000000001AAA",
      "MetadataPackageId": "033000000000001AAA",
      "Name": "Spring",
      "ReleaseState": "Released",
      "Version": "1.2.0",
      "BuildNumber": 3
    }`) || !strings.Contains(query, "WHERE id = '04t000000000001AAA'") {
		t.Errorf("display json: status %d, query %s\n%s", status, query, out)
	}
}
