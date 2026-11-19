package sf

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// package convert follows the source of the sf packaging library.

func TestPackageConvert(t *testing.T) {
	org := devHubSetup(t)
	interval := packageConvertInterval
	packageConvertInterval = time.Millisecond
	t.Cleanup(func() { packageConvertInterval = interval })

	existing := false
	statuses := []string{"Queued"}
	var created []string
	var request map[string]interface{}
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		var records []string
		switch {
		case strings.Contains(query, "FROM Package2 WHERE ConvertedFromPackageId = '033000000000001AAA'"):
			if existing {
				records = []string{`{"Id":"0Ho000000000009AAA","Name":"Mentesa"}`}
			}
		case strings.Contains(query, "FROM SubscriberPackage WHERE Id = '033000000000001AAA'"):
			records = []string{`{"Name":"Mentesa","Description":"Behavioral health","NamespacePrefix":"osbh"}`}
		case strings.Contains(query, "FROM Package2VersionCreateRequestError"):
			records = []string{`{"Message":"The package has no released version"}`}
		case strings.Contains(query, "FROM Package2VersionCreateRequest WHERE Id = '08c000000000001AAA'"):
			status := statuses[0]
			if len(statuses) > 1 {
				statuses = statuses[1:]
			}
			version := "null"
			if status == "Success" {
				version = `{"SubscriberPackageVersionId":"04t000000000007AAA","HasMetadataRemoved":false,"MajorVersion":1,"MinorVersion":19,"PatchVersion":0,"BuildNumber":1,"CodeCoverage":null,"HasPassedCodeCoverageCheck":false,"ConvertedFromVersionId":"04t000000000001AAA"}`
			}
			records = []string{`{"Id":"08c000000000001AAA","Status":"` + status + `","Package2Id":"0Ho000000000009AAA","Package2":{"Name":"Mentesa"},"Package2VersionId":"05i000000000007AAA","Package2Version":` + version + `,"Tag":null,"Branch":null,"CreatedDate":"2026-01-02T03:04:05.000+0000","CreatedById":"005000000000001AAA","IsConversionRequest":true}`}
		case strings.Contains(query, "FROM Package2Version WHERE Id = '05i000000000007AAA'"):
			records = []string{`{"MajorVersion":1,"MinorVersion":19,"PatchVersion":0,"BuildNumber":1}`}
		}
		io.WriteString(w, fmt.Sprintf(`{"done":true,"totalSize":%d,"records":[%s]}`, len(records), strings.Join(records, ",")))
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/Package2"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		created = append(created, string(body))
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"0Ho000000000009AAA","success":true,"errors":[]}`)
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/Package2VersionCreateRequest"] = func(w http.ResponseWriter, r *http.Request) {
		request = nil
		json.NewDecoder(r.Body).Decode(&request)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"08c000000000001AAA","success":true,"errors":[]}`)
	}
	project, _ := os.Getwd()

	// Without --wait the command reports the request.  The Dev Hub has no
	// converted package yet, so one is created.
	_, errOut, status := run("package", "convert", "-p", "033000000000001AAA", "-x")
	if status != 0 || errOut != "Converting Package... Package version creation request status is 'Queued'. Run \"sf package version create report -i 08c000000000001AAA\" to query for status.\n" {
		t.Errorf("no wait: status %d, stderr %q", status, errOut)
	}
	if len(created) != 1 || created[0] != `{"Name":"Mentesa","Description":"Behavioral health","NamespacePrefix":"osbh","ContainerOptions":"Managed","ConvertedFromPackageId":"033000000000001AAA"}` {
		t.Errorf("created packages = %q", created)
	}
	if request["Package2Id"] != "0Ho000000000009AAA" || request["IsConversionRequest"] != true || request["CalculateCodeCoverage"] != false {
		t.Errorf("request = %v", request)
	}
	if _, sent := request["InstallKey"]; sent {
		t.Errorf("an installation key was sent with --installation-key-bypass: %v", request)
	}
	// The version info is a zip of a descriptor, a manifest and the
	// settings.
	archive, err := base64.StdEncoding.DecodeString(request["VersionInfo"].(string))
	if err != nil {
		t.Fatal(err)
	}
	files, err := unzipFiles(archive)
	if err != nil {
		t.Fatal(err)
	}
	if string(files["package2-descriptor.json"]) != "{\n  \"id\": \"0Ho000000000009AAA\"\n}" {
		t.Errorf("descriptor = %s", files["package2-descriptor.json"])
	}
	for _, name := range []string{"package.zip", "settings.zip"} {
		inner, err := unzipFiles(files[name])
		if err != nil || !strings.Contains(string(inner["package.xml"]), "<Package xmlns='http://soap.sforce.com/2006/04/metadata'>") {
			t.Errorf("%s: %v, %s", name, err, inner["package.xml"])
		}
	}
	if data, _ := os.ReadFile(filepath.Join(project, "sfdx-project.json")); !strings.Contains(string(data), `"Mentesa": "0Ho000000000009AAA"`) {
		t.Errorf("sfdx-project.json lacks the package alias:\n%s", data)
	}

	// With --wait the command waits for the version, and the converted
	// package that exists is used.
	existing = true
	statuses = []string{"Queued", "InProgress", "Success"}
	out, errOut, status := run("package", "convert", "-p", "033000000000001AAA", "-k", "secret", "-w", "5", "--verbose", "-a", "1.19.3")
	want := "Converting Package\n" +
		"Request in progress. Will wait a total of 300 more seconds before timing out. Current Status='Queued'.\n" +
		"Request in progress. Will wait a total of 270 more seconds before timing out. Current Status='In Progress'.\n" +
		"Successfully created the package version [08c000000000001AAA]. Subscriber Package Version Id: 04t000000000007AAA\n" +
		"Package Installation URL: https://login.salesforce.com/packaging/installPackage.apexp?p0=04t000000000007AAA\n" +
		"As an alternative, you can use the \"sf package install\" command.\n"
	if status != 0 || out != want || errOut != "" || len(created) != 1 || request["InstallKey"] != "secret" {
		t.Errorf("wait: status %d, packages created %d, request key %v, stderr %q\n%s", status, len(created), request["InstallKey"], errOut, out)
	}
	archive, _ = base64.StdEncoding.DecodeString(request["VersionInfo"].(string))
	files, _ = unzipFiles(archive)
	if !strings.Contains(string(files["package2-descriptor.json"]), `"versionNumber": "1.19.3"`) {
		t.Errorf("descriptor = %s", files["package2-descriptor.json"])
	}
	if data, _ := os.ReadFile(filepath.Join(project, "sfdx-project.json")); !strings.Contains(string(data), `"Mentesa@1.19.0-1-main": "04t000000000007AAA"`) {
		t.Errorf("sfdx-project.json lacks the version alias:\n%s", data)
	}

	statuses = []string{"Error"}
	_, errOut, status = run("package", "convert", "-p", "033000000000001AAA", "-x", "-w", "5")
	if status != 1 || !strings.HasSuffix(errOut, "Error (SfError): The package has no released version\n\n\n") {
		t.Errorf("failed conversion: status %d, stderr %q", status, errOut)
	}

	for _, c := range []struct {
		args   []string
		stderr string
	}{
		{[]string{"-p", "033000000000001AAA"}, "Exactly one of the following must be provided: --installation-key, --installation-key-bypass"},
		{[]string{"-p", "033000000000001AAA", "-x", "-k", "secret"}, "Exactly one of the following must be provided: --installation-key, --installation-key-bypass"},
		{[]string{"-p", "0Ho000000000001AAA", "-x"}, "The id must begin with 033."},
		{[]string{"-x"}, "Missing required flag package"},
	} {
		_, errOut, status := run(append([]string{"package", "convert"}, c.args...)...)
		if status != 2 || !strings.Contains(errOut, c.stderr) {
			t.Errorf("%v: status %d, stderr %q", c.args, status, errOut)
		}
	}
}
