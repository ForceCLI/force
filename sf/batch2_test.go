package sf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ForceCLI/force/lib"
)

// The output formats in this file were captured from sf 2.138.6, except for
// org login sfdx-url, which follows the sf plugin source.

func TestForceIgnorePatterns(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, ".forceignore"), []byte("# comment\nforce-app/main/default/classes/GreeterTest.cls\n**/*.profile-meta.xml\nscratch/\n*.bak\n!keep.bak\n/top.txt\n"), 0o644)
	ignore := loadForceIgnore(root)
	cases := map[string]bool{
		"force-app/main/default/classes/GreeterTest.cls":         true,
		"force-app/main/default/classes/Greeter.cls":             false,
		"force-app/main/default/profiles/Admin.profile-meta.xml": true,
		"scratch/notes.txt":                           true,
		"force-app/scratch/deep/file.cls":             true,
		"force-app/old.bak":                           true,
		"force-app/keep.bak":                          false,
		"top.txt":                                     true,
		"force-app/top.txt":                           false,
		"force-app/main/default/.eslintrc.json":       true,
		"force-app/main/default/classes/Copy.cls.dup": true,
		"force-app/main/default/objects/A__c/A__c.object-meta.xml": false,
	}
	for path, want := range cases {
		if got := ignore.ignores(filepath.Join(root, filepath.FromSlash(path))); got != want {
			t.Errorf("ignores(%s) = %v, want %v", path, got, want)
		}
	}
	if !ignore.ignoresComponentFile(filepath.Join(root, "force-app/main/default/classes/GreeterTest.cls-meta.xml")) {
		t.Error("the -meta.xml file of an ignored class is not left out")
	}
}

func TestProjectListIgnored(t *testing.T) {
	home := useTempHome(t)
	project := newSourceProject(t, home)
	expectRun(t, 0, "No ignored files found in paths:\nforce-app\n", "project", "list", "ignored")
	os.WriteFile(filepath.Join(project, ".forceignore"), []byte("force-app/main/default/classes/GreeterTest.cls\n"), 0o644)
	expectRun(t, 0, "Found the following ignored files:\nforce-app/main/default/classes/GreeterTest.cls\n", "project", "list", "ignored")
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "ignoredFiles": [
      "force-app/main/default/classes/GreeterTest.cls"
    ]
  },
  "warnings": [
    "The \"force source ignored list\" command has been deprecated. Use \"project list ignored\" instead."
  ]
}
`, "force:source:ignored:list", "-p", "force-app/main/default/classes", "--json")
}

func TestForceIgnoredFilesAreNotDeployedOrConverted(t *testing.T) {
	_, fake, project := deploySetup(t)
	os.WriteFile(filepath.Join(project, ".forceignore"), []byte("**/GreeterTest.cls\n**/Invoice__c/fields/\n"), 0o644)
	if _, errOut, status := run("project", "deploy", "start", "-d", "force-app", "--json"); status != 0 {
		t.Fatalf("status %d %s", status, errOut)
	}
	if _, ok := fake.deployed["classes/GreeterTest.cls"]; ok {
		t.Error("an ignored class was deployed")
	}
	if _, ok := fake.deployed["classes/GreeterTest.cls-meta.xml"]; ok {
		t.Error("the -meta.xml file of an ignored class was deployed")
	}
	if _, ok := fake.deployed["classes/Greeter.cls"]; !ok {
		t.Errorf("a class that is not ignored was left out: %v", keys(fake.deployed))
	}
	if strings.Contains(fake.deployed["package.xml"], "GreeterTest") || strings.Contains(fake.deployed["package.xml"], "Amount__c") {
		t.Errorf("package.xml lists ignored components:\n%s", fake.deployed["package.xml"])
	}
	if strings.Contains(fake.deployed["objects/Invoice__c.object"], "Amount__c") {
		t.Errorf("an ignored field was composed into the object:\n%s", fake.deployed["objects/Invoice__c.object"])
	}
}

func TestProjectGenerateManifest(t *testing.T) {
	home := useTempHome(t)
	project := newSourceProject(t, home)
	expectRun(t, 0, "successfully wrote package.xml\n", "project", "generate", "manifest", "-p", "force-app/main/default/classes")
	manifest, _ := os.ReadFile(filepath.Join(project, "package.xml"))
	want := `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>Greeter</members>
        <members>GreeterTest</members>
        <name>ApexClass</name>
    </types>
    <version>67.0</version>
</Package>
`
	if string(manifest) != want {
		t.Errorf("package.xml =\n%s", manifest)
	}

	out := filepath.Join(home, "out")
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "path": "`+filepath.Join(out, "mine.xml")+`",
    "name": "mine.xml"
  },
  "warnings": []
}
`, "project", "generate", "manifest", "-m", "ApexClass:NoSuch", "-m", "CustomObject", "-n", "mine", "-d", out, "--json")
	written, _ := os.ReadFile(filepath.Join(out, "mine.xml"))
	if !strings.Contains(string(written), "<members>NoSuch</members>") || !strings.Contains(string(written), "<members>*</members>\n        <name>CustomObject</name>") {
		t.Errorf("mine.xml =\n%s", written)
	}

	expectRun(t, 0, "successfully wrote destructiveChanges.xml\n", "project", "generate", "manifest", "-m", "ApexClass", "-t", "destroy")
	expectRun(t, 0, "successfully wrote destructiveChangesPre.xml\n", "force:source:manifest:create", "-m", "ApexClass", "--type", "pre")

	expectRun(t, 0, "successfully wrote package.xml\n", "project", "generate", "manifest", "-p", "force-app", "--excluded-metadata", "ApexClass,CustomField")
	manifest, _ = os.ReadFile(filepath.Join(project, "package.xml"))
	if strings.Contains(string(manifest), "ApexClass") || !strings.Contains(string(manifest), "<name>CustomObject</name>") {
		t.Errorf("--excluded-metadata:\n%s", manifest)
	}

	_, errOut, status := run("project", "generate", "manifest")
	if status != 1 || errOut != "Error (1): provided flags must include at least one of: metadata,source-dir,from-org\n\n\n" {
		t.Errorf("no flags: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("project", "generate", "manifest", "-m", "ApexClass", "-t", "pre", "-n", "x")
	if status != 2 || errOut != "Error (2): The following errors occurred:\n  --name=x cannot also be provided when using --type\n  --type=pre cannot also be provided when using --name\nSee more help with --help\n\n\n" {
		t.Errorf("name and type: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("project", "generate", "manifest", "-p", "nosuchdir")
	if status != 1 || errOut != "Error (SfError): nosuchdir: File or folder not found\n\n\n" {
		t.Errorf("missing dir: status %d, stderr %q", status, errOut)
	}
}

func TestOrgAuthShowCommands(t *testing.T) {
	loggedInFakeOrg(t)
	out, errOut, status := run("org", "auth", "show-access-token", "--no-prompt")
	want := "┌──────────────┬───────────────────────────┐\n│ Key          │ Value                     │\n├──────────────┼───────────────────────────┤\n│ Access Token │ " + fakeToken + " │\n└──────────────┴───────────────────────────┘\n\n"
	if status != 0 || out != want || !strings.HasPrefix(errOut, "Warning: This command exposes a sensitive Access Token") {
		t.Errorf("status %d, stdout\n%s\nstderr %q", status, out, errOut)
	}
	out, _, _ = run("org", "auth", "show-access-token", "--json")
	if !strings.Contains(out, `"accessToken": "`+fakeToken+`"`) || !strings.Contains(out, `"This command exposes a sensitive Access Token`) {
		t.Errorf("json:\n%s", out)
	}
	// Without --no-prompt the command asks; no answer means no.
	_, errOut, status = run("org", "auth", "show-access-token")
	if status != 1 || !strings.Contains(errOut, "You're about to reveal the access token for \"me@example.com\".") || !strings.Contains(errOut, "confirmation denied or timed out.") {
		t.Errorf("declined: status %d, stderr %q", status, errOut)
	}

	_, errOut, status = run("org", "auth", "show-sfdx-auth-url", "--no-prompt")
	if status != 1 || !strings.Contains(errOut, `Error (NoRefreshTokenError): An SFDX Auth URL is not available for "me@example.com".`) {
		t.Errorf("no refresh token: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("org", "auth", "show-user-password", "--no-prompt")
	if status != 1 || !strings.Contains(errOut, `Error (NoPasswordError): No password found for "me@example.com".`) {
		t.Errorf("no password: status %d, stderr %q", status, errOut)
	}

	writeTestAuth(t, "web@example.com", lib.SFDXAuthFields{"refreshToken": "5Aep861", "clientId": "PlatformCLI", "instanceUrl": "https://example.my.salesforce.com", "password": "hunter2"})
	out, _, _ = run("org", "auth", "show-sfdx-auth-url", "-o", "web@example.com", "--json")
	if !strings.Contains(out, `"sfdxAuthUrl": "force://PlatformCLI::5Aep861@example.my.salesforce.com"`) {
		t.Errorf("sfdx auth url:\n%s", out)
	}
	out, _, _ = run("org", "auth", "show-user-password", "-o", "web@example.com", "--json")
	if !strings.Contains(out, `"password": "hunter2"`) {
		t.Errorf("password:\n%s", out)
	}
}

func TestOrgLoginSfdxURL(t *testing.T) {
	useTempHome(t)
	org := newFakeOrg(t)
	host := strings.TrimPrefix(org.URL, "http://")
	file := filepath.Join(t.TempDir(), "auth.json")
	// The file can hold the URL or JSON with sfdxAuthUrl, as "org display
	// --verbose --json" prints it.  The fake org is reached over http, so
	// the URL carries the scheme.
	os.WriteFile(file, []byte(`{"result":{"sfdxAuthUrl":"force://my-client:my-secret:my-refresh@http://`+host+`"}}`), 0o644)
	out, errOut, status := run("org", "login", "sfdx-url", "-f", file, "-a", "fromurl", "-s")
	if status != 0 || out != "Successfully authorized me@example.com with org ID 00D000000000000\n" {
		t.Fatalf("status %d, stdout %q, stderr %q", status, out, errOut)
	}
	if len(org.refreshRequests) != 1 {
		t.Fatalf("token requests = %d", len(org.refreshRequests))
	}
	form := org.refreshRequests[0]
	if form.Get("refresh_token") != "my-refresh" || form.Get("client_id") != "my-client" || form.Get("client_secret") != "my-secret" || form.Get("grant_type") != "refresh_token" {
		t.Errorf("token request = %v", form)
	}
	fields, err := lib.ReadSFDXAuthFields("me@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if fields.String("refreshToken") != "my-refresh" || fields.String("clientId") != "my-client" || fields.String("clientSecret") != "my-secret" || fields.String("accessToken") != fakeToken {
		t.Errorf("fields = %v", fields)
	}
	if lib.ResolveSFDXUsername("fromurl") != "me@example.com" {
		t.Error("the alias was not set")
	}
	// The stored authorization produces the same URL.
	if got := sfdxAuthURL(fields); got != "force://my-client:my-secret:my-refresh@"+host {
		t.Errorf("sfdxAuthURL = %s", got)
	}
	out, _, _ = run("org", "display", "--verbose", "--json")
	if !strings.Contains(out, `"sfdxAuthUrl": "[REDACTED] Use 'sf org auth show-sfdx-auth-url' to view"`) {
		t.Errorf("org display --verbose:\n%s", out)
	}

	os.WriteFile(file, []byte("https://not-a-force-url"), 0o644)
	_, errOut, status = run("org", "login", "sfdx-url", "--sfdx-url-file", file)
	if status != 1 || !strings.HasPrefix(errOut, "Error (INVALID_SFDX_AUTH_URL): Invalid SFDX authorization URL.") {
		t.Errorf("bad url: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("org", "login", "sfdx-url")
	if status != 2 || !strings.Contains(errOut, "Exactly one of the following must be provided: --sfdx-url-file, --sfdx-url-stdin") {
		t.Errorf("no source: status %d, stderr %q", status, errOut)
	}
}

func TestApexGetTestAndResultFiles(t *testing.T) {
	org := loggedInFakeOrg(t)
	serveTestRun(org)
	previous := testPollInterval
	testPollInterval = time.Millisecond
	defer func() { testPollInterval = previous }()

	out, _, status := run("apex", "get", "test", "-i", "707000000000001AAA")
	if status != 100 || !strings.HasPrefix(out, "=== Test Results\nTEST NAME ") || !strings.Contains(out, "Test Run Id          707000000000001AAA") {
		t.Errorf("get test: status %d\n%s", status, out)
	}
	out, _, _ = run("force:apex:test:report", "--test-run-id", "707000000000001AAA", "-r", "tap")
	if !strings.HasPrefix(out, "1..2\nnot ok 1 GreeterTest.fails\n") {
		t.Errorf("tap:\n%s", out)
	}

	dir := filepath.Join(t.TempDir(), "results")
	out, _, status = run("apex", "run", "test", "-n", "GreeterTest", "-w", "1", "-c", "-d", dir)
	if status != 100 || !strings.HasPrefix(out, "Test result files written to "+dir+"\n=== Test Results\n") {
		t.Errorf("output dir: status %d\n%s", status, out)
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	wantNames := "test-result-707000000000001AAA-codecoverage.json test-result-707000000000001AAA-junit.xml test-result-707000000000001AAA.json test-result-codecoverage.json test-result.txt test-run-id.txt"
	if strings.Join(names, " ") != wantNames {
		t.Errorf("files = %v", names)
	}
	if id, _ := os.ReadFile(filepath.Join(dir, "test-run-id.txt")); string(id) != "707000000000001AAA" {
		t.Errorf("test-run-id.txt = %q", id)
	}
	perTest, _ := os.ReadFile(filepath.Join(dir, "test-result-707000000000001AAA-codecoverage.json"))
	wantPerTest := `[[{"apexClassOrTriggerName":"Greeter","apexClassOrTriggerId":"01p000000000001AAA","apexTestClassId":"01p000000000002AAA","apexTestMethodName":"passes","numLinesCovered":3,"numLinesUncovered":1,"percentage":"75%","coverage":{"coveredLines":[2,3,5],"uncoveredLines":[7]}}]]`
	if string(perTest) != wantPerTest {
		t.Errorf("per-test coverage = %s", perTest)
	}
	byClass, _ := os.ReadFile(filepath.Join(dir, "test-result-codecoverage.json"))
	if string(byClass) != `[{"id":"01p000000000001AAA","name":"Greeter","totalLines":4,"lines":{"2":1,"3":1,"5":1,"7":0},"totalCovered":3,"coveredPercent":75}]` {
		t.Errorf("class coverage = %s", byClass)
	}
	var result map[string]interface{}
	data, _ := os.ReadFile(filepath.Join(dir, "test-result-707000000000001AAA.json"))
	if err := json.Unmarshal(data, &result); err != nil || result["coverage"] == nil || strings.Contains(string(data), "\n") {
		t.Errorf("result file is not compact JSON with coverage: %v", err)
	}
	if text, _ := os.ReadFile(filepath.Join(dir, "test-result.txt")); !strings.HasPrefix(string(text), "=== Test Results\n") {
		t.Errorf("test-result.txt = %q", text)
	}
}

func TestApexRunTestDetailedCoverage(t *testing.T) {
	org := loggedInFakeOrg(t)
	serveTestRun(org)
	previous := testPollInterval
	testPollInterval = time.Millisecond
	defer func() { testPollInterval = previous }()
	out, _, _ := run("apex", "run", "test", "-n", "GreeterTest", "-w", "1", "-c", "-v")
	if !strings.HasPrefix(out, "=== Apex Code Coverage for Test Run 707000000000001AAA\nTEST NAME           CLASS BEING TESTED  OUTCOME  PERCENT  MESSAGE") {
		t.Errorf("header:\n%s", out)
	}
	// The failing test has no coverage record; the passing test covered
	// Greeter.
	if !strings.Contains(out, "\nGreeterTest.fails                       Fail              System.AssertException") ||
		!strings.Contains(out, "\nGreeterTest.passes  Greeter             Pass     75%   ") {
		t.Errorf("rows:\n%s", out)
	}
	if strings.Contains(out, "=== Test Results") {
		t.Errorf("the detailed table does not replace the test results table:\n%s", out)
	}
}

func TestUnknownCommandUnderATopicWithFlags(t *testing.T) {
	_, errOut, status := run("apex", "get", "nosuch", "--json")
	if status != 127 || !strings.Contains(errOut, "apex get nosuch is not a sf command.") {
		t.Errorf("status %d, stderr %q", status, errOut)
	}
}
