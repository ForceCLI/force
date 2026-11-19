package sf

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackagePushUpgradeCommands(t *testing.T) {
	org := devHubSetup(t)
	var queries []string
	patched := ""
	status := "Failed"
	org.handlers["/services/data/v67.0/query"] = func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		queries = append(queries, query)
		switch {
		case strings.Contains(query, "033000000000009AAA") || strings.Contains(query, "0DV000000000009AAA"):
			io.WriteString(w, `{"done":true,"totalSize":0,"records":[]}`)
		case strings.Contains(query, "FROM PackagePushJob") && strings.Contains(query, "Status = 'Failed'"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"attributes":{"type":"AggregateResult"},"expr0":1}]}`)
		case strings.Contains(query, "FROM PackagePushJob") && strings.Contains(query, "Status = 'Succeeded'"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"attributes":{"type":"AggregateResult"},"expr0":2}]}`)
		case strings.Contains(query, "FROM PackagePushJob"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"attributes":{"type":"AggregateResult"},"expr0":3}]}`)
		case strings.Contains(query, "from PackagePushError"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"ErrorMessage":"Apex compile failure"}]}`)
		case strings.HasPrefix(query, "SELECT Id, Status FROM PackagePushRequest"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"0DV000000000001AAA","Status":"`+status+`"}]}`)
		default:
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"attributes":{"type":"PackagePushRequest"},"Id":"0DV000000000001AAA","PackageVersionId":"04t000000000001AAA",`+
				`"PackageVersion":{"Name":"Spring","MajorVersion":1,"MinorVersion":2,"MetadataPackageId":"033000000000001AAA","MetadataPackage":{"Name":"Billing","NamespacePrefix":"acme"}},`+
				`"Status":"Failed","ScheduledStartTime":"2026-01-02T03:04:05.000+0000","StartTime":null,"EndTime":null,"DurationSeconds":7,"IsMigration":false}]}`)
		}
	}
	org.handlers["/services/data/v67.0/sobjects/packagepushrequest/0DV000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		patched = r.Method + " " + string(body)
		w.WriteHeader(http.StatusNoContent)
	}

	// Compared with sf: a package without push upgrade requests.
	out, errOut, code := run("package", "push-upgrade", "list", "-p", "033000000000009AAA")
	if code != 0 || out != "" || errOut != "Warning: No results found\n" {
		t.Errorf("empty list: status %d, stdout %q, stderr %q", code, out, errOut)
	}

	queries = nil
	expectRun(t, 0, `┌────────────────────┬────────────────────┬──────────────────────┬────────────────────────┬────────┬──────────────────────────────┬────────────┬──────────┬────────────────────┬────────────────────────┬───────────────────────────┐
│ Id                 │ Package Version Id │ Package Version Name │ Package Version Number │ Status │ Scheduled Start Time         │ Start Time │ End Time │ Num Orgs Scheduled │ Num Orgs Upgraded Fail │ Num Orgs Upgraded Success │
├────────────────────┼────────────────────┼──────────────────────┼────────────────────────┼────────┼──────────────────────────────┼────────────┼──────────┼────────────────────┼────────────────────────┼───────────────────────────┤
│ 0DV000000000001AAA │ 04t000000000001AAA │ Spring               │ 1.2                    │ Failed │ 2026-01-02T03:04:05.000+0000 │            │          │ 3                  │ 1                      │ 2                         │
└────────────────────┴────────────────────┴──────────────────────┴────────────────────────┴────────┴──────────────────────────────┴────────────┴──────────┴────────────────────┴────────────────────────┴───────────────────────────┘

`, "package", "push-upgrade", "list", "-p", "033000000000001AAA", "-s", "Failed", "--show-push-migrations-only")
	if len(queries) == 0 || !strings.HasSuffix(queries[0], "FROM PackagePushRequest WHERE PackageVersion.MetadataPackageId = '033000000000001AAA' AND Status = 'Failed' AND IsMigration = true") {
		t.Errorf("list queries = %q", queries)
	}
	_, errOut, code = run("package", "push-upgrade", "list", "-p", "033000000000001AAA", "-l", "0")
	if code != 1 || errOut != "Error (1): Invalid value for --scheduled-last-days. It must be a positive integer.\n\n\n" {
		t.Errorf("days: status %d, stderr %q", code, errOut)
	}

	out, errOut, code = run("package", "push-upgrade", "report", "-i", "0DV000000000001AAA")
	for _, want := range []string{"│ Package Name             │ Billing ", "│ Package Version          │ 1.2 ", "│ Namespace                │ acme ", "│ Duration Seconds         │ 7 ",
		"│ # Orgs Scheduled         │ 3 ", "│ # Orgs Upgrade Succeeded │ 2 ", "│ # Orgs Upgrade Failed    │ 1 ", "\n=== Errors\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
	if code != 0 || !strings.Contains(errOut, "Warning: (1) Apex compile failure") {
		t.Errorf("report: status %d, stderr %q", code, errOut)
	}
	out, errOut, code = run("package", "push-upgrade", "report", "-i", "0DV000000000009AAA", "--json")
	if code != 0 || !strings.Contains(out, `"result": null`) || !strings.Contains(out, `"No results found"`) {
		t.Errorf("missing report: status %d, stderr %q\n%s", code, errOut, out)
	}

	_, errOut, code = run("package", "push-upgrade", "abort", "-i", "0DV000000000001AAA")
	if code != 1 || !strings.Contains(errOut, "Error (1): Can’t abort package push upgrade request with status 'Failed'.") || patched != "" {
		t.Errorf("abort of a failed request: status %d, stderr %q, request %q", code, errOut, patched)
	}
	status = "Pending"
	expectRun(t, 0, "Scheduled push upgrade ID [0DV000000000001AAA] was cancelled.\n", "package", "push-upgrade", "abort", "-i", "0DV000000000001AAA")
	if patched != `PATCH {"Status":"Canceled"}` {
		t.Errorf("abort request = %q", patched)
	}
	_, errOut, code = run("package", "push-upgrade", "abort", "-i", "0DV000000000009AAA")
	if code != 1 || !strings.Contains(errOut, "The specified push upgrade ID isn’t valid.") {
		t.Errorf("abort of a missing request: status %d, stderr %q", code, errOut)
	}
	if _, errOut, code = run("package", "push-upgrade", "abort", "-i", "033000000000001AAA"); code == 0 || !strings.Contains(errOut, "The id must begin with 0DV.") {
		t.Errorf("abort with another id: status %d, stderr %q", code, errOut)
	}
}

func TestPackagePushUpgradeSchedule(t *testing.T) {
	org := devHubSetup(t)
	bulk := serveBulk(org)
	var requests []string
	org.handlers["/services/data/v67.0/sobjects/packagepushrequest/"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, r.Method+" "+string(body))
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"0DV000000000001AAA","success":true,"errors":[]}`)
	}
	org.handlers["/services/data/v67.0/sobjects/packagepushrequest/0DV000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, r.Method+" "+string(body))
		w.WriteHeader(http.StatusNoContent)
	}
	org.handlers["/services/data/v67.0/jobs/ingest/750000000000001AAA/failedResults"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "\"sf__Id\",\"sf__Error\",PackagePushRequestId,SubscriberOrganizationKey\n")
	}

	expectRun(t, 0, "Push upgrade has been scheduled. To check the status of this push upgrade, use push upgrade request ID [0DV000000000001AAA] with either \"package push-upgrade list\" or \"package push-upgrade report\".\n"+
		"Orgs scheduled for push upgrade: {00D000000000001, 00D000000000002}\n",
		"package", "push-upgrade", "schedule", "-p", "04t000000000001AAA", "-l", "00D000000000001, 00D000000000002", "-t", "2026-12-01T00:00:00Z", "--migrate-to-2gp")
	if len(requests) != 2 || requests[0] != `POST {"PackageVersionId":"04t000000000001AAA","ScheduledStartTime":"2026-12-01T00:00:00Z","IsMigration":true}` || requests[1] != `PATCH {"Status":"Pending"}` {
		t.Errorf("requests = %q", requests)
	}
	if bulk.created["object"] != "PackagePushJob" || bulk.created["operation"] != "insert" ||
		bulk.uploaded != "PackagePushRequestId,SubscriberOrganizationKey\n0DV000000000001AAA,00D000000000001\n0DV000000000001AAA,00D000000000002\n" {
		t.Errorf("bulk job %v, data %q", bulk.created, bulk.uploaded)
	}

	// An org file keeps the lines that are 15 character org Ids.
	file := filepath.Join(t.TempDir(), "orgs.csv")
	os.WriteFile(file, []byte("00D000000000003\r\nnot an org\n\n00D000000000004AAA\n"), 0o644)
	out, _, status := run("package", "push-upgrade", "schedule", "-p", "04t000000000001AAA", "-f", file, "--json")
	if status != 0 || !strings.Contains(out, "\"PushRequestId\": \"0DV000000000001AAA\",\n    \"Status\": \"Pending\"") || !strings.HasSuffix(bulk.uploaded, "\n0DV000000000001AAA,00D000000000003\n") {
		t.Errorf("org file: status %d, data %q\n%s", status, bulk.uploaded, out)
	}

	_, errOut, status := run("package", "push-upgrade", "schedule", "-p", "04t000000000001AAA")
	if status != 1 || !strings.HasPrefix(errOut, "Error (SfError): Can’t schedule the package push upgrade. You must specify either a list of orgs, or a file") {
		t.Errorf("no orgs: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("package", "push-upgrade", "schedule", "-p", "04t000000000001AAA", "-l", "00D000000000001", "-f", file)
	if status != 2 || !strings.Contains(errOut, "--org-list=00D000000000001 cannot also be provided when using --org-file") {
		t.Errorf("both: status %d, stderr %q", status, errOut)
	}

	// A job with failed rows fails the command and leaves a log file.
	project, _ := os.Getwd()
	org.handlers["/services/data/v67.0/jobs/ingest/750000000000001AAA/failedResults"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "\"sf__Id\",\"sf__Error\",PackagePushRequestId,SubscriberOrganizationKey\n\"\",\"INVALID_OPERATION:The org is not a subscriber\",0DV000000000001AAA,00D000000000001\n")
	}
	deleted := false
	previous := org.handlers["/services/data/v67.0/jobs/ingest/750000000000001AAA"]
	org.handlers["/services/data/v67.0/jobs/ingest/750000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		previous(w, r)
	}
	log := filepath.Join(project, "job_errors", "push_request_0DV000000000001AAA_errors.log")
	_, errOut, status = run("package", "push-upgrade", "schedule", "-p", "04t000000000001AAA", "-l", "00D000000000001")
	if status != 1 || errOut != "Error (SfError): Push upgrade failed, job errors have been written to file: "+log+"\n\n\n" || !deleted {
		t.Errorf("failed job: status %d, deleted %v, stderr %q", status, deleted, errOut)
	}
	if data, _ := os.ReadFile(log); string(data) != `Job 1 Error:"INVALID_OPERATION:The org is not a subscriber"` {
		t.Errorf("log = %q", data)
	}
}
