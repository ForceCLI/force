package sf

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// The tables of these commands follow the sf plugin source: the Dev Hub
// used for the comparison with sf did not have snapshots enabled.  The
// error for that case was compared.

func TestOrgSnapshotCommands(t *testing.T) {
	org := devHubSetup(t)
	record := map[string]interface{}{
		"attributes": map[string]interface{}{"type": "OrgSnapshot"}, "Id": "0Oo000000000001AAA", "SnapshotName": "nightly", "Description": nil, "Status": "Active",
		"SourceOrg": "00D000000000001", "CreatedDate": "2026-01-02T15:04:05.000+0000", "LastModifiedDate": "2026-01-03T09:00:00.000+0000", "ExpirationDate": "2026-02-01", "Error": nil,
	}
	org.queries["FROM OrgSnapshot Order by CreatedDate"] = []map[string]interface{}{record}
	org.queries["FROM OrgSnapshot WHERE SnapshotName = 'nightly'"] = []map[string]interface{}{record}
	org.queries["FROM OrgSnapshot WHERE Id = '0Oo000000000001AAA'"] = []map[string]interface{}{record}
	org.queries["FROM OrgSnapshot WHERE SnapshotName = 'missing'"] = nil
	expectRun(t, 0, `Org Snapshots [1]
┌────────────────────┬───────────────┬────────┬─────────────────┬──────────────────────┬──────────────────────┬─────────────────┐
│ Id                 │ Snapshot Name │ Status │ Source Org Id   │ Created Date         │ Last Modified Date   │ Expiration Date │
├────────────────────┼───────────────┼────────┼─────────────────┼──────────────────────┼──────────────────────┼─────────────────┤
│ 0Oo000000000001AAA │ nightly       │ Active │ 00D000000000001 │ 01/02/2026, 03:04 PM │ 01/03/2026, 09:00 AM │ 2/1/2026        │
└────────────────────┴───────────────┴────────┴─────────────────┴──────────────────────┴──────────────────────┴─────────────────┘

`, "org", "list", "snapshot")
	out, _, status := run("org", "get", "snapshot", "-s", "0Oo000000000001AAA")
	if status != 0 || !strings.Contains(out, "│ Snapshot Name      │ nightly ") || !strings.Contains(out, "│ Created Date       │ 01/02/2026, 03:04 PM │") || strings.Contains(out, "│ Error ") {
		t.Errorf("get: status %d\n%s", status, out)
	}
	_, errOut, status := run("org", "get", "snapshot", "-s", "missing")
	if status != 1 || errOut != "Error (SingleRecordQuery_NoRecords): No snapshot found with the given name or id: missing\n\n\n" {
		t.Errorf("missing: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("org", "get", "snapshot")
	if status != 2 || !strings.Contains(errOut, "Missing required flag snapshot") {
		t.Errorf("no name: status %d, stderr %q", status, errOut)
	}
	deleted := ""
	org.handlers["/services/data/v67.0/sobjects/OrgSnapshot/0Oo000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		deleted = r.Method
		w.WriteHeader(http.StatusNoContent)
	}
	expectRun(t, 0, "Successfully deleted snapshot nightly.\n", "org", "delete", "snapshot", "-s", "nightly", "-p")
	if deleted != "DELETE" {
		t.Errorf("delete request method = %q", deleted)
	}
}

func TestOrgListSnapshotWhenTheDevHubHasNoSnapshots(t *testing.T) {
	org := devHubSetup(t)
	org.handlers["/services/data/v67.0/query"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`[{"message":"sObject type 'OrgSnapshot' is not supported.","errorCode":"INVALID_TYPE"}]`))
	}
	_, errOut, status := run("org", "list", "snapshot")
	if status != 1 || errOut != "Error (1): Scratch Org Snapshots isn't enabled for your Dev Hub.\n\n\n" {
		t.Errorf("status %d, stderr %q", status, errOut)
	}
	out, _, _ := run("org", "list", "snapshot", "--json")
	for _, want := range []string{`"name": "INVALID_TYPE"`, `"message": "Scratch Org Snapshots isn't enabled for your Dev Hub."`, `"message": "sObject type 'OrgSnapshot' is not supported.",
    "errorCode": "INVALID_TYPE"`, `"code": "1"`} {
		if !strings.Contains(out, want) {
			t.Errorf("json lacks %s:\n%s", want, out)
		}
	}
}

func TestOrgCreateSnapshot(t *testing.T) {
	org := devHubSetup(t)
	request := ""
	org.handlers["/services/data/v67.0/sobjects/OrgSnapshot"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		request = string(body)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"0Oo000000000001AAA","success":true,"errors":[]}`)
	}
	org.queries["FROM OrgSnapshot WHERE Id = '0Oo000000000001AAA'"] = []map[string]interface{}{{
		"attributes": map[string]interface{}{"type": "OrgSnapshot"}, "Id": "0Oo000000000001AAA", "SnapshotName": "nightly", "Description": "After the seed data", "Status": "In Progress",
		"SourceOrg": "00D000000000000", "CreatedDate": "2026-01-02T15:04:05.000+0000", "LastModifiedDate": "2026-01-02T15:04:05.000+0000", "ExpirationDate": nil, "Error": nil,
	}}
	// The alias of an authorized org gives its org Id.
	out, errOut, status := run("org", "create", "snapshot", "-o", "hub", "-n", "nightly", "-d", "After the seed data")
	if status != 0 || !strings.Contains(out, "│ Snapshot Name      │ nightly ") || !strings.Contains(out, "│ Status             │ In Progress ") {
		t.Errorf("create: status %d, stderr %q\n%s", status, errOut, out)
	}
	if request != `{"SourceOrg":"00D000000000000","Description":"After the seed data","SnapshotName":"nightly","Content":"metadatadata"}` {
		t.Errorf("request = %s", request)
	}
	if _, _, status = run("force:org:snapshot:create", "-o", "00D000000000009", "-n", "nightly", "--json"); status != 0 || request != `{"SourceOrg":"00D000000000009","SnapshotName":"nightly","Content":"metadatadata"}` {
		t.Errorf("org id: status %d, request %s", status, request)
	}
	_, errOut, status = run("org", "create", "snapshot", "-o", "nobody", "-n", "nightly")
	if status != 2 || !strings.Contains(errOut, "No org found for nobody") {
		t.Errorf("unknown org: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("org", "create", "snapshot")
	if status != 2 || !strings.Contains(errOut, "Missing required flag name") || !strings.Contains(errOut, "Missing required flag source-org") {
		t.Errorf("no flags: status %d, stderr %q", status, errOut)
	}
	org.handlers["/services/data/v67.0/sobjects/OrgSnapshot"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `[{"message":"The requested resource does not exist","errorCode":"NOT_FOUND"}]`)
	}
	_, errOut, status = run("org", "create", "snapshot", "-o", "hub", "-n", "nightly")
	if status != 1 || !strings.Contains(errOut, "Scratch Org Snapshots isn't enabled for your Dev Hub.") {
		t.Errorf("not enabled: status %d, stderr %q", status, errOut)
	}
}
