package sf

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
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

// The convert output, the queued deploy output and the progress display
// layout in this file were captured from sf 2.138.6.  The tables printed
// after a finished deploy or retrieve follow the sf plugin source.

const greeterMeta = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <apiVersion>67.0</apiVersion>\n    <status>Active</status>\n</ApexClass>\n"

// newSourceProject creates a project with two Apex classes and a custom
// object with one field, in source format, and makes it the working
// directory.
func newSourceProject(t *testing.T, home string) string {
	t.Helper()
	project := newProject(t, home)
	files := map[string]string{
		"force-app/main/default/classes/Greeter.cls":                                "public class Greeter {}",
		"force-app/main/default/classes/Greeter.cls-meta.xml":                       greeterMeta,
		"force-app/main/default/classes/GreeterTest.cls":                            "@IsTest private class GreeterTest {}",
		"force-app/main/default/classes/GreeterTest.cls-meta.xml":                   greeterMeta,
		"force-app/main/default/objects/Invoice__c/Invoice__c.object-meta.xml":      "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <label>Invoice</label>\n    <pluralLabel>Invoices</pluralLabel>\n</CustomObject>\n",
		"force-app/main/default/objects/Invoice__c/fields/Amount__c.field-meta.xml": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <fullName>Amount__c</fullName>\n    <label>Amount</label>\n    <type>Number</type>\n</CustomField>\n",
	}
	for name, content := range files {
		path := filepath.Join(project, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(project, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`), 0o644)
	return project
}

func TestProjectConvertSourceWritesMetadataFormat(t *testing.T) {
	home := useTempHome(t)
	newSourceProject(t, home)
	out := filepath.Join(home, "mdapi_out")
	expectRun(t, 0, "Source was successfully converted to Metadata API format and written to the location: "+out+"\n",
		"project", "convert", "source", "-r", "force-app", "-d", out)

	manifest, err := os.ReadFile(filepath.Join(out, "package.xml"))
	if err != nil {
		t.Fatal(err)
	}
	want := `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>Greeter</members>
        <members>GreeterTest</members>
        <name>ApexClass</name>
    </types>
    <types>
        <members>Invoice__c.Amount__c</members>
        <name>CustomField</name>
    </types>
    <types>
        <members>Invoice__c</members>
        <name>CustomObject</name>
    </types>
    <version>67.0</version>
</Package>
`
	if string(manifest) != want {
		t.Errorf("package.xml =\n%s\nwant\n%s", manifest, want)
	}
	for _, name := range []string{"classes/Greeter.cls", "classes/Greeter.cls-meta.xml", "classes/GreeterTest.cls"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("%s is missing: %v", name, err)
		}
	}
	// The object and its field are composed into one metadata format file.
	object, err := os.ReadFile(filepath.Join(out, "objects", "Invoice__c.object"))
	if err != nil || !strings.Contains(string(object), "<fullName>Amount__c</fullName>") || !strings.Contains(string(object), "<label>Invoice</label>") {
		t.Errorf("composed object = %q, err = %v", object, err)
	}

	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "location": "`+filepath.Join(home, "one")+`"
  },
  "warnings": [
    "The \"force source convert\" command has been deprecated. Use \"project convert source\" instead."
  ]
}
`, "force:source:convert", "--metadata", "ApexClass:Greeter", "--output-dir", filepath.Join(home, "one"), "--json")
	if _, err := os.Stat(filepath.Join(home, "one", "classes", "GreeterTest.cls")); !os.IsNotExist(err) {
		t.Errorf("--metadata ApexClass:Greeter also converted GreeterTest: %v", err)
	}
}

func TestProjectConvertMdapiWritesSourceFormat(t *testing.T) {
	home := useTempHome(t)
	newSourceProject(t, home)
	mdapi := filepath.Join(home, "mdapi_out")
	if _, errOut, status := run("project", "convert", "source", "-m", "ApexClass", "-d", mdapi); status != 0 {
		t.Fatalf("convert source: status %d, %s", status, errOut)
	}
	out, errOut, status := run("project", "convert", "mdapi", "-r", mdapi, "-d", filepath.Join(home, "src_out"))
	want := `┌───────┬─────────────┬───────────┬──────────────────────────────────────────────────────────┐
│ STATE │ FULL NAME   │ TYPE      │ PROJECT PATH                                             │
├───────┼─────────────┼───────────┼──────────────────────────────────────────────────────────┤
│ Add   │ Greeter     │ ApexClass │ ../src_out/main/default/classes/Greeter.cls-meta.xml     │
│ Add   │ Greeter     │ ApexClass │ ../src_out/main/default/classes/Greeter.cls              │
│ Add   │ GreeterTest │ ApexClass │ ../src_out/main/default/classes/GreeterTest.cls-meta.xml │
│ Add   │ GreeterTest │ ApexClass │ ../src_out/main/default/classes/GreeterTest.cls          │
└───────┴─────────────┴───────────┴──────────────────────────────────────────────────────────┘

`
	if status != 0 || out != want {
		t.Errorf("status %d, stdout\n%s\nwant\n%s", status, out, want)
	}
	if errOut != "Converting 2 metadata components... done\n" {
		t.Errorf("stderr = %q", errOut)
	}
	out, _, _ = run("project", "convert", "mdapi", "--root-dir", mdapi, "--output-dir", filepath.Join(home, "src_out"), "--json")
	if !strings.Contains(out, "\"filePath\": \"../src_out/main/default/classes/Greeter.cls-meta.xml\",\n      \"state\": \"Unchanged\"") {
		t.Errorf("a second conversion does not report unchanged files:\n%s", out)
	}
	_, errOut, status = run("project", "convert", "mdapi", "-r", filepath.Join(home, "nosuch"))
	if want := "Error (2): Parsing --root-dir \n\tNo directory found at " + filepath.Join(home, "nosuch") + "\nSee more help with --help\n\n\n"; status != 2 || errOut != want {
		t.Errorf("missing root dir: status %d, stderr %q", status, errOut)
	}
}

// convertProject writes a project with two classes, a trigger and an object
// with one field, and converts it to metadata format in the directory md.
func convertProject(t *testing.T) {
	t.Helper()
	useTempHome(t)
	os.WriteFile("sfdx-project.json", []byte(`{"packageDirectories":[{"path":"force-app","default":true},{"path":"out"}],"sourceApiVersion":"60.0"}`), 0o644)
	base := filepath.Join("force-app", "main", "default")
	write := func(path, content string) {
		t.Helper()
		path = filepath.Join(base, path)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll("out", 0o755)
	head := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"
	for _, class := range []string{"Alpha", "Beta"} {
		write("classes/"+class+".cls", "public class "+class+" {}\n")
		write("classes/"+class+".cls-meta.xml", head+"<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <apiVersion>60.0</apiVersion>\n    <status>Active</status>\n</ApexClass>\n")
	}
	write("triggers/Tr.trigger", "trigger Tr on Account (before insert) {}\n")
	write("triggers/Tr.trigger-meta.xml", head+"<ApexTrigger xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <apiVersion>60.0</apiVersion>\n    <status>Active</status>\n</ApexTrigger>\n")
	write("objects/Invoice__c/Invoice__c.object-meta.xml", head+"<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <label>Invoice</label>\n    <pluralLabel>Invoices</pluralLabel>\n</CustomObject>\n")
	write("objects/Invoice__c/fields/Amount__c.field-meta.xml", head+"<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <fullName>Amount__c</fullName>\n    <label>Amount</label>\n    <precision>18</precision>\n    <scale>2</scale>\n    <type>Number</type>\n</CustomField>\n")
	if _, errOut, status := run("project", "convert", "source", "-d", "md", "-n", "My <Pkg>"); status != 0 {
		t.Fatalf("convert source: status %d %s", status, errOut)
	}
}

func TestProjectConvertSourceWritesThePackageName(t *testing.T) {
	convertProject(t)
	_, errOut, status := run("project", "convert", "source", "-r", "nosuch", "-d", "x")
	if status != 2 || errOut != "Error (2): Parsing --root-dir \n\tNo directory found at nosuch\nSee more help with --help\n\n\n" {
		t.Errorf("missing root dir: status %d, stderr %q", status, errOut)
	}
	manifest := readFile(t, filepath.Join("md", "package.xml"))
	if !strings.HasSuffix(manifest, "    <version>60.0</version>\n    <fullName>My &lt;Pkg&gt;</fullName>\n</Package>\n") {
		t.Errorf("package.xml = %s", manifest)
	}
}

func TestProjectConvertMdapiSelectsComponents(t *testing.T) {
	convertProject(t)
	os.WriteFile("one.xml", []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Package xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <types>\n        <members>Alpha</members>\n        <name>ApexClass</name>\n    </types>\n    <version>60.0</version>\n</Package>\n"), 0o644)

	out, errOut, status := run("project", "convert", "mdapi", "-r", "md", "-d", "out", "-x", "one.xml")
	want := `┌───────┬───────────┬───────────┬─────────────────────────────────────────────┐
│ STATE │ FULL NAME │ TYPE      │ PROJECT PATH                                │
├───────┼───────────┼───────────┼─────────────────────────────────────────────┤
│ Add   │ Alpha     │ ApexClass │ out/main/default/classes/Alpha.cls-meta.xml │
│ Add   │ Alpha     │ ApexClass │ out/main/default/classes/Alpha.cls          │
└───────┴───────────┴───────────┴─────────────────────────────────────────────┘

`
	if status != 0 || out != want || errOut != "Converting 1 metadata components... done\n" {
		t.Errorf("--manifest: status %d, stdout\n%s\nstderr %q", status, out, errOut)
	}
	if _, err := os.Stat(filepath.Join("out", "main", "default", "classes", "Beta.cls")); err == nil {
		t.Error("--manifest converted a class the manifest does not name")
	}

	os.RemoveAll(filepath.Join("out", "main"))
	out, errOut, _ = run("project", "convert", "mdapi", "-r", "md", "-d", "out", "-m", "ApexClass:Beta", "-m", "ApexTrigger")
	if errOut != "Converting 2 metadata components... done\n" || !strings.Contains(out, "│ Add   │ Beta      │ ApexClass   │ out/main/default/classes/Beta.cls-meta.xml    │") ||
		!strings.Contains(out, "│ Add   │ Tr        │ ApexTrigger │ out/main/default/triggers/Tr.trigger          │") || strings.Contains(out, "Alpha") || strings.Contains(out, "Invoice__c") {
		t.Errorf("--metadata: stdout\n%s\nstderr %q", out, errOut)
	}

	os.RemoveAll(filepath.Join("out", "main"))
	out, _, _ = run("project", "convert", "mdapi", "-r", "md", "-d", "out", "-p", "md/classes/Alpha.cls", "-p", "md/triggers", "--json")
	for _, path := range []string{"out/main/default/classes/Alpha.cls-meta.xml", "out/main/default/classes/Alpha.cls", "out/main/default/triggers/Tr.trigger"} {
		if !strings.Contains(out, `"filePath": "`+path+`"`) {
			t.Errorf("--metadata-dir did not convert %s:\n%s", path, out)
		}
	}
	if strings.Contains(out, "Beta") || strings.Contains(out, "Invoice__c") {
		t.Errorf("--metadata-dir converted a component outside the paths:\n%s", out)
	}

	// sf lists a field of a decomposed object as a CustomField, and lists
	// the directory of the object.
	os.RemoveAll(filepath.Join("out", "main"))
	out, errOut, _ = run("project", "convert", "mdapi", "-r", "md", "-d", "out", "-m", "CustomObject:Invoice__c")
	want = `┌───────┬──────────────────────┬──────────────┬─────────────────────────────────────────────────────────────────────┐
│ STATE │ FULL NAME            │ TYPE         │ PROJECT PATH                                                        │
├───────┼──────────────────────┼──────────────┼─────────────────────────────────────────────────────────────────────┤
│ Add   │ Invoice__c.Amount__c │ CustomField  │ out/main/default/objects/Invoice__c/fields/Amount__c.field-meta.xml │
│ Add   │ Invoice__c           │ CustomObject │ out/main/default/objects/Invoice__c/Invoice__c.object-meta.xml      │
│ Add   │ Invoice__c           │ CustomObject │ out/main/default/objects/Invoice__c                                 │
└───────┴──────────────────────┴──────────────┴─────────────────────────────────────────────────────────────────────┘

`
	if out != want || errOut != "Converting 1 metadata components... done\n" {
		t.Errorf("object: stdout\n%s\nstderr %q", out, errOut)
	}

	for _, c := range []struct {
		args   []string
		status int
		want   string
	}{
		{[]string{"-x", "nosuch.xml"}, 2, "Error (2): Parsing --manifest \n\tNo file found at nosuch.xml\nSee more help with --help\n\n\n"},
		{[]string{"-p", "nosuch"}, 1, "Error (InvalidFlagPath): The metadatapath command flag specifies an invalid path: nosuch\nNo such file or directory.\n\n\n"},
		{[]string{"-m", "ApexClass:Nope"}, 1, "Error (SfError): No results to format\n\n\n"},
		{[]string{"-m", "ApexClass", "-x", "one.xml"}, 2, "Error (2): The following error occurred:\n  --manifest=one.xml cannot also be provided when using --metadata\nSee more help with --help\n\n\n"},
		{[]string{"-p", "md/classes", "-x", "one.xml"}, 2, "Error (2): The following error occurred:\n  --manifest=one.xml cannot also be provided when using --metadata-dir\nSee more help with --help\n\n\n"},
		{[]string{"-p", "md/classes", "-m", "ApexClass:Alpha"}, 2, "Error (2): The following error occurred:\n  --metadata=ApexClass:Alpha cannot also be provided when using --metadata-dir\nSee more help with --help\n\n\n"},
	} {
		_, errOut, status := run(append([]string{"project", "convert", "mdapi", "-r", "md", "-d", "out"}, c.args...)...)
		if status != c.status || errOut != c.want {
			t.Errorf("%v: status %d, stderr %q, want %q", c.args, status, errOut, c.want)
		}
	}
	_, errOut, status = run("project", "convert", "mdapi", "-r", "md", "-d", "out", "-m", "Bogus")
	if status != 1 || !strings.HasPrefix(errOut, "Error (RegistryError): Missing metadata type definition in registry for id 'Bogus'.\n\n\nTry this:\n\nDid you mean one of the following types? [Bot]\n\nConfirm the metadata type name is correct.") {
		t.Errorf("unknown type: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("project", "convert", "mdapi", "-r", "one.xml", "-d", "out")
	if status != 2 || errOut != "Error (2): Parsing --root-dir \n\tone.xml exists but is not a directory\nSee more help with --help\n\n\n" {
		t.Errorf("root dir that is a file: status %d, stderr %q", status, errOut)
	}
}

// fakeDeploy makes the fake org answer Metadata API deploy and retrieve
// requests.
type fakeDeploy struct {
	requests     []string
	deployed     map[string]string // files of the last deployed zip
	options      string
	statusResult string // content of <result> for checkDeployStatus
	retrieveZip  []byte
	// retrieveDetails is the content the retrieve result has between
	// <done> and <id>: file properties.  retrieveMessages follows <id>.
	retrieveDetails, retrieveMessages string
}

func soapBody(content string) string {
	return strings.Replace(soapEnvelope, "%s", content, 1)
}

var soapOperationPattern = regexp.MustCompile(`(?s)<(?:[A-Za-z0-9]+:)?Body[^>]*>\s*<(?:[A-Za-z0-9]+:)?([A-Za-z]+)`)

var zipFilePattern = regexp.MustCompile(`(?s)<[A-Za-z:]*[zZ]ipFile>(.*?)</[A-Za-z:]*[zZ]ipFile>`)

func serveMetadata(org *fakeOrg) *fakeDeploy {
	fake := &fakeDeploy{}
	org.handlers["/services/Soap/m/67.0"] = func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body := string(data)
		w.Header().Set("Content-Type", "text/xml")
		// The operation is the first element of the SOAP body.
		operation := ""
		if match := soapOperationPattern.FindStringSubmatch(body); match != nil {
			operation = match[1]
		}
		is := func(name string) bool { return operation == name }
		switch {
		case is("checkStatus"):
			// lib waits for a retrieve with this operation.
			fake.requests = append(fake.requests, "checkStatus")
			io.WriteString(w, soapBody("<checkStatusResponse><result><done>true</done><id>09S000000000001AAA</id><state>Completed</state></result></checkStatusResponse>"))
		case is("checkDeployStatus"):
			fake.requests = append(fake.requests, "checkDeployStatus")
			io.WriteString(w, soapBody("<checkDeployStatusResponse><result>"+fake.statusResult+"</result></checkDeployStatusResponse>"))
		case is("deployRecentValidation"):
			fake.requests = append(fake.requests, "deployRecentValidation")
			io.WriteString(w, soapBody("<deployRecentValidationResponse><result>0Af000000000002AAA</result></deployRecentValidationResponse>"))
		case is("cancelDeploy"):
			fake.requests = append(fake.requests, "cancelDeploy")
			io.WriteString(w, soapBody("<cancelDeployResponse><result><done>false</done><id>0Af000000000001AAA</id></result></cancelDeployResponse>"))
		case is("checkRetrieveStatus"):
			fake.requests = append(fake.requests, "checkRetrieveStatus")
			io.WriteString(w, soapBody("<checkRetrieveStatusResponse><result><done>true</done>"+fake.retrieveDetails+"<id>09S000000000001AAA</id>"+fake.retrieveMessages+"<status>Succeeded</status><success>true</success><zipFile>"+base64.StdEncoding.EncodeToString(fake.retrieveZip)+"</zipFile></result></checkRetrieveStatusResponse>"))
		case is("retrieve"):
			fake.requests = append(fake.requests, "retrieve")
			fake.options = body
			io.WriteString(w, soapBody("<retrieveResponse><result><done>false</done><id>09S000000000001AAA</id><state>Queued</state></result></retrieveResponse>"))
		case is("deploy"):
			fake.requests = append(fake.requests, "deploy")
			fake.options = body
			fake.deployed = map[string]string{}
			if match := zipFilePattern.FindStringSubmatch(body); match != nil {
				if archive, err := base64.StdEncoding.DecodeString(strings.TrimSpace(match[1])); err == nil {
					if reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive))); err == nil {
						for _, file := range reader.File {
							content, _ := file.Open()
							data, _ := io.ReadAll(content)
							content.Close()
							fake.deployed[file.Name] = string(data)
						}
					}
				}
			}
			io.WriteString(w, soapBody("<deployResponse><result><done>false</done><id>0Af000000000001AAA</id><state>Queued</state></result></deployResponse>"))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
	return fake
}

const deploySucceeded = `<checkOnly>false</checkOnly><completedDate>2026-01-02T03:04:06.000Z</completedDate><createdBy>005000000000001</createdBy><createdByName>Ada Lovelace</createdByName><createdDate>2026-01-02T03:04:05.000Z</createdDate>` +
	`<details>` +
	`<componentSuccesses><changed>false</changed><componentType>ApexClass</componentType><created>true</created><createdDate>2026-01-02T03:04:06.000Z</createdDate><deleted>false</deleted><fileName>classes/Greeter.cls</fileName><fullName>Greeter</fullName><id>01p000000000001AAA</id><success>true</success></componentSuccesses>` +
	`<componentSuccesses><changed>true</changed><componentType></componentType><created>false</created><deleted>false</deleted><fileName>package.xml</fileName><fullName>package.xml</fullName><success>true</success></componentSuccesses>` +
	`<runTestResult><numFailures>0</numFailures><numTestsRun>0</numTestsRun><totalTime>0.0</totalTime></runTestResult>` +
	`</details>` +
	`<done>true</done><id>0Af000000000001AAA</id><ignoreWarnings>false</ignoreWarnings><lastModifiedDate>2026-01-02T03:04:06.000Z</lastModifiedDate>` +
	`<numberComponentErrors>0</numberComponentErrors><numberComponentsDeployed>1</numberComponentsDeployed><numberComponentsTotal>1</numberComponentsTotal>` +
	`<numberTestErrors>0</numberTestErrors><numberTestsCompleted>0</numberTestsCompleted><numberTestsTotal>0</numberTestsTotal>` +
	`<rollbackOnError>true</rollbackOnError><runTestsEnabled>false</runTestsEnabled><status>Succeeded</status><success>true</success>`

const deployFailed = `<checkOnly>false</checkOnly>` +
	`<details><componentFailures><changed>false</changed><columnNumber>12</columnNumber><componentType>ApexClass</componentType><created>false</created><deleted>false</deleted><fileName>classes/Greeter.cls</fileName><fullName>Greeter</fullName><lineNumber>3</lineNumber><problem>Unexpected token '}'.</problem><problemType>Error</problemType><success>false</success></componentFailures></details>` +
	`<done>true</done><id>0Af000000000001AAA</id><numberComponentErrors>1</numberComponentErrors><numberComponentsDeployed>0</numberComponentsDeployed><numberComponentsTotal>1</numberComponentsTotal>` +
	`<numberTestErrors>0</numberTestErrors><numberTestsCompleted>0</numberTestsCompleted><numberTestsTotal>0</numberTestsTotal><status>Failed</status><success>false</success>`

func deploySetup(t *testing.T) (*fakeOrg, *fakeDeploy, string) {
	t.Helper()
	org := loggedInFakeOrg(t)
	fake := serveMetadata(org)
	fake.statusResult = deploySucceeded
	project := newSourceProject(t, os.Getenv("HOME"))
	previous := deployPollInterval
	deployPollInterval = time.Millisecond
	t.Cleanup(func() { deployPollInterval = previous })
	return org, fake, project
}

var durationPattern = regexp.MustCompile(`\d+(\.\d+)?m?s`)

func TestProjectDeployStartFromSourceDirectory(t *testing.T) {
	_, fake, project := deploySetup(t)
	out, errOut, status := run("project", "deploy", "start", "-d", "force-app/main/default/classes", "-w", "5")
	if status != 0 {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	want := `
 ─────────────── Deploying Metadata ───────────────

 Deploying v67.0 metadata to me@example.com using the v67.0 SOAP API.

 ✔ Preparing T
 ◯ Waiting for the org to respond - Skipped
 ✔ Deploying Metadata T
   ▸ Components: 1/1 (100%)
 ◯ Running Tests - Skipped
 ◯ Updating Source Tracking - Skipped
 ✔ Done T

 Status: Succeeded
 Deploy ID: 0Af000000000001AAA
 Target Org: me@example.com
 Elapsed Time: T


Deployed Source
┌─────────┬─────────┬───────────┬─────────────────────────────────────────────────────┐
│ State   │ Name    │ Type      │ Path                                                │
├─────────┼─────────┼───────────┼─────────────────────────────────────────────────────┤
│ Created │ Greeter │ ApexClass │ force-app/main/default/classes/Greeter.cls          │
│ Created │ Greeter │ ApexClass │ force-app/main/default/classes/Greeter.cls-meta.xml │
└─────────┴─────────┴───────────┴─────────────────────────────────────────────────────┘


`
	if got := durationPattern.ReplaceAllString(out, "T"); got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}

	// The zip sent to the org is in metadata format with a manifest.
	for _, name := range []string{"classes/Greeter.cls", "classes/Greeter.cls-meta.xml", "classes/GreeterTest.cls", "package.xml"} {
		if _, ok := fake.deployed[name]; !ok {
			t.Errorf("the deployed zip lacks %s; it has %v", name, keys(fake.deployed))
		}
	}
	if _, ok := fake.deployed["objects/Invoice__c.object"]; ok {
		t.Error("deploying the classes directory also deployed the object")
	}
	if !strings.Contains(fake.deployed["package.xml"], "<members>Greeter</members>") || !strings.Contains(fake.deployed["package.xml"], "<version>67.0</version>") {
		t.Errorf("package.xml = %s", fake.deployed["package.xml"])
	}
	for _, option := range []string{"<checkOnly>false</checkOnly>", "<rollbackOnError>true</rollbackOnError>", "<singlePackage>true</singlePackage>"} {
		if !strings.Contains(fake.options, option) {
			t.Errorf("deploy options lack %s", option)
		}
	}

	out, _, status = run("project", "deploy", "start", "--source-dir", "force-app", "--json")
	var env struct {
		Status int
		Result map[string]interface{}
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	result := env.Result
	if status != 0 || result["status"] != "Succeeded" || result["done"] != true || result["checkOnly"] != false ||
		result["numberComponentsDeployed"] != float64(1) || result["createdByName"] != "Ada Lovelace" {
		t.Errorf("status %d, result %v", status, result)
	}
	details := result["details"].(map[string]interface{})
	if len(details["componentSuccesses"].([]interface{})) != 2 || len(details["componentFailures"].([]interface{})) != 0 {
		t.Errorf("details = %v", details)
	}
	files := result["files"].([]interface{})
	first := files[0].(map[string]interface{})
	if len(files) != 2 || first["filePath"] != filepath.Join(project, "force-app", "main", "default", "classes", "Greeter.cls") || first["state"] != "Created" {
		t.Errorf("files = %v", files)
	}
	if result["zipFileCount"] != float64(6) || !strings.Contains(result["deployUrl"].(string), "asyncId%3D0Af000000000001AAA") {
		t.Errorf("zipFileCount %v, deployUrl %v", result["zipFileCount"], result["deployUrl"])
	}
	if _, ok := fake.deployed["objects/Invoice__c.object"]; !ok {
		t.Errorf("deploying force-app does not include the composed object; zip has %v", keys(fake.deployed))
	}
}

func keys(m map[string]string) []string {
	var names []string
	for name := range m {
		names = append(names, name)
	}
	return names
}

// sf wraps the subtitle of the progress display at 79 characters.  The space
// at the break starts the next line only after a line of exactly 79
// characters, which the subtitle for admin@aer.local is and the subtitle for
// me@example.com is not.
func TestWrapLineKeepsTheSpaceOnlyAfterAFullLine(t *testing.T) {
	full := wrapLine("Deploying <version specified in manifest> metadata to admin@aer.local using the v67.0 SOAP API.", 79)
	if len(full) != 2 || len(full[0]) != 79 || full[1] != " v67.0 SOAP API." {
		t.Errorf("after a full line: %q", full)
	}
	short := wrapLine("Deploying v62.0 metadata to test-xzdyiek98fwg@example.com using the v68.0 SOAP API.", 79)
	if len(short) != 2 || short[0] != "Deploying v62.0 metadata to test-xzdyiek98fwg@example.com using the v68.0 SOAP" || short[1] != "API." {
		t.Errorf("after a shorter line: %q", short)
	}
}

func TestProjectDeployStartByMetadataManifestAndMetadataDir(t *testing.T) {
	home := os.Getenv("HOME")
	_, fake, _ := deploySetup(t)
	home = os.Getenv("HOME")

	if _, errOut, status := run("project", "deploy", "start", "-m", "ApexClass:GreeterTest", "--json"); status != 0 {
		t.Fatalf("--metadata: status %d %s", status, errOut)
	}
	if _, ok := fake.deployed["classes/GreeterTest.cls"]; !ok || len(fake.deployed) != 3 {
		t.Errorf("--metadata ApexClass:GreeterTest deployed %v", keys(fake.deployed))
	}

	manifest := filepath.Join(home, "package.xml")
	os.WriteFile(manifest, []byte(`<?xml version="1.0" encoding="UTF-8"?><Package xmlns="http://soap.sforce.com/2006/04/metadata"><types><members>*</members><name>ApexClass</name></types><version>67.0</version></Package>`), 0o644)
	if _, errOut, status := run("project", "deploy", "start", "--manifest", manifest, "--json"); status != 0 {
		t.Fatalf("--manifest: status %d %s", status, errOut)
	}
	if len(fake.deployed) != 5 {
		t.Errorf("--manifest with ApexClass * deployed %v", keys(fake.deployed))
	}

	mdapi := filepath.Join(home, "mdapi")
	os.MkdirAll(filepath.Join(mdapi, "classes"), 0o755)
	os.WriteFile(filepath.Join(mdapi, "classes", "Other.cls"), []byte("public class Other {}"), 0o644)
	os.WriteFile(filepath.Join(mdapi, "package.xml"), []byte("<Package/>"), 0o644)
	out, _, status := run("project", "deploy", "start", "--metadata-dir", mdapi, "-l", "RunSpecifiedTests", "-t", "OtherTest", "--ignore-errors", "--ignore-warnings")
	if status != 0 || !strings.Contains(out, " Deploying <version specified in manifest> metadata to me@example.com using the\n v67.0 SOAP API.\n") {
		t.Errorf("--metadata-dir: status %d\n%s", status, out)
	}
	if fake.deployed["classes/Other.cls"] != "public class Other {}" || fake.deployed["package.xml"] != "<Package/>" {
		t.Errorf("--metadata-dir deployed %v", fake.deployed)
	}
	for _, option := range []string{"<testLevel>RunSpecifiedTests</testLevel>", "<runTests>OtherTest</runTests>", "<rollbackOnError>false</rollbackOnError>", "<ignoreWarnings>true</ignoreWarnings>"} {
		if !strings.Contains(fake.options, option) {
			t.Errorf("deploy options lack %s", option)
		}
	}

	_, errOut, status := run("project", "deploy", "start")
	if status != 1 || !strings.HasPrefix(errOut, "Error (NothingToDeploy): No local changes to deploy.") {
		t.Errorf("nothing to deploy: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("project", "deploy", "start", "-d", "force-app", "-l", "RunSpecifiedTests")
	if status != 1 || !strings.Contains(errOut, "You must specify tests using the --tests flag if the --test-level flag is set to RunSpecifiedTests.") {
		t.Errorf("no tests: status %d, stderr %q", status, errOut)
	}
}

func TestProjectDeployFailureListsComponentFailures(t *testing.T) {
	_, fake, _ := deploySetup(t)
	fake.statusResult = deployFailed
	out, _, status := run("project", "deploy", "start", "-d", "force-app")
	if status != 1 {
		t.Errorf("status = %d, want 1 for a failed deploy", status)
	}
	for _, want := range []string{
		" ✘ Deploying Metadata ", "   ▸ Components: 0/1 (0%)\n", " ◼ Done\n", " Status: Failed\n",
		"Component Failures [1]\n", "│ Type      │ Name    │ Problem               │ Line:Column │", "│ ApexClass │ Greeter │ Unexpected token '}'. │ 3:12        │",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Deployed Source") {
		t.Errorf("a failed deploy lists deployed source:\n%s", out)
	}
	out, _, status = run("project", "deploy", "start", "-d", "force-app", "--json")
	if status != 1 || !strings.HasPrefix(out, "{\n  \"status\": 1,\n  \"result\": {") || !strings.Contains(out, `"error": "Unexpected token '}'."`) ||
		!strings.Contains(out, `"lineNumber": 3`) || !strings.Contains(out, `"state": "Failed"`) {
		t.Errorf("json: status %d\n%s", status, out)
	}
}

func TestProjectDeployAsyncAndValidate(t *testing.T) {
	_, fake, _ := deploySetup(t)
	out, _, status := run("project", "deploy", "start", "-d", "force-app", "--async")
	want := `
 ─────────────── Deploying Metadata ───────────────

 Deploying v67.0 metadata to me@example.com using the v67.0 SOAP API.

 ✔ Preparing T
 ◯ Waiting for the org to respond - Skipped
 ◯ Deploying Metadata - Skipped
 ◯ Running Tests - Skipped
 ◯ Updating Source Tracking - Skipped
 ✔ Done T

 Status: Queued
 Deploy ID: 0Af000000000001AAA
 Target Org: me@example.com
 Elapsed Time: T

Deploy has been queued.

Run "sf project deploy resume --job-id 0Af000000000001AAA" to resume watching the deploy.
Run "sf project deploy report --job-id 0Af000000000001AAA" to get the latest status.
Run "sf project deploy cancel --job-id 0Af000000000001AAA" to cancel the deploy.
`
	if got := durationPattern.ReplaceAllString(out, "T"); status != 0 || got != want {
		t.Errorf("async: status %d\n%s\nwant\n%s", status, got, want)
	}
	for _, request := range fake.requests {
		if request == "checkDeployStatus" {
			t.Error("--async checked the deploy status")
		}
	}
	out, _, _ = run("project", "deploy", "start", "-d", "force-app", "--async", "--json")
	if !strings.Contains(out, "\"done\": false,\n    \"status\": \"Queued\",\n    \"files\": [],\n    \"zipSize\": ") {
		t.Errorf("async json:\n%s", out)
	}

	fake.statusResult = strings.Replace(deploySucceeded, "<checkOnly>false</checkOnly>", "<checkOnly>true</checkOnly>", 1)
	out, _, status = run("project", "deploy", "validate", "-d", "force-app")
	if status != 0 || !strings.HasPrefix(out, "Validating Deployment of v67.0 metadata to me@example.com using the v67.0 SOAP API.\n\n ───────────── Validating Deployment ─────────────\n\n\n ✔ Preparing") ||
		!strings.Contains(out, "\nValidated Source\n") ||
		!strings.HasSuffix(out, "Successfully validated the deployment (0Af000000000001AAA).\nRun \"sf project deploy quick --job-id 0Af000000000001AAA\" to execute this deploy\n") {
		t.Errorf("validate: status %d\n%s", status, out)
	}
	if !strings.Contains(fake.options, "<checkOnly>true</checkOnly>") || !strings.Contains(fake.options, "<testLevel>RunLocalTests</testLevel>") {
		t.Errorf("validate options = %s", fake.options)
	}
}

func TestProjectDeployQuickReportResumeAndCancel(t *testing.T) {
	org, fake, _ := deploySetup(t)
	// A quick deploy prints the id and the URL of the deploy, and the
	// components as the org names them.
	out, errOut, status := run("project", "deploy", "quick", "-i", "0Af000000000001AAA")
	want := "Deploy ID: 0Af000000000001AAA\nDeploy URL: " + org.URL + `//lightning/setup/DeployStatus/page?address=%2Fchangemgmt%2FmonitorDeploymentsDetails.apexp%3FasyncId%3D0Af000000000001AAA%26retURL%3D%252Fchangemgmt%252FmonitorDeployment.apexp

Deployed Source
┌─────────┬─────────┬───────────┬─────────────────────┐
│ State   │ Name    │ Type      │ Path                │
├─────────┼─────────┼───────────┼─────────────────────┤
│ Created │ Greeter │ ApexClass │ classes/Greeter.cls │
└─────────┴─────────┴───────────┴─────────────────────┘



Successfully deployed (0Af000000000001AAA).
`
	if status != 0 || out != want || fake.requests[0] != "deployRecentValidation" {
		t.Errorf("quick: status %d, stderr %q\n%s\nwant\n%s", status, errOut, out, want)
	}

	// The quick deploy is in the cache as succeeded, so the report lists
	// no files.
	out, _, status = run("project", "deploy", "report", "--job-id", "0Af000000000001AAA")
	if status != 0 || !strings.HasPrefix(out, "0Af000000000001AAA... Succeeded\n\nDeploy Info\n") || !strings.Contains(out, "│ numberComponentsDeployed │ 1 ") ||
		!strings.Contains(out, `
Deploy Options
┌────────────┬────────────────────┐
│ Key        │ Value              │
├────────────┼────────────────────┤
│ status     │ Succeeded          │
│ job-id     │ 0Af000000000001AAA │
│ target-org │ me@example.com     │
└────────────┴────────────────────┘
`) || strings.Contains(out, "Deployed Source") {
		t.Errorf("report: status %d\n%s", status, out)
	}

	out, errOut, status = run("project", "deploy", "resume", "-i", "0Af000000000001AAA")
	if status != 0 || errOut != "Warning: Job ID 0Af000000000001AAA is not resumable because it already completed with status: Succeeded. Displaying results...\n" {
		t.Errorf("resume: status %d, stderr %q\n%s", status, errOut, out)
	}
	_, errOut, status = run("project", "deploy", "resume", "-i", "0Af000000000009AAA")
	if status != 1 || errOut != "Error (NoMatchingJobIdError): No matching job found in cache for ID: 0Af000000000009AAA.\n\n\n" {
		t.Errorf("resume of an unknown job: status %d, stderr %q", status, errOut)
	}

	fake.statusResult = `<done>false</done><id>0Af000000000001AAA</id><status>InProgress</status><success>false</success>`
	expectRun(t, 0, "Deploy has been queued for cancellation.\n\nRun \"sf project deploy resume --job-id 0Af000000000001AAA\" to resume watching the deploy.\nRun \"sf project deploy report --job-id 0Af000000000001AAA\" to get the latest status.\n",
		"project", "deploy", "cancel", "-i", "0Af000000000001AAA", "--async")
}

func TestProjectDeployReportNeedsAJob(t *testing.T) {
	deploySetup(t)
	_, errOut, status := run("project", "deploy", "report", "--use-most-recent")
	if status != 1 || errOut != "Error (NoRecentJobIdError): There are no recent job IDs available.\n\n\n" {
		t.Errorf("--use-most-recent with an empty cache: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("project", "deploy", "report")
	if status != 2 || !strings.Contains(errOut, "Exactly one of the following must be provided: --job-id, --use-most-recent") {
		t.Errorf("no job: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("project", "deploy", "resume")
	if status != 2 || !strings.Contains(errOut, "Exactly one of the following must be provided: --use-most-recent, --job-id") {
		t.Errorf("resume without a job: status %d, stderr %q", status, errOut)
	}
}

func TestDeployCacheRecordsTheDeployAsSfDoes(t *testing.T) {
	_, fake, project := deploySetup(t)
	home := os.Getenv("HOME")
	if _, errOut, status := run("project", "deploy", "start", "-d", "force-app/main/default/classes", "-l", "RunSpecifiedTests", "-t", "GreeterTest", "--async"); status != 0 {
		t.Fatalf("deploy: status %d %s", status, errOut)
	}
	manifest := filepath.Join(home, ".sf", "manifestCache", "0Af000000000001AAA.xml")
	cache := regexp.MustCompile(`"timestamp": "[^"]+"`).ReplaceAllString(readFile(t, filepath.Join(home, ".sf", "deploy-cache.json")), `"timestamp": "T"`)
	wantCache := `{
  "0Af000000000001AAA": {
    "async": true,
    "dry-run": false,
    "ignore-conflicts": false,
    "ignore-errors": false,
    "ignore-warnings": false,
    "target-org": "me@example.com",
    "tests": [
      "GreeterTest"
    ],
    "test-level": "RunSpecifiedTests",
    "api": "SOAP",
    "manifest": "` + manifest + `",
    "wait": 33,
    "isMdapi": false,
    "timestamp": "T"
  }
}`
	if cache != wantCache {
		t.Errorf("cache =\n%s\nwant\n%s", cache, wantCache)
	}
	if got := readFile(t, manifest); !strings.Contains(got, "<members>Greeter</members>") || !strings.Contains(got, "<members>GreeterTest</members>") || !strings.Contains(got, "<name>ApexClass</name>") {
		t.Errorf("manifest = %s", got)
	}

	// The default org is removed, so the report finds the org in the
	// cache entry.  It finds the local files through the manifest.
	if _, _, status := run("config", "unset", "target-org", "--global"); status != 0 {
		t.Fatal("config unset failed")
	}
	fake.requests = nil
	out, errOut, status := run("project", "deploy", "report", "--use-most-recent")
	if status != 0 || !strings.HasPrefix(out, "0Af000000000001AAA... Succeeded\n") || !strings.Contains(out, `
│ tests            │ GreeterTest `) || !strings.Contains(out, "│ wait             │ 33 minutes ") || !strings.Contains(out, "│ use-most-recent  │ true ") ||
		!strings.Contains(out, "│ Created │ Greeter │ ApexClass │ force-app/main/default/classes/Greeter.cls-meta.xml │") {
		t.Errorf("report --use-most-recent: status %d, stderr %q\n%s", status, errOut, out)
	}

	// The resume watches the deploy, shows the stages of a deploy that
	// was already finished as skipped, and records the status.
	out, errOut, status = run("project", "deploy", "resume", "-r")
	if status != 0 || errOut != "" || !strings.Contains(out, " ──────────────── Resuming Deploy ────────────────\n") ||
		!strings.Contains(out, " ◯ Deploying Metadata - Skipped\n ◯ Running Tests - Skipped\n") ||
		!strings.Contains(out, "│ Created │ Greeter │ ApexClass │ "+strings.TrimPrefix(filepath.Join(project, "force-app/main/default/classes/Greeter.cls"), project+"/")+" ") {
		t.Errorf("resume -r: status %d, stderr %q\n%s", status, errOut, out)
	}
	if cache := readFile(t, filepath.Join(home, ".sf", "deploy-cache.json")); !strings.Contains(cache, `"isMdapi": false,
    "timestamp": "`) || !strings.Contains(cache, `"status": "Succeeded"`) {
		t.Errorf("cache after resume =\n%s", cache)
	}
	// A cancel of a finished deploy is refused by the org.
	out, _, status = run("project", "deploy", "resume", "-r", "--json")
	if status != 0 || !strings.Contains(out, `"id": "0Af000000000001AAA"`) || !strings.Contains(out, "is not resumable because it already completed with status: Succeeded") {
		t.Errorf("resume -r --json: status %d\n%s", status, out)
	}
}

func retrieveZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create("unpackaged/" + name)
		if err != nil {
			t.Fatal(err)
		}
		entry.Write([]byte(content))
	}
	writer.Close()
	return buffer.Bytes()
}

func TestProjectRetrieveStartWritesSourceFormat(t *testing.T) {
	_, fake, project := deploySetup(t)
	fake.retrieveZip = retrieveZip(t, map[string]string{
		"classes/Remote.cls":          "public class Remote {}",
		"classes/Remote.cls-meta.xml": greeterMeta,
		"package.xml":                 "<Package/>",
	})
	out, errOut, status := run("project", "retrieve", "start", "-m", "ApexClass:Remote")
	if status != 0 {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	want := `
 ────────────── Retrieving Metadata ──────────────

 Retrieving v67.0 metadata from me@example.com using the v67.0 SOAP API

 ✔ Preparing retrieve request T
 ✔ Sending request to org T
 ✔ Waiting for the org to respond T
 ✔ Done T

 Status: Succeeded
 Elapsed Time: T


Retrieved Source
┌─────────┬────────┬───────────┬────────────────────────────────────────────────────┐
│ State   │ Name   │ Type      │ Path                                               │
├─────────┼────────┼───────────┼────────────────────────────────────────────────────┤
│ Created │ Remote │ ApexClass │ force-app/main/default/classes/Remote.cls          │
│ Created │ Remote │ ApexClass │ force-app/main/default/classes/Remote.cls-meta.xml │
└─────────┴────────┴───────────┴────────────────────────────────────────────────────┘

`
	if got := durationPattern.ReplaceAllString(out, "T"); got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}
	if content, _ := os.ReadFile(filepath.Join(project, "force-app", "main", "default", "classes", "Remote.cls")); string(content) != "public class Remote {}" {
		t.Errorf("retrieved class = %q", content)
	}
	if !strings.Contains(fake.options, "<members>Remote</members>") || !strings.Contains(fake.options, "<name>ApexClass</name>") {
		t.Errorf("retrieve request = %s", fake.options)
	}

	out, _, status = run("project", "retrieve", "start", "--metadata", "ApexClass:Remote", "--json")
	// sf reports the files of a component that existed as changed.
	if status != 0 || !strings.Contains(out, `"state": "Changed"`) ||
		!strings.Contains(out, `"filePath": "`+filepath.Join(project, "force-app", "main", "default", "classes", "Remote.cls")+`"`) {
		t.Errorf("json: status %d\n%s", status, out)
	}

	_, errOut, status = run("project", "retrieve", "start")
	if status != 1 || !strings.HasPrefix(errOut, "Error (NoSourceTrackingError): Unable to track changes in your source files.") {
		t.Errorf("no components: status %d, stderr %q", status, errOut)
	}
}

func TestProjectRetrieveStartReportsTheResponseOfTheOrg(t *testing.T) {
	_, fake, project := deploySetup(t)
	fake.retrieveZip = retrieveZip(t, map[string]string{
		"classes/Remote.cls":          "public class Remote {}",
		"classes/Remote.cls-meta.xml": greeterMeta,
		"package.xml":                 "<Package/>",
	})
	fake.retrieveDetails = `<fileProperties><createdById>005000000000001AAA</createdById><fileName>unpackaged/classes/Remote.cls</fileName><fullName>Remote</fullName><id>01p000000000001AAA</id><type>ApexClass</type></fileProperties>`
	out, errOut, status := run("project", "retrieve", "start", "-m", "ApexClass:Remote", "--json")
	if status != 0 {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	class := filepath.Join(project, "force-app", "main", "default", "classes", "Remote.cls")
	want := `{
  "status": 0,
  "result": {
    "done": true,
    "fileProperties": [
      {
        "createdById": "005000000000001AAA",
        "fileName": "unpackaged/classes/Remote.cls",
        "fullName": "Remote",
        "id": "01p000000000001AAA",
        "type": "ApexClass"
      }
    ],
    "id": "09S000000000001AAA",
    "status": "Succeeded",
    "success": true,
    "messages": [],
    "files": [
      {
        "fullName": "Remote",
        "type": "ApexClass",
        "state": "Created",
        "filePath": "` + class + `"
      },
      {
        "fullName": "Remote",
        "type": "ApexClass",
        "state": "Created",
        "filePath": "` + class + `-meta.xml"
      }
    ]
  },
  "warnings": []
}
`
	if out != want {
		t.Errorf("json =\n%s\nwant\n%s", out, want)
	}

	// A component the org does not have is a message of the response, a
	// failed file, and a row of the warnings table.
	fake.retrieveMessages = `<messages><fileName>unpackaged/package.xml</fileName><problem>Entity of type 'ApexClass' named 'Nope' cannot be found</problem></messages>`
	out, _, status = run("project", "retrieve", "start", "-m", "ApexClass:Remote", "-m", "ApexClass:Nope", "--json")
	for _, want := range []string{`    "id": "09S000000000001AAA",
    "messages": [
      {
        "fileName": "unpackaged/package.xml",
        "problem": "Entity of type 'ApexClass' named 'Nope' cannot be found"
      }
    ],
    "status": "Succeeded",
    "success": true,
    "files": [
      {
        "fullName": "Nope",
        "type": "ApexClass",
        "state": "Failed",
        "error": "Entity of type 'ApexClass' named 'Nope' cannot be found",
        "problemType": "Error"
      },
      {
        "fullName": "Remote",`, `"warnings": []`} {
		if status != 0 || !strings.Contains(out, want) {
			t.Errorf("status %d, json lacks %s:\n%s", status, want, out)
		}
	}
	out, errOut, status = run("project", "retrieve", "start", "-m", "ApexClass:Remote", "-m", "ApexClass:Nope")
	if status != 0 || errOut != "" || !strings.HasSuffix(out, `└─────────┴────────┴───────────┴────────────────────────────────────────────────────┘


Warnings
┌────────────────────────┬─────────────────────────────────────────────────────────┐
│ File                   │ Problem                                                 │
├────────────────────────┼─────────────────────────────────────────────────────────┤
│ unpackaged/package.xml │ Entity of type 'ApexClass' named 'Nope' cannot be found │
└────────────────────────┴─────────────────────────────────────────────────────────┘

`) {
		t.Errorf("status %d, stderr %q\n%s", status, errOut, out)
	}
	// When only the missing component was asked for, sf warns that
	// nothing was retrieved and still prints the table.
	fake.retrieveZip = retrieveZip(t, map[string]string{"package.xml": "<Package/>"})
	out, errOut, status = run("project", "retrieve", "start", "-m", "ApexClass:Nope")
	if status != 0 || errOut != " ›   Warning: Nothing retrieved\n" || !strings.Contains(out, "\n\nWarnings\n┌") || strings.Contains(out, "Retrieved Source") {
		t.Errorf("only missing: status %d, stderr %q\n%s", status, errOut, out)
	}
}

func TestProjectRetrieveStartWritesAComponentWhereItIs(t *testing.T) {
	_, fake, project := deploySetup(t)
	os.WriteFile(filepath.Join(project, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true},{"path":"other"}],"sourceApiVersion":"67.0"}`), 0o644)
	other := filepath.Join(project, "other", "main", "default", "classes")
	os.MkdirAll(other, 0o755)
	os.WriteFile(filepath.Join(other, "Elsewhere.cls"), []byte("public class Elsewhere { /* local */ }"), 0o644)
	os.WriteFile(filepath.Join(other, "Elsewhere.cls-meta.xml"), []byte(greeterMeta), 0o644)
	fake.retrieveZip = retrieveZip(t, map[string]string{
		"classes/Elsewhere.cls":          "public class Elsewhere {}",
		"classes/Elsewhere.cls-meta.xml": greeterMeta,
		"classes/Remote.cls":             "public class Remote {}",
		"classes/Remote.cls-meta.xml":    greeterMeta,
		"package.xml":                    "<Package/>",
	})
	out, errOut, status := run("project", "retrieve", "start", "-m", "ApexClass:Elsewhere", "-m", "ApexClass:Remote")
	// The rows are in the order of their paths.
	if status != 0 || !strings.HasSuffix(out, `
│ Created │ Remote    │ ApexClass │ force-app/main/default/classes/Remote.cls          │
│ Created │ Remote    │ ApexClass │ force-app/main/default/classes/Remote.cls-meta.xml │
│ Changed │ Elsewhere │ ApexClass │ other/main/default/classes/Elsewhere.cls           │
│ Changed │ Elsewhere │ ApexClass │ other/main/default/classes/Elsewhere.cls-meta.xml  │
└─────────┴───────────┴───────────┴────────────────────────────────────────────────────┘

`) {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	if content := readFile(t, filepath.Join(other, "Elsewhere.cls")); content != "public class Elsewhere {}" {
		t.Errorf("class in the second package directory = %q", content)
	}
	if exists(filepath.Join(project, "force-app", "main", "default", "classes", "Elsewhere.cls")) {
		t.Error("the class was also written to the default package directory")
	}

	// With --output-dir every file goes to that directory.
	out, errOut, status = run("project", "retrieve", "start", "-m", "ApexClass:Elsewhere", "-r", "third")
	if status != 0 || !exists(filepath.Join(project, "third", "main", "default", "classes", "Elsewhere.cls")) {
		t.Errorf("output-dir: status %d, stderr %s\n%s", status, errOut, out)
	}
}

func TestProjectRetrieveStartToMetadataDirectory(t *testing.T) {
	_, fake, _ := deploySetup(t)
	fake.retrieveZip = retrieveZip(t, map[string]string{"classes/Remote.cls": "public class Remote {}", "package.xml": "<Package/>"})
	target := filepath.Join(os.Getenv("HOME"), "retrieved")
	manifest := filepath.Join(os.Getenv("HOME"), "package.xml")
	os.WriteFile(manifest, []byte(`<Package xmlns="http://soap.sforce.com/2006/04/metadata"><types><members>Remote</members><name>ApexClass</name></types><version>67.0</version></Package>`), 0o644)
	out, _, status := run("project", "retrieve", "start", "-x", manifest, "-t", target, "--unzip")
	if status != 0 || !strings.HasSuffix(out, "Wrote retrieve zip file to "+filepath.Join(target, "unpackaged.zip")+".\nExtracted "+filepath.Join(target, "unpackaged.zip")+" to "+filepath.Join(target, "unpackaged")+".\n") {
		t.Errorf("status %d\n%s", status, out)
	}
	if content, _ := os.ReadFile(filepath.Join(target, "unpackaged", "unpackaged", "classes", "Remote.cls")); string(content) != "public class Remote {}" {
		t.Errorf("extracted class = %q", content)
	}
	// sf deletes the zip file after it extracted it.
	if exists(filepath.Join(target, "unpackaged.zip")) {
		t.Error("the zip file is still there after --unzip")
	}
	// With --single-package the files are at the top of the zip file.
	single := filepath.Join(os.Getenv("HOME"), "single")
	if _, _, status := run("project", "retrieve", "start", "-x", manifest, "-t", single, "--unzip", "--single-package"); status != 0 {
		t.Errorf("single package: status %d", status)
	}
	if content, _ := os.ReadFile(filepath.Join(single, "unpackaged", "classes", "Remote.cls")); string(content) != "public class Remote {}" {
		t.Errorf("single package class = %q", content)
	}
	if _, errOut, status := run("project", "retrieve", "start", "-x", manifest, "--single-package"); status != 2 || !strings.Contains(errOut, "All of the following must be provided when using --single-package: --target-metadata-dir") {
		t.Errorf("single package without a directory: status %d, stderr %q", status, errOut)
	}
	// A relative directory is reported as an absolute path, and the
	// version is the one of the project.
	wd, _ := os.Getwd()
	out, _, status = run("project", "retrieve", "start", "-x", manifest, "-t", "relative-out")
	if status != 0 || !strings.HasSuffix(out, "Wrote retrieve zip file to "+filepath.Join(wd, "relative-out", "unpackaged.zip")+".\n") {
		t.Errorf("relative: status %d\n%s", status, out)
	}
	archive, err := zip.OpenReader(filepath.Join(wd, "relative-out", "unpackaged.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	var names []string
	for _, file := range archive.File {
		names = append(names, file.Name)
	}
	if strings.Join(names, " ") != "unpackaged/classes/Remote.cls unpackaged/package.xml" {
		t.Errorf("zip entries = %v", names)
	}
}

// Compared with sf: a child component named on its own selects nothing, and
// the files of the child components of an object are listed with the object,
// before its own files.  The custom labels file is listed last.
func TestProjectConvertMdapiChildComponentsAndOrder(t *testing.T) {
	convertProject(t)
	os.MkdirAll(filepath.Join("src", "objects"), 0o755)
	os.MkdirAll(filepath.Join("src", "classes"), 0o755)
	os.MkdirAll(filepath.Join("src", "labels"), 0o755)
	os.WriteFile(filepath.Join("src", "package.xml"), []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Package xmlns=\"http://soap.sforce.com/2006/04/metadata\"><types><members>Bill__c</members><name>CustomObject</name></types><types><members>Hello</members><name>ApexClass</name></types><types><members>CustomLabels</members><name>CustomLabels</name></types><version>62.0</version></Package>\n"), 0o644)
	os.WriteFile(filepath.Join("src", "objects", "Bill__c.object"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
    <deploymentStatus>Deployed</deploymentStatus>
    <fields>
        <fullName>Amount__c</fullName>
        <label>Amount</label>
        <precision>10</precision>
        <scale>2</scale>
        <type>Number</type>
    </fields>
    <label>Bill</label>
    <listViews>
        <fullName>All</fullName>
        <filterScope>Everything</filterScope>
        <label>All</label>
    </listViews>
    <nameField><label>Name</label><type>Text</type></nameField>
    <pluralLabel>Bills</pluralLabel>
    <sharingModel>ReadWrite</sharingModel>
    <validationRules>
        <fullName>Positive</fullName>
        <active>true</active>
        <errorConditionFormula>Amount__c &lt; 0</errorConditionFormula>
        <errorMessage>Must be positive</errorMessage>
    </validationRules>
</CustomObject>
`), 0o644)
	os.WriteFile(filepath.Join("src", "classes", "Hello.cls"), []byte("public class Hello {}\n"), 0o644)
	os.WriteFile(filepath.Join("src", "classes", "Hello.cls-meta.xml"), []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <apiVersion>62.0</apiVersion>\n    <status>Active</status>\n</ApexClass>\n"), 0o644)
	os.WriteFile(filepath.Join("src", "labels", "CustomLabels.labels"), []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomLabels xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <labels><fullName>Welcome</fullName><language>en_US</language><protected>false</protected><shortDescription>Welcome</shortDescription><value>Welcome</value></labels>\n</CustomLabels>\n"), 0o644)

	for _, selection := range [][]string{{"-m", "CustomField:Bill__c.Amount__c"}, {"-m", "CustomField"}, {"-m", "ValidationRule:Bill__c.Positive"}} {
		_, errOut, status := run(append([]string{"project", "convert", "mdapi", "-r", "src", "-d", "none"}, selection...)...)
		if status != 1 || errOut != "Error (SfError): No results to format\n\n\n" {
			t.Errorf("%v: status %d, stderr %q", selection, status, errOut)
		}
	}
	// A child component named with other components does not change what
	// the others select.
	out, _, status := run("project", "convert", "mdapi", "-r", "src", "-d", "some", "-m", "CustomField:Bill__c.Amount__c", "-m", "ApexClass")
	if status != 0 || !strings.Contains(out, "│ Hello     │ ApexClass │") || strings.Contains(out, "Bill__c") {
		t.Errorf("child with a class: status %d\n%s", status, out)
	}

	out, _, status = run("project", "convert", "mdapi", "-r", "src", "-d", "all")
	var order []string
	for _, line := range strings.Split(out, "\n") {
		cells := strings.Split(line, "│")
		if len(cells) > 4 && strings.TrimSpace(cells[1]) == "Add" {
			order = append(order, strings.TrimSpace(cells[3])+" "+strings.TrimSpace(cells[2]))
		}
	}
	want := "ApexClass Hello|ApexClass Hello|CustomField Bill__c.Amount__c|ListView Bill__c.All|ValidationRule Bill__c.Positive|CustomObject Bill__c|CustomObject Bill__c|CustomLabels CustomLabels"
	if status != 0 || strings.Join(order, "|") != want {
		t.Errorf("order: status %d\n got %s\nwant %s\n%s", status, strings.Join(order, "|"), want, out)
	}

	// Compared with sf: a custom labels file without labels is not written.
	os.WriteFile(filepath.Join("src", "labels", "CustomLabels.labels"), []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomLabels xmlns=\"http://soap.sforce.com/2006/04/metadata\"/>\n"), 0o644)
	out, _, status = run("project", "convert", "mdapi", "-r", "src", "-d", "nolabels")
	if _, err := os.Stat(filepath.Join("nolabels", "main", "default", "labels")); status != 0 || err == nil || strings.Contains(out, "CustomLabels") {
		t.Errorf("empty labels file: status %d, stat error %v\n%s", status, err, out)
	}
}
