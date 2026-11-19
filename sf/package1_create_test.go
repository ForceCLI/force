package sf

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPackage1VersionCreate(t *testing.T) {
	useTempHome(t)
	org := newFakeOrg(t)
	org.login(t, "--alias", "release", "--set-default")
	interval := package1UploadInterval
	package1UploadInterval = time.Millisecond
	t.Cleanup(func() { package1UploadInterval = interval })

	request := ""
	org.handlers["/services/data/v67.0/tooling/sobjects/PackageUploadRequest"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		request = string(body)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"0HD000000000001AAA","success":true,"errors":[]}`)
	}
	statuses := []string{"QUEUED"}
	org.handlers["/services/data/v67.0/tooling/sobjects/PackageUploadRequest/0HD000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		status := statuses[0]
		if len(statuses) > 1 {
			statuses = statuses[1:]
		}
		errors := "null"
		if status == "ERROR" {
			errors = `{"errors":[{"message":"Apex test failure"},{"message":"Coverage is 10%"}]}`
		}
		io.WriteString(w, `{"attributes":{"type":"PackageUploadRequest"},"Id":"0HD000000000001AAA","MetadataPackageId":"033000000000001AAA","MetadataPackageVersionId":"04t000000000001AAA","Status":"`+status+`","Errors":`+errors+`}`)
	}

	expectRun(t, 0, "PackageUploadRequest has been enqueued. You can query the status using\nsf package1:version:create:get -i 0HD000000000001AAA -o me@example.com\n",
		"package1", "version", "create", "-i", "033000000000001AAA", "-n", "Spring", "-v", "3.2", "-m", "-d", "First", "-k", "secret")
	if request != `{"MetadataPackageId":"033000000000001AAA","VersionName":"Spring","Description":"First","MajorVersion":3,"MinorVersion":2,"IsReleaseVersion":true,"Password":"secret"}` {
		t.Errorf("request = %s", request)
	}

	statuses = []string{"QUEUED", "IN_PROGRESS", "SUCCESS"}
	expectRun(t, 0, "Successfully uploaded package [04t000000000001AAA]\n",
		"package1", "version", "create", "-i", "033000000000001AAA", "-n", "Spring", "-w", "1")
	if request != `{"MetadataPackageId":"033000000000001AAA","VersionName":"Spring"}` {
		t.Errorf("request without options = %s", request)
	}

	statuses = []string{"ERROR"}
	_, errOut, status := run("package1", "version", "create", "-i", "033000000000001AAA", "-n", "Spring", "-w", "1")
	if status != 1 || errOut != "Error (Package1VersionCreateCommandUploadFailureError): Package upload failed.\nApex test failure\nCoverage is 10%\n\n\n" {
		t.Errorf("failed upload: status %d, stderr %q", status, errOut)
	}

	_, errOut, status = run("package1", "version", "create", "-i", "033000000000001AAA", "-n", "Spring", "-v", "3")
	if status != 1 || !strings.Contains(errOut, "Version supplied, 3, is not formatted correctly. Enter in major.minor format, for example, 3.2.") {
		t.Errorf("version: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("package1", "version", "create")
	if status != 2 || !strings.Contains(errOut, "Missing required flag name") || !strings.Contains(errOut, "Missing required flag package-id") {
		t.Errorf("no flags: status %d, stderr %q", status, errOut)
	}
	if _, errOut, status = run("package1", "version", "create", "-i", "0Ho000000000001AAA", "-n", "Spring"); status == 0 || !strings.Contains(errOut, "The id must begin with 033.") {
		t.Errorf("other id: status %d, stderr %q", status, errOut)
	}

	statuses = []string{"IN_PROGRESS"}
	expectRun(t, 0, "PackageUploadRequest is still InProgress. You can query the status using\nsf package1:version:create:get -i 0HD000000000001AAA -o me@example.com\n",
		"package1", "version", "create", "get", "-i", "0HD000000000001AAA")
	statuses = []string{"SUCCESS"}
	out, _, _ := run("force:package1:version:create:get", "-i", "0HD000000000001AAA", "--json")
	if !strings.Contains(out, `"MetadataPackageVersionId": "04t000000000001AAA"`) || !strings.Contains(out, `"Status": "SUCCESS"`) {
		t.Errorf("get json:\n%s", out)
	}
	statuses = []string{"ERROR"}
	_, errOut, status = run("package1", "version", "create", "get", "-i", "0HD000000000001AAA")
	if status != 1 || errOut != "Error (UploadFailureError): Package upload failed.\nApex test failure\nCoverage is 10%\n\n\n" {
		t.Errorf("failed request: status %d, stderr %q", status, errOut)
	}
}

// The command that the status message suggests names the program as it was
// started.
func TestPackage1UploadStatusNamesTheStartedProgram(t *testing.T) {
	previous := binName
	binName = "force --sf"
	t.Cleanup(func() { binName = previous })
	request := orderedFields("Id", "0HD000000000001AAA", "Status", "IN_PROGRESS")
	want := "PackageUploadRequest is still InProgress. You can query the status using\nforce --sf package1:version:create:get -i 0HD000000000001AAA -o me@example.com"
	if got := uploadStatusMessage(request, "me@example.com"); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}
