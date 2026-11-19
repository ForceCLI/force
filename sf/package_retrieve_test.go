package sf

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(entry, content)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestPackageVersionRetrieve(t *testing.T) {
	org := devHubSetup(t)
	inner := zipOf(t, map[string]string{
		"package.xml":                  `<?xml version="1.0" encoding="UTF-8"?><Package xmlns="http://soap.sforce.com/2006/04/metadata"><types><members>Greeter</members><name>ApexClass</name></types><version>67.0</version></Package>`,
		"classes/Greeter.cls":          "public class Greeter {}",
		"classes/Greeter.cls-meta.xml": `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`,
	})
	archive := zipOf(t, map[string]string{"package.zip": string(inner), "dependency-ids.json": `{"ids":["04t000000000009AAA"]}`})
	zipField := `"/services/data/v67.0/tooling/sobjects/Package2Version/05i000000000001AAA/DeveloperUsePkgZip"`
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		var records []string
		switch {
		case strings.Contains(query, "FROM Package2Version WHERE SubscriberPackageVersionId = '04t000000000001AAA'"):
			records = []string{`{"Id":"05i000000000001AAA","Package2Id":"0Ho000000000007AAA","SubscriberPackageVersionId":"04t000000000001AAA","MajorVersion":1,"MinorVersion":2,"PatchVersion":0,"BuildNumber":3,"ConvertedFromVersionId":null,"DeveloperUsePkgZip":` + zipField + `}`}
		case strings.Contains(query, "FROM Package2Version WHERE SubscriberPackageVersionId = '04t000000000002AAA'"):
			records = []string{`{"Id":"05i000000000002AAA","Package2Id":"0Ho000000000007AAA","DeveloperUsePkgZip":null}`}
		case strings.Contains(query, "FROM Package2 WHERE Id = '0Ho000000000007AAA'"):
			records = []string{`{"Id":"0Ho000000000007AAA","Name":"Greetings","Description":"Says hello","NamespacePrefix":"greet","ContainerOptions":"Managed"}`}
		}
		io.WriteString(w, fmt.Sprintf(`{"done":true,"totalSize":%d,"records":[%s]}`, len(records), strings.Join(records, ",")))
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/Package2Version/05i000000000001AAA/DeveloperUsePkgZip"] = func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}
	project, _ := os.Getwd()

	_, errOut, status := run("package", "version", "retrieve", "-p", "04t000000000002AAA", "-d", "pkg")
	if status != 1 || !strings.HasPrefix(errOut, "Error (DeveloperUsePkgZipFieldUnavailableError): Can't retrieve package metadata. To use this feature, you must first assign yourself the DownloadPackageVersionZips user permission.") {
		t.Errorf("no zip: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("package", "version", "retrieve", "-p", "04t000000000001AAA", "-d", filepath.Join(project, "pkg"))
	if status != 1 || !strings.Contains(errOut, "The specified directory must be relative to your Salesforce DX project directory") {
		t.Errorf("absolute directory: status %d, stderr %q", status, errOut)
	}

	out, errOut, status := run("package", "version", "retrieve", "-p", "04t000000000001AAA", "-d", "pkg")
	if status != 0 || !strings.Contains(out, "│ FULL NAME │ TYPE      │ PROJECT PATH ") ||
		!strings.Contains(out, "│ Greeter   │ ApexClass │ "+filepath.Join("pkg", "main", "default", "classes", "Greeter.cls")+" ") {
		t.Fatalf("retrieve: status %d, stderr %q\n%s", status, errOut, out)
	}
	if data, err := os.ReadFile(filepath.Join(project, "pkg", "main", "default", "classes", "Greeter.cls")); err != nil || string(data) != "public class Greeter {}" {
		t.Errorf("class file: %q, %v", data, err)
	}
	data, _ := os.ReadFile(filepath.Join(project, "sfdx-project.json"))
	for _, want := range []string{
		`"path": "pkg"`, `"default": false`, `"package": "Greetings"`, `"versionDescription": "Says hello"`, `"versionNumber": "<set version number>"`,
		`"versionName": "<set version name>"`, `"ancestorVersion": "<set ancestor version>"`, `"package": "04t000000000009AAA"`,
		`"Greetings@1.2.0-3-main": "04t000000000001AAA"`,
		// The project already has a namespace, which is kept.
		`"namespace": "acme"`,
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("sfdx-project.json lacks %s:\n%s", want, data)
		}
	}

	// The directory now has files.
	_, errOut, status = run("package", "version", "retrieve", "-p", "04t000000000001AAA", "-d", "pkg")
	if status != 1 || !strings.Contains(errOut, "Error (SourcesDownloadDirectoryNotEmptyError): Can't retrieve package version metadata. The specified directory isn't empty.") {
		t.Errorf("second retrieve: status %d, stderr %q", status, errOut)
	}
}
