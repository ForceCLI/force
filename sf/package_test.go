package sf

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The table layouts and messages in this file follow the sf 2.138.6
// packaging plugin source.  They were not compared with sf against a Dev
// Hub.

// devHubSetup logs in to the fake org, makes it the default Dev Hub and the
// default org, and creates a project with a package alias.
func devHubSetup(t *testing.T) *fakeOrg {
	t.Helper()
	useTempHome(t)
	org := newFakeOrg(t)
	org.login(t, "--alias", "hub", "--set-default", "--set-default-dev-hub")
	project := newProject(t, os.Getenv("HOME"))
	os.WriteFile(filepath.Join(project, "sfdx-project.json"), []byte(`{
  "packageDirectories": [
    {
      "path": "force-app",
      "default": true
    }
  ],
  "namespace": "acme",
  "packageAliases": {
    "billing": "0Ho000000000001AAA",
    "billing@1.0.0-1": "04t000000000001AAA"
  }
}`), 0o644)
	previous := packagePollInterval
	packagePollInterval = time.Millisecond
	t.Cleanup(func() { packagePollInterval = previous })
	return org
}

func TestPackageList(t *testing.T) {
	org := devHubSetup(t)
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"done":true,"totalSize":2,"records":[`+
			`{"Id":"0Ho000000000001AAA","SubscriberPackageId":"033000000000001AAA","Name":"Billing","Description":"Invoices","NamespacePrefix":"acme","ContainerOptions":"Managed","ConvertedFromPackageId":null,"IsOrgDependent":false,"PackageErrorUsername":null,"AppAnalyticsEnabled":false,"CreatedById":"005000000000001AAA","IsDeprecated":false},`+
			`{"Id":"0Ho000000000002AAA","SubscriberPackageId":"033000000000002AAA","Name":"Tools","Description":null,"NamespacePrefix":null,"ContainerOptions":"Unlocked","ConvertedFromPackageId":null,"IsOrgDependent":true,"PackageErrorUsername":null,"AppAnalyticsEnabled":false,"CreatedById":"005000000000001AAA","IsDeprecated":false}]}`)
	}
	expectRun(t, 0, `Packages [2]
┌──────────────────┬─────────┬────────────────────┬─────────┬─────────────┬───────────────────┐
│ Namespace Prefix │ Name    │ Id                 │ Alias   │ Description │ Container Options │
├──────────────────┼─────────┼────────────────────┼─────────┼─────────────┼───────────────────┤
│ acme             │ Billing │ 0Ho000000000001AAA │ billing │ Invoices    │ Managed           │
│                  │ Tools   │ 0Ho000000000002AAA │         │             │ Unlocked          │
└──────────────────┴─────────┴────────────────────┴─────────┴─────────────┴───────────────────┘

`, "package", "list")
	out, _, status := run("package", "list", "--json")
	if status != 0 || !strings.Contains(out, `"Alias": "billing"`) || !strings.Contains(out, `"IsOrgDependent": "N/A"`) || !strings.Contains(out, `"IsOrgDependent": "Yes"`) {
		t.Errorf("json: status %d\n%s", status, out)
	}
	out, _, _ = run("force:package:list", "--verbose", "-v", "hub")
	if !strings.Contains(out, "│ Org Dependent Unlocked Package │") || !strings.Contains(out, "033000000000002AAA") {
		t.Errorf("verbose:\n%s", out)
	}
}

func TestPackageCommandsNeedADevHub(t *testing.T) {
	useTempHome(t)
	_, errOut, status := run("package", "list")
	if status != 1 || errOut != "Error (NoDefaultDevHubError): No default dev hub found. Use -v or --target-dev-hub to specify an environment.\n\n\n" {
		t.Errorf("status %d, stderr %q", status, errOut)
	}
}

func TestPackageCreateRecordsThePackageInTheProject(t *testing.T) {
	org := devHubSetup(t)
	var request map[string]interface{}
	org.handlers["/services/data/v67.0/tooling/sobjects/Package2"] = func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&request)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"0Ho000000000009AAA","success":true,"errors":[]}`)
	}
	expectRun(t, 0, `Ids
┌────────────┬────────────────────┐
│ Name       │ Value              │
├────────────┼────────────────────┤
│ Package Id │ 0Ho000000000009AAA │
└────────────┴────────────────────┘

`, "package", "create", "-n", "Payments", "-t", "Managed", "-r", "force-app", "-d", "Card payments")
	if request["Name"] != "Payments" || request["ContainerOptions"] != "Managed" || request["NamespacePrefix"] != "acme" || request["Description"] != "Card payments" {
		t.Errorf("request = %v", request)
	}
	data, _ := os.ReadFile("sfdx-project.json")
	var project struct {
		PackageDirectories []map[string]interface{} `json:"packageDirectories"`
		Namespace          string                   `json:"namespace"`
		PackageAliases     map[string]string        `json:"packageAliases"`
	}
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatal(err)
	}
	directory := project.PackageDirectories[0]
	if project.PackageAliases["Payments"] != "0Ho000000000009AAA" || project.PackageAliases["billing"] != "0Ho000000000001AAA" || project.Namespace != "acme" ||
		directory["package"] != "Payments" || directory["versionNumber"] != "0.1.0.NEXT" || directory["default"] != true {
		t.Errorf("sfdx-project.json = %s", data)
	}
	_, errOut, status := run("package", "create", "-n", "X", "-t", "Other", "-r", "force-app")
	if status != 1 || !strings.Contains(errOut, "Expected --package-type=Other to be one of: Managed, Unlocked") {
		t.Errorf("bad type: status %d, stderr %q", status, errOut)
	}
}

func TestPackageInstalledList(t *testing.T) {
	org := devHubSetup(t)
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"0A3000000000001AAA","SubscriberPackageId":"033000000000001AAA",`+
			`"SubscriberPackage":{"NamespacePrefix":"acme","Name":"Billing"},`+
			`"SubscriberPackageVersion":{"Id":"04t000000000001AAA","Name":"Spring","MajorVersion":1,"MinorVersion":2,"PatchVersion":0,"BuildNumber":3,"IsManaged":true,"Package2ContainerOptions":"Managed"}}]}`)
	}
	expectRun(t, 0, `┌────────────────────┬────────────────────┬──────────────┬───────────┬──────────────────┬────────────────────┬──────────────┬─────────┐
│ ID                 │ Package ID         │ Package Name │ Namespace │ Version Settings │ Package Version ID │ Version Name │ Version │
├────────────────────┼────────────────────┼──────────────┼───────────┼──────────────────┼────────────────────┼──────────────┼─────────┤
│ 0A3000000000001AAA │ 033000000000001AAA │ Billing      │ acme      │ packageId        │ 04t000000000001AAA │ Spring       │ 1.2.0.3 │
└────────────────────┴────────────────────┴──────────────┴───────────┴──────────────────┴────────────────────┴──────────────┴─────────┘

`, "package", "installed", "list")
	out, _, _ := run("package", "installed", "list", "--json")
	if !strings.Contains(out, `"SubscriberPackageVersionNumber": "1.2.0.3"`) || !strings.Contains(out, `"VersionSettings": "packageId"`) {
		t.Errorf("json:\n%s", out)
	}
}

func TestPackageInstallAndUninstall(t *testing.T) {
	org := devHubSetup(t)
	var install, uninstall map[string]interface{}
	installStatus := "SUCCESS"
	org.handlers["/services/data/v67.0/tooling/sobjects/PackageInstallRequest"] = func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&install)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"0Hf000000000001AAA","success":true,"errors":[]}`)
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/PackageInstallRequest/0Hf000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		errors := "null"
		if installStatus == "ERROR" {
			errors = `{"errors":[{"message":"Missing dependency"}]}`
		}
		io.WriteString(w, `{"Id":"0Hf000000000001AAA","Status":"`+installStatus+`","SubscriberPackageVersionKey":"04t000000000001AAA","Errors":`+errors+`}`)
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/SubscriberPackageVersionUninstallRequest"] = func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&uninstall)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"06y000000000001AAA","success":true,"errors":[]}`)
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/SubscriberPackageVersionUninstallRequest/06y000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"Id":"06y000000000001AAA","Status":"Success","SubscriberPackageVersionId":"04t000000000001AAA"}`)
	}

	out, errOut, status := run("package", "install", "-p", "billing@1.0.0-1", "-w", "5", "-k", "secret", "-s", "AllUsers", "-r")
	if status != 0 || out != "Successfully installed package [04t000000000001AAA]\n" || errOut != "Waiting 5 minutes for package install to complete.... done\n" {
		t.Errorf("install: status %d, stdout %q, stderr %q", status, out, errOut)
	}
	if install["SubscriberPackageVersionKey"] != "04t000000000001AAA" || install["Password"] != "secret" || install["SecurityType"] != "full" ||
		install["UpgradeType"] != "mixed-mode" || install["ApexCompileType"] != "all" {
		t.Errorf("install request = %v", install)
	}

	installStatus = "IN_PROGRESS"
	expectRun(t, 0, "PackageInstallRequest is currently InProgress. You can continue to query the status using\nsf package:install:report -i 0Hf000000000001AAA -o me@example.com\n",
		"package", "install", "--package", "04t000000000001AAA")

	installStatus = "ERROR"
	_, errOut, status = run("package", "install", "-p", "04t000000000001AAA", "-w", "1")
	if status != 1 || !strings.Contains(errOut, "Error (PackageInstallError): Encountered errors installing the package! Installation errors: \n(1) Missing dependency") {
		t.Errorf("install error: status %d, stderr %q", status, errOut)
	}

	expectRun(t, 0, "Successfully uninstalled package [04t000000000001AAA]\n", "package", "uninstall", "-p", "billing@1.0.0-1", "-w", "1")
	if uninstall["SubscriberPackageVersionId"] != "04t000000000001AAA" {
		t.Errorf("uninstall request = %v", uninstall)
	}
}

const packageVersionRecord = `{"Id":"05i000000000001AAA","Package2Id":"0Ho000000000001AAA","SubscriberPackageVersionId":"04t000000000001AAA","Name":"Spring",` +
	`"Package2":{"Name":"Billing","NamespacePrefix":"acme","IsOrgDependent":false,"ContainerOptions":"Managed"},` +
	`"Description":"First","Tag":null,"Branch":null,"MajorVersion":1,"MinorVersion":0,"PatchVersion":0,"BuildNumber":1,"IsReleased":false,` +
	`"CreatedDate":"2026-01-02T03:04:05.000+0000","LastModifiedDate":"2026-01-03T03:04:05.000+0000","IsPasswordProtected":false,"AncestorId":null,` +
	`"ValidationSkipped":false,"ValidatedAsync":false,"CreatedById":"005000000000001AAA","ConvertedFromVersionId":null,"ReleaseVersion":67.0,` +
	`"BuildDurationInSeconds":120,"HasMetadataRemoved":false,"Language":"en_US","CodeCoverage":{"apexCodeCoveragePercentage":85},"HasPassedCodeCoverageCheck":true}`

func TestPackageVersionListReportAndPromote(t *testing.T) {
	org := devHubSetup(t)
	var soql string
	var promoted string
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		soql = r.URL.Query().Get("q")
		io.WriteString(w, `{"done":true,"totalSize":1,"records":[`+packageVersionRecord+`]}`)
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/Package2Version/05i000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		promoted = string(body)
		w.WriteHeader(http.StatusNoContent)
	}

	expectRun(t, 0, "Package Versions [1]\n"+
		"┌────────────────────┬─────────┬───────────────────────────────┬──────────┐\n"+
		"│ Package Id         │ Version │ Subscriber Package Version Id │ Released │\n"+
		"├────────────────────┼─────────┼───────────────────────────────┼──────────┤\n"+
		"│ 0Ho000000000001AAA │ 1.0.0.1 │ 04t000000000001AAA            │ false    │\n"+
		"└────────────────────┴─────────┴───────────────────────────────┴──────────┘\n\n",
		"package", "version", "list", "--concise", "-p", "billing", "--released")
	if !strings.Contains(soql, "Package2Id IN ('0Ho000000000001AAA')") || !strings.Contains(soql, "IsReleased = true") || !strings.Contains(soql, "IsDeprecated != true") {
		t.Errorf("query = %s", soql)
	}

	out, _, status := run("package", "version", "list")
	if status != 0 || !strings.HasPrefix(out, "Package Versions [1]\n") ||
		!strings.Contains(out, "│ Package Name │ Namespace │ Version Name │ Version │ Subscriber Package Version Id │ Alias           │ Installation Key │ Released │ Validation Skipped │ Validated Async │ Ancestor │ Ancestor Version │ Branch │") ||
		!strings.Contains(out, "│ Billing      │ acme      │ Spring       │ 1.0.0.1 │ 04t000000000001AAA            │ billing@1.0.0-1 │ false            │ false    │") {
		t.Errorf("version list: status %d\n%s", status, out)
	}
	out, _, _ = run("package", "version", "list", "--json")
	if !strings.Contains(out, `"IsReleased": false`) || !strings.Contains(out, `"CreatedDate": "2026-01-02 03:04"`) ||
		!strings.Contains(out, `"InstallUrl": "https://login.salesforce.com/packaging/installPackage.apexp?p0=04t000000000001AAA"`) ||
		!strings.Contains(out, `"HasMetadataRemoved": "No"`) || !strings.Contains(out, `"ReleaseVersion": "67.0"`) ||
		!strings.Contains(out, `"CodeCoverage": "use --verbose for code coverage"`) {
		t.Errorf("version list json:\n%s", out)
	}
	// sf reads these only for --verbose and leaves out keys without a value.
	for _, absent := range []string{`"Language"`, `"HasPassedCodeCoverageCheck"`, `"AncestorVersion"`} {
		if strings.Contains(out, absent) {
			t.Errorf("version list json has %s:\n%s", absent, out)
		}
	}
	if strings.Contains(soql, "CodeCoverage") {
		t.Errorf("query without --verbose reads the code coverage: %s", soql)
	}
	out, _, _ = run("package", "version", "list", "--verbose")
	if !strings.Contains(soql, ", CodeCoverage, HasPassedCodeCoverageCheck, Language FROM") ||
		!strings.Contains(out, "│ Code Coverage │ Code Coverage Met │ Converted From Version Id │ Org-Dependent Unlocked Package │ Release Version │ Build Duration in Seconds │ Managed Metadata Removed │ Created By         │ Language │") ||
		!strings.Contains(out, "│ 85%           │ true              │                           │ N/A                            │ 67.0            │ 120                       │ No                       │ 005000000000001AAA │ en_US    │") {
		t.Errorf("verbose version list: query %s\n%s", soql, out)
	}
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Query().Get("q"), "SubscriberPackageVersionId IN ('04t000000000009AAA')") {
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"SubscriberPackageVersionId":"04t000000000009AAA","MajorVersion":0,"MinorVersion":9,"PatchVersion":0,"BuildNumber":2}]}`)
			return
		}
		io.WriteString(w, `{"done":true,"totalSize":1,"records":[`+strings.Replace(packageVersionRecord, `"AncestorId":null`, `"AncestorId":"04t000000000009AAA"`, 1)+`]}`)
	}
	out, _, _ = run("package", "version", "list", "--json")
	if !strings.Contains(out, "\"AncestorId\": \"04t000000000009AAA\",\n      \"AncestorVersion\": \"0.9.0.2\",") {
		t.Errorf("ancestor version:\n%s", out)
	}
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"done":true,"totalSize":0,"records":[]}`)
	}
	_, errOut, status := run("package", "version", "report", "-p", "04t000000000000AAA")
	if status != 1 || errOut != "Error (ErrorInvalidIdNoMatchingVersionIdError): The Subscriber Package Version Id 04t000000000000AAA is invalid, as a corresponding Package Version Id was not found\n\n\n" {
		t.Errorf("missing version: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("package", "version", "create", "report", "-i", "08c000000000000AAA")
	if status != 10 || errOut != "Error (10): Cannot read properties of undefined (reading 'Status')\n\n\n" {
		t.Errorf("missing request: status %d, stderr %q", status, errOut)
	}
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		soql = r.URL.Query().Get("q")
		io.WriteString(w, `{"done":true,"totalSize":1,"records":[`+packageVersionRecord+`]}`)
	}

	var reportQueries []string
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		soql = r.URL.Query().Get("q")
		reportQueries = append(reportQueries, soql)
		io.WriteString(w, `{"done":true,"totalSize":1,"records":[`+packageVersionRecord+`]}`)
	}
	out, _, status = run("package", "version", "report", "-p", "04t000000000001AAA")
	for _, row := range []string{
		"│ Key                            │ Value              │",
		"│ Subscriber Package Version Id  │ 04t000000000001AAA │",
		"│ Version                        │ 1.0.0.1            │",
		"│ Released                       │ false              │",
		"│ Code Coverage                  │ 85.00%             │",
		"│ Org-Dependent Unlocked Package │ No                 │",
		"│ Release Version                │ 67.0               │",
		"│ Created By                     │ 005000000000001AAA │",
	} {
		if !strings.Contains(out, row) {
			t.Errorf("version report lacks %q\n%s", row, out)
		}
	}
	if status != 0 || !strings.HasPrefix(out, "Package Version\n") || strings.Contains(out, "│ Id ") || strings.Contains(out, "Dependencies") {
		t.Errorf("version report: status %d\n%s", status, out)
	}
	if len(reportQueries) < 2 || !strings.Contains(reportQueries[0], "WHERE SubscriberPackageVersionId = '04t000000000001AAA'") ||
		!strings.Contains(reportQueries[1], "WHERE Id = '05i000000000001AAA' AND IsDeprecated != true") || strings.Contains(reportQueries[1], "CodeCoveragePercentages") {
		t.Errorf("report queries = %q", reportQueries)
	}
	reportQueries = nil
	out, _, _ = run("package", "version", "report", "-p", "04t000000000001AAA", "--verbose")
	for _, row := range []string{"│ Id ", "│ Dependencies ", "│ Converted From Version Id ", "│ End To End Build Duration In Seconds ", "│ Code Coverage Details "} {
		if !strings.Contains(out, row) {
			t.Errorf("verbose version report lacks %q\n%s", row, out)
		}
	}
	if len(reportQueries) < 2 || !strings.Contains(reportQueries[1], ",SubscriberPackageVersion.Dependencies,CodeCoveragePercentages,EndToEndBuildDurationInSeconds FROM") {
		t.Errorf("verbose report queries = %q", reportQueries)
	}
	out, _, _ = run("package", "version", "report", "-p", "04t000000000001AAA", "--json")
	for _, field := range []string{`"Version": "1.0.0.1"`, `"AncestorId": "N/A"`, `"HasMetadataRemoved": "N/A"`, `"IsOrgDependent": "No"`, `"ConvertedFromVersionId": " "`} {
		if !strings.Contains(out, field) {
			t.Errorf("version report JSON lacks %s\n%s", field, out)
		}
	}

	expectRun(t, 0, "Successfully promoted the package version, ID: 04t000000000001AAA, to released.\n", "package", "version", "promote", "-p", "05i000000000001AAA", "--no-prompt")
	if promoted != `{"IsReleased":true}` {
		t.Errorf("promote request = %s", promoted)
	}
	// Without --no-prompt the command asks, and an empty answer is "no".
	_, errOut, status = run("package", "version", "promote", "-p", "05i000000000001AAA")
	if status != 1 || !strings.Contains(errOut, "Error (promote-deny): Promote operation denied") {
		t.Errorf("declined promote: status %d, stderr %q", status, errOut)
	}
}

func TestPackageVersionCreateRequiresAPackageDirectory(t *testing.T) {
	devHubSetup(t)
	_, errOut, status := run("package", "version", "create", "-x")
	if status != 1 || !strings.Contains(errOut, "Either --package or --path is required") {
		t.Errorf("status %d, stderr %q", status, errOut)
	}
	// As in sf, the command needs an installation key or the flag that
	// says the version has none.
	for _, args := range [][]string{{"-p", "billing", "-d", "force-app"}, {"-p", "billing", "-d", "force-app", "-k", "key", "-x"}} {
		_, errOut, status = run(append([]string{"package", "version", "create"}, args...)...)
		if status != 2 || !strings.Contains(errOut, "Exactly one of the following must be provided: --installation-key, --installation-key-bypass") {
			t.Errorf("%v: status %d, stderr %q", args, status, errOut)
		}
	}
	_, errOut, status = run("package", "version", "create", "-p", "billing", "-d", "force-app", "-x", "--post-install-url", "not a url")
	if status != 1 || !strings.Contains(errOut, "Error (MalformedUrlError): The postInstallUrl value \"not a url\"") {
		t.Errorf("post install url: status %d, stderr %q", status, errOut)
	}
}
