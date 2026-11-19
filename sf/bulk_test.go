package sf

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The output formats in this file were captured from sf 2.138.6.

// fakeBulk makes the fake org answer Bulk API 2.0 requests.  An ingest job
// reports processed and failed record counts; a query job returns csv.
type fakeBulk struct {
	created   map[string]interface{}
	uploaded  string
	state     string
	processed int
	failed    int
	csv       string
}

func serveBulk(org *fakeOrg) *fakeBulk {
	bulk := &fakeBulk{state: "JobComplete", processed: 2}
	const ingest = "/services/data/v67.0/jobs/ingest"
	const query = "/services/data/v67.0/jobs/query"
	info := func() string {
		encoded, _ := json.Marshal(map[string]interface{}{
			"id": "750000000000001AAA", "state": bulk.state, "object": "Account", "operation": "insert",
			"numberRecordsProcessed": bulk.processed, "numberRecordsFailed": bulk.failed,
		})
		return string(encoded)
	}
	org.handlers[ingest] = func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&bulk.created)
		io.WriteString(w, `{"id":"750000000000001AAA","state":"Open"}`)
	}
	org.handlers[ingest+"/750000000000001AAA/batches"] = func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		bulk.uploaded = string(data)
		w.WriteHeader(http.StatusCreated)
	}
	org.handlers[ingest+"/750000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			io.WriteString(w, `{"id":"750000000000001AAA","state":"UploadComplete"}`)
			return
		}
		io.WriteString(w, info())
	}
	org.handlers[ingest+"/750000000000001AAA/successfulResults"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "sf__Id,sf__Created,Name\n001000000000001AAA,true,One\n")
	}
	org.handlers[ingest+"/750000000000001AAA/failedResults"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "sf__Id,sf__Error,Name\n,REQUIRED_FIELD_MISSING,Two\n")
	}
	org.handlers[ingest+"/750000000000001AAA/unprocessedrecords"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Name\n")
	}
	org.handlers[query] = func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&bulk.created)
		io.WriteString(w, `{"id":"750000000000002AAA","state":"UploadComplete"}`)
	}
	org.handlers[query+"/750000000000002AAA"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":"750000000000002AAA","state":"`+bulk.state+`","numberRecordsProcessed":2}`)
	}
	org.handlers[query+"/750000000000002AAA/results"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		io.WriteString(w, bulk.csv)
	}
	return bulk
}

func bulkSetup(t *testing.T) (*fakeOrg, *fakeBulk, string) {
	t.Helper()
	org := loggedInFakeOrg(t)
	bulk := serveBulk(org)
	previous := bulkPollInterval
	bulkPollInterval = time.Millisecond
	t.Cleanup(func() { bulkPollInterval = previous })
	file := filepath.Join(t.TempDir(), "accounts.csv")
	if err := os.WriteFile(file, []byte("Name\nOne\nTwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return org, bulk, file
}

var elapsedPattern = regexp.MustCompile(`\d+ms`)

func TestDataImportBulkWaitsAndReportsCounts(t *testing.T) {
	org, bulk, file := bulkSetup(t)
	out, errOut, status := run("data", "import", "bulk", "-f", file, "-s", "Account", "-w", "2")
	if status != 0 {
		t.Fatalf("status %d, stderr %s", status, errOut)
	}
	url := org.URL + "//lightning/setup/AsyncApiJobStatus/page?address=%2F750000000000001AAA"
	want := "\n ───────────────── Importing data ─────────────────\n\n" +
		" ✔ Creating ingest job Nms\n" +
		" ✔ Processing the job Nms\n" +
		"   ▸ Processed records: 2\n" +
		"   ▸ Successful records: 2\n" +
		"   ▸ Failed records: 0\n" +
		"\n Status: JobComplete\n" +
		" Job Id:\n" +
		" " + ("750000000000001AAA (" + url + ")")[:79] + "\n" +
		" " + ("750000000000001AAA (" + url + ")")[79:] + "\n" +
		" Elapsed Time: Nms\n\n"
	if got := elapsedPattern.ReplaceAllString(out, "Nms"); got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}
	if bulk.created["object"] != "Account" || bulk.created["operation"] != "insert" || bulk.uploaded != "Name\nOne\nTwo\n" {
		t.Errorf("job request %v, uploaded %q", bulk.created, bulk.uploaded)
	}

	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "jobId": "750000000000001AAA",
    "processedRecords": 2,
    "successfulRecords": 2,
    "failedRecords": 0
  },
  "warnings": []
}
`, "data", "import", "bulk", "--file", file, "--sobject", "Account", "--wait", "2", "--json")
}

func TestDataBulkWithoutWaitRecordsTheJobForResume(t *testing.T) {
	_, _, file := bulkSetup(t)
	out, _, status := run("data", "import", "bulk", "-f", file, "-s", "Account")
	if status != 0 || !strings.Contains(out, " ───────────── Importing data (async) ─────────────\n") ||
		!strings.Contains(out, " ◼ Processing the job\n") || !strings.Contains(out, " Status: UploadComplete\n") ||
		!strings.HasSuffix(out, "Run \"sf data import resume --job-id 750000000000001AAA\" to resume the operation.\n") {
		t.Errorf("status %d\n%s", status, out)
	}
	cache, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".sf", "bulk-data-import-cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	var entries map[string]map[string]string
	json.Unmarshal(cache, &entries)
	entry := entries["750000000000001AAA"]
	if entry["jobId"] != "750000000000001AAA" || entry["username"] != "me@example.com" || entry["apiVersion"] != "67.0" || entry["timestamp"] == "" {
		t.Errorf("cache entry = %v", entry)
	}

	for _, args := range [][]string{
		{"data", "import", "resume", "--job-id", "750000000000001AAA", "--json"},
		{"data", "import", "resume", "--use-most-recent", "--json"},
	} {
		out, _, status = run(args...)
		if status != 0 || !strings.Contains(out, `"processedRecords": 2`) {
			t.Errorf("%v: status %d\n%s", args, status, out)
		}
	}
	_, errOut, status := run("data", "import", "resume", "--job-id", "750000000000009AAA")
	if status != 1 || !strings.Contains(errOut, "Could not find a cache entry for job ID 750000000000009AAA.") {
		t.Errorf("unknown job: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("data", "update", "resume", "--use-most-recent")
	if status != 1 || !strings.Contains(errOut, "Could not load a most recent cache entry") {
		t.Errorf("empty cache: status %d, stderr %q", status, errOut)
	}
}

func TestDataBulkFailureCases(t *testing.T) {
	_, bulk, file := bulkSetup(t)

	bulk.processed = 0
	out, errOut, status := run("data", "delete", "bulk", "-f", file, "-s", "Account", "-w", "1")
	if status != 1 || !strings.Contains(out, " ✘ Processing the job ") || !strings.Contains(out, "   ▸ Processed records: ✘\n") {
		t.Errorf("no records: status %d\n%s", status, out)
	}
	wantErr := "Error (NoProcessedRecordsError): Job finished successfully but it didn't process any record.\n\n\n" +
		"Try this:\n\nCheck that the provided CSV file is valid.\n" +
		"View the job in the org: \"sf org open -o me@example.com --path '/lightning/setup/AsyncApiJobStatus/page?address=%2F750000000000001AAA'\".\n\n"
	if errOut != wantErr {
		t.Errorf("stderr = %q\nwant     %q", errOut, wantErr)
	}
	if bulk.created["operation"] != "delete" {
		t.Errorf("operation = %v", bulk.created["operation"])
	}

	bulk.processed, bulk.failed = 2, 1
	out, _, status = run("data", "update", "bulk", "-f", file, "-s", "Account", "-w", "1", "--json")
	if status != 1 || !strings.Contains(out, `"name": "FailedRecordDetailsError"`) ||
		!strings.Contains(out, `"message": "Job finished being processed but failed to process 1 records."`) ||
		!strings.Contains(out, `Get the job results by running: \"sf data bulk results -o me@example.com --job-id 750000000000001AAA\".`) ||
		!strings.Contains(out, `"context": "DataUpdateBulk"`) {
		t.Errorf("failed records: status %d\n%s", status, out)
	}

	bulk.state = "InProgress"
	_, errOut, status = run("data", "import", "bulk", "-f", file, "-s", "Account", "-w", "0")
	if status != 1 || !strings.Contains(errOut, "The operation timed out after 0 minutes.\nRun \"sf data import resume --job-id 750000000000001AAA\" to resume it.") {
		t.Errorf("timeout: status %d, stderr %q", status, errOut)
	}
}

func TestDataUpsertAndHardDeleteRequests(t *testing.T) {
	_, bulk, file := bulkSetup(t)
	_, missing, missingStatus := run("data", "upsert", "bulk", "-f", file, "-s", "Account")
	if errOut, status := missing, missingStatus; status != 2 || !strings.Contains(errOut, "Missing required flag external-id") {
		t.Errorf("missing external id: status %d, stderr %q", status, errOut)
	}
	// The upsert and delete commands report the job and its records.
	out, errOut, status := run("data", "upsert", "bulk", "-f", file, "-s", "Account", "-i", "Ext__c", "--column-delimiter", "PIPE", "--line-ending", "CRLF", "--json")
	if status != 0 || !strings.Contains(out, `"jobInfo": {`) || !strings.Contains(out, `    "records": {
      "successfulResults": [
        {
          "sf__Id": "001000000000001AAA",
          "sf__Created": "true",
          "Name": "One"
        }
      ],
      "failedResults": [
        {
          "sf__Id": "",
          "sf__Error": "REQUIRED_FIELD_MISSING",
          "Name": "Two"
        }
      ],
      "unprocessedRecords": []
    }`) {
		t.Fatalf("upsert: status %d, %s\n%s", status, errOut, out)
	}
	if bulk.created["operation"] != "upsert" || bulk.created["externalIdFieldName"] != "Ext__c" || bulk.created["columnDelimiter"] != "PIPE" || bulk.created["lineEnding"] != "CRLF" {
		t.Errorf("upsert request = %v", bulk.created)
	}
	if _, _, status := run("data", "delete", "bulk", "-f", file, "-s", "Account", "--hard-delete", "--json"); status != 0 {
		t.Fatalf("hard delete: status %d", status)
	}
	if bulk.created["operation"] != "hardDelete" {
		t.Errorf("hard delete operation = %v", bulk.created["operation"])
	}
	_, errOut, status = run("data", "import", "bulk", "-f", file, "-s", "Account", "--column-delimiter", "COLON")
	if status != 1 || !strings.Contains(errOut, "Expected --column-delimiter=COLON to be one of: BACKQUOTE, CARET, COMMA, PIPE, SEMICOLON, TAB") {
		t.Errorf("bad delimiter: status %d, stderr %q", status, errOut)
	}
}

func TestDataExportBulk(t *testing.T) {
	_, bulk, _ := bulkSetup(t)
	bulk.csv = "\"Id\",\"Name\"\n\"001000000000001AAA\",\"One\"\n\"001000000000002AAA\",\"Two, Inc.\"\n"
	dir := t.TempDir()
	csvFile := filepath.Join(dir, "export.csv")
	out, _, status := run("data", "export", "bulk", "-q", "SELECT Id, Name FROM Account", "--output-file", csvFile, "-w", "2")
	if status != 0 || !strings.Contains(out, " ───────────────── Exporting data ─────────────────\n") ||
		!strings.Contains(out, " ✔ exporting records ") || !strings.Contains(out, "(​http") ||
		!strings.HasSuffix(out, "2 records written to "+csvFile+"\n") {
		t.Errorf("csv export: status %d\n%s", status, out)
	}
	if written, _ := os.ReadFile(csvFile); string(written) != bulk.csv {
		t.Errorf("csv file = %q", written)
	}
	if bulk.created["operation"] != "query" || bulk.created["query"] != "SELECT Id, Name FROM Account" {
		t.Errorf("query job request = %v", bulk.created)
	}

	jsonFile := filepath.Join(dir, "export.json")
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "totalSize": 2,
    "filePath": "`+jsonFile+`"
  },
  "warnings": []
}
`, "data", "export", "bulk", "--query", "SELECT Id, Name FROM Account", "--output-file", jsonFile, "-r", "json", "-w", "2", "--all-rows", "--json")
	written, _ := os.ReadFile(jsonFile)
	want := "[\n  {\"Id\":\"001000000000001AAA\",\"Name\":\"One\"},\n  {\"Id\":\"001000000000002AAA\",\"Name\":\"Two, Inc.\"}\n]"
	if string(written) != want {
		t.Errorf("json file = %q\nwant        %q", written, want)
	}
	if bulk.created["operation"] != "queryAll" {
		t.Errorf("--all-rows operation = %v", bulk.created["operation"])
	}

	// Without --wait the job is recorded and "data export resume" writes
	// the file.
	resumeFile := filepath.Join(dir, "later.csv")
	out, _, status = run("data", "export", "bulk", "-q", "SELECT Id FROM Account", "--output-file", resumeFile)
	if status != 0 || !strings.HasSuffix(out, "Run \"sf data export resume --job-id 750000000000002AAA\" to get the latest status and results.\n") ||
		!regexp.MustCompile(` ✔ creating query job \d+ms\n ✔ done \d+ms\n\n Status: ⠁\n`).MatchString(out) {
		t.Errorf("async export: status %d\n%s", status, out)
	}
	if _, err := os.Stat(resumeFile); !os.IsNotExist(err) {
		t.Errorf("the output file exists before the job is resumed: %v", err)
	}
	out, _, status = run("data", "export", "resume", "-i", "750000000000002AAA")
	if status != 0 || !strings.HasSuffix(out, "2 records written to "+resumeFile+"\n") ||
		!strings.Contains(durationPattern.ReplaceAllString(out, "T"), " ✔ checking query job status T\n ✔ exporting records T\n\n Status: JobComplete\n Elapsed Time: T\n") {
		t.Errorf("export resume: status %d\n%s", status, out)
	}
	if written, _ := os.ReadFile(resumeFile); !bytes.Equal(written, []byte(bulk.csv)) {
		t.Errorf("resumed file = %q", written)
	}
}

func TestDataBulkResults(t *testing.T) {
	_, bulk, _ := bulkSetup(t)
	chdir(t, t.TempDir())
	bulk.processed, bulk.failed = 2, 1
	expectRun(t, 0, "Status: JobComplete\nOperation: insert\nObject: Account\n\n"+
		"Processed records: 2\nFailed records: 1\n\nSuccessful records: 1\n\n"+
		"Saved successful results to 750000000000001AAA-success-records.csv\n"+
		"Saved failed results to 750000000000001AAA-failed-records.csv\n",
		"data", "bulk", "results", "-i", "750000000000001AAA")
	if failed, _ := os.ReadFile("750000000000001AAA-failed-records.csv"); !strings.Contains(string(failed), "REQUIRED_FIELD_MISSING") {
		t.Errorf("failed records file = %q", failed)
	}
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "processedRecords": 2,
    "successfulRecords": 1,
    "failedRecords": 1,
    "status": "JobComplete",
    "operation": "insert",
    "object": "Account",
    "successFilePath": "750000000000001AAA-success-records.csv",
    "failedFilePath": "750000000000001AAA-failed-records.csv"
  },
  "warnings": []
}
`, "data", "bulk", "results", "--job-id", "750000000000001AAA", "--json")

	bulk.state = "InProgress"
	_, errOut, status := run("data", "bulk", "results", "-i", "750000000000001AAA")
	if status != 1 || !strings.Contains(errOut, "Job hasn't finished being processed yet.") {
		t.Errorf("in progress: status %d, stderr %q", status, errOut)
	}
	bulk.processed = 0
	_, errOut, status = run("data", "bulk", "results", "-i", "750000000000001AAA")
	if status != 1 || !strings.Contains(errOut, "Unable to get results because the job processed 0 records.") {
		t.Errorf("no records: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("data", "bulk", "results", "-i", "750000000000009AAA")
	if status != 1 || !strings.Contains(errOut, "Error (InvalidIdError): Can't find a bulk job with ID 750000000000009AAA.") {
		t.Errorf("unknown job: status %d, stderr %q", status, errOut)
	}
}
