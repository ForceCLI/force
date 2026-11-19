package sf

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The layout in this file was compared with sf 2.138.6 against a scratch
// org.

const legacyNamespace = `xmlns="http://www.force.com/2009/06/asyncapi/dataload"`

func serveLegacyBulk(org *fakeOrg) (jobRequest *string, batchContent *string) {
	var request, content string
	job := `<jobInfo ` + legacyNamespace + `><id>750000000000001AAA</id><operation>upsert</operation><object>Account</object><createdById>005000000000001AAA</createdById><createdDate>2026-01-02T03:04:05.000Z</createdDate><systemModstamp>2026-01-02T03:04:05.000Z</systemModstamp><state>Closed</state><concurrencyMode>Parallel</concurrencyMode><contentType>CSV</contentType><numberBatchesQueued>0</numberBatchesQueued><numberBatchesInProgress>0</numberBatchesInProgress><numberBatchesCompleted>1</numberBatchesCompleted><numberBatchesFailed>0</numberBatchesFailed><numberBatchesTotal>1</numberBatchesTotal><numberRecordsProcessed>2</numberRecordsProcessed><numberRetries>0</numberRetries><apiVersion>67.0</apiVersion><numberRecordsFailed>0</numberRecordsFailed><totalProcessingTime>35</totalProcessingTime><apiActiveProcessingTime>20</apiActiveProcessingTime><apexProcessingTime>0</apexProcessingTime></jobInfo>`
	batch := `<batchInfo ` + legacyNamespace + `><id>751000000000001AAA</id><jobId>750000000000001AAA</jobId><state>Completed</state><createdDate>2026-01-02T03:04:05.000Z</createdDate><systemModstamp>2026-01-02T03:04:06.000Z</systemModstamp><numberRecordsProcessed>2</numberRecordsProcessed><numberRecordsFailed>0</numberRecordsFailed></batchInfo>`
	xmlResponse := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`+body)
		}
	}
	org.handlers["/services/async/67.0/job"] = func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		request = string(data)
		xmlResponse(job)(w, r)
	}
	org.handlers["/services/async/67.0/job/750000000000001AAA"] = xmlResponse(job)
	org.handlers["/services/async/67.0/job/750000000000001AAA/batch"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			data, _ := io.ReadAll(r.Body)
			content = string(data)
			xmlResponse(batch)(w, r)
			return
		}
		xmlResponse(`<batchInfoList `+legacyNamespace+`>`+strings.Replace(batch, " "+legacyNamespace, "", 1)+`</batchInfoList>`)(w, r)
	}
	org.handlers["/services/async/67.0/job/750000000000001AAA/batch/751000000000001AAA"] = xmlResponse(batch)
	org.handlers["/services/async/67.0/job/750000000000001AAA/batch/751000000000001AAA/result"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		io.WriteString(w, "\"Id\",\"Success\",\"Created\",\"Error\"\n\"001000000000001AAA\",\"true\",\"false\",\"\"\n\"\",\"false\",\"false\",\"ENTITY_IS_DELETED:entity is deleted:--\"\n")
	}
	return &request, &content
}

func TestLegacyBulkStatus(t *testing.T) {
	org := loggedInFakeOrg(t)
	serveLegacyBulk(org)
	want := "\n=== Job Status\n\n" +
		"id:                      750000000000001AAA\n" +
		"operation:               upsert\n" +
		"object:                  Account\n" +
		"createdById:             005000000000001AAA\n" +
		"createdDate:             2026-01-02T03:04:05.000Z\n" +
		"systemModstamp:          2026-01-02T03:04:05.000Z\n" +
		"state:                   Closed\n" +
		"concurrencyMode:         Parallel\n" +
		"contentType:             CSV\n" +
		"numberBatchesQueued:     0\n" +
		"numberBatchesInProgress: 0\n" +
		"numberBatchesCompleted:  1\n" +
		"numberBatchesFailed:     0\n" +
		"numberBatchesTotal:      1\n" +
		"numberRecordsProcessed:  2\n" +
		"numberRetries:           0\n" +
		"apiVersion:              67.0\n" +
		"numberRecordsFailed:     0\n" +
		"totalProcessingTime:     35\n" +
		"apiActiveProcessingTime: 20\n" +
		"apexProcessingTime:      0\n"
	for _, args := range [][]string{
		{"force", "data", "bulk", "status", "-i", "750000000000001AAA"},
		{"data", "resume", "--job-id", "750000000000001AAA"},
		{"force:data:bulk:status", "-i", "750000000000001AAA"},
	} {
		out, errOut, status := run(args...)
		if status != 0 || out != want || errOut != "Getting Status... done\n" {
			t.Errorf("%v: status %d, stdout\n%s\nstderr %q", args, status, out, errOut)
		}
	}

	out, _, status := run("force", "data", "bulk", "status", "-i", "750000000000001AAA", "-b", "751000000000001AAA")
	if status != 0 || !strings.HasPrefix(out, "\n=== Batch Status\n\nid:                      751000000000001AAA\njobId:                   750000000000001AAA\nstate:                   Completed\n") {
		t.Errorf("batch status: status %d\n%s", status, out)
	}
	_, errOut, status := run("data", "resume", "-i", "750000000000001AAA", "-b", "751000000000009AAA")
	if status != 1 || !strings.Contains(errOut, "Error (NoBatchFound): Unable to find batch 751000000000009AAA for job 750000000000001AAA.") {
		t.Errorf("unknown batch: status %d, stderr %q", status, errOut)
	}
}

func TestLegacyBulkUpsertAndDelete(t *testing.T) {
	org := loggedInFakeOrg(t)
	request, content := serveLegacyBulk(org)
	file := filepath.Join(t.TempDir(), "accounts.csv")
	os.WriteFile(file, []byte("Ext__c,Name\n1,One\n2,Two\n"), 0o644)

	out, errOut, status := run("force", "data", "bulk", "upsert", "-f", file, "-s", "Account", "-i", "Ext__c", "-r")
	wantOut := "Check batch #1’s status with the command:\nsf force data bulk status -i 750000000000001AAA -b 751000000000001AAA\n"
	if status != 0 || out != wantOut || errOut != "Bulk Upsert... done\n" {
		t.Errorf("upsert: status %d, stdout %q, stderr %q", status, out, errOut)
	}
	for _, want := range []string{"<operation>upsert</operation>", "<object>Account</object>", "<externalIdFieldName>Ext__c</externalIdFieldName>", "<concurrencyMode>Serial</concurrencyMode>", "<contentType>CSV</contentType>"} {
		if !strings.Contains(*request, want) {
			t.Errorf("job request lacks %s: %s", want, *request)
		}
	}
	if *content != "Ext__c,Name\n1,One\n2,Two\n" {
		t.Errorf("batch content = %q", *content)
	}

	out, _, status = run("force", "data", "bulk", "delete", "-f", file, "-s", "Account", "-w", "1")
	if status != 0 || !strings.HasPrefix(out, "Will poll the batch statuses every 5 seconds.\nTo fetch the status on your own, press CTRL+C and use the command:\nsf force data bulk status -i 750000000000001AAA -b 751000000000001AAA\n"+
		"Batch #1 queued (Batch ID: 751000000000001AAA). \n\n\n=== Batch #1\n\n=== Upsert errors\n\nENTITY_IS_DELETED:entity is deleted:--\n=== Batch Status\n\nid:                      751000000000001AAA\n") ||
		!strings.Contains(out, "apexProcessingTime:      0\n\n=== Job Status\n\nid:") {
		t.Errorf("delete with wait: status %d\n%s", status, out)
	}
	if !strings.Contains(*request, "<operation>delete</operation>") {
		t.Errorf("delete job request = %s", *request)
	}
	out, _, status = run("force", "data", "bulk", "delete", "-f", file, "-s", "Account", "--json")
	if status != 0 || !strings.Contains(out, "\"result\": [\n    {\n      \"$\": {\n        \"xmlns\": \"http://www.force.com/2009/06/asyncapi/dataload\"\n      },\n      \"id\": \"751000000000001AAA\",\n      \"jobId\": \"750000000000001AAA\",") ||
		!strings.Contains(out, "\"apexProcessingTime\": \"0\"") {
		t.Errorf("delete json: status %d\n%s", status, out)
	}
	_, errOut, status = run("force", "data", "bulk", "upsert", "-f", file, "-s", "Account")
	if status != 2 || !strings.Contains(errOut, "Missing required flag external-id") {
		t.Errorf("missing external id: status %d, stderr %q", status, errOut)
	}
}
