package sf

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrgOpenURLOnly(t *testing.T) {
	org := loggedInFakeOrg(t)
	var redirect string
	org.handlers["/services/oauth2/singleaccess"] = func(w http.ResponseWriter, r *http.Request) {
		redirect = r.URL.Query().Get("redirect_uri")
		io.WriteString(w, `{"frontdoor_uri":"https://example.my.salesforce.com/secur/frontdoor.jsp?otp=abc"}`)
	}
	out, errOut, status := run("org", "open", "--url-only", "--path", "/lightning/setup/SetupOneHome/home")
	want := "\nAccess org 00D000000000000 as user me@example.com with the following URL: https://example.my.salesforce.com/secur/frontdoor.jsp?otp=abc\n"
	if status != 0 || out != want {
		t.Errorf("status %d, stdout %q", status, out)
	}
	if redirect != "/lightning/setup/SetupOneHome/home" {
		t.Errorf("redirect_uri = %q", redirect)
	}
	if !strings.HasPrefix(errOut, "Warning: This command will expose sensitive information") {
		t.Errorf("stderr = %q", errOut)
	}

	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "orgId": "00D000000000000",
    "url": "https://example.my.salesforce.com/secur/frontdoor.jsp?otp=abc",
    "username": "me@example.com"
  },
  "warnings": [
    "This command will expose sensitive information that allows for subsequent activity using your current authenticated session.\nSharing this information is equivalent to logging someone in under the current credential, resulting in unintended access and escalation of privilege.\nFor additional information, please review the authorization section of the https://developer.salesforce.com/docs/atlas.en-us.sfdx_dev.meta/sfdx_dev/sfdx_dev_auth_web_flow.htm."
  ]
}
`, "org", "open", "-r", "--json")
}

func TestOrgOpenStartsTheBrowser(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/oauth2/singleaccess"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"frontdoor_uri":"https://example.my.salesforce.com/secur/frontdoor.jsp?otp=abc"}`)
	}
	var opened string
	previous := openBrowser
	openBrowser = func(url string) error {
		opened = url
		return nil
	}
	defer func() { openBrowser = previous }()
	expectRun(t, 0, "Opening org 00D000000000000 as user me@example.com\n", "org", "open")
	if opened != "https://example.my.salesforce.com/secur/frontdoor.jsp?otp=abc" {
		t.Errorf("opened %q", opened)
	}
}

func TestOrgOpenSourceFileAndBrowser(t *testing.T) {
	org := loggedInFakeOrg(t)
	var redirect string
	org.handlers["/services/oauth2/singleaccess"] = func(w http.ResponseWriter, r *http.Request) {
		redirect = r.URL.Query().Get("redirect_uri")
		io.WriteString(w, `{"frontdoor_uri":"https://example.my.salesforce.com/secur/frontdoor.jsp?otp=abc"}`)
	}
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		switch {
		case strings.Contains(query, "FROM ApexClass WHERE Name = 'Greeter'"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"01p000000000001AAA"}]}`)
		case strings.Contains(query, "FROM CustomObject WHERE DeveloperName = 'Invoice'"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"01I000000000001AAA"}]}`)
		case strings.Contains(query, "FROM flexipage WHERE DeveloperName='Home_Page'"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"0M0000000000001AAA"}]}`)
		default:
			io.WriteString(w, `{"done":true,"totalSize":0,"records":[]}`)
		}
	}
	org.queries["FROM FlowVersionView WHERE FlowDefinitionView.ApiName = 'Approve'"] = []map[string]interface{}{{"DurableId": "301000000000001AAA"}}
	dir := t.TempDir()
	file := func(name string) string {
		path := filepath.Join(dir, name)
		os.WriteFile(path, []byte("x"), 0o644)
		return path
	}
	for name, want := range map[string]string{
		"Greeter.cls":                  "lightning/setup/ApexClasses/page?address=%2F01p000000000001AAA",
		"Invoice__c.object-meta.xml":   "lightning/setup/ObjectManager/01I000000000001AAA/Details/view",
		"Welcome.page":                 "/apex/Welcome",
		"Approve.flow-meta.xml":        "/builder_platform_interaction/flowBuilder.app?flowId=301000000000001AAA",
		"Home_Page.flexipage-meta.xml": "/visualEditor/appBuilder.app?pageId=0M0000000000001AAA",
		"Admin.permissionset-meta.xml": "/lightning/setup/FlexiPageList/home",
		"notes.txt":                    "",
	} {
		redirect = "unset"
		if _, errOut, status := run("org", "open", "-r", "-f", file(name)); status != 0 || redirect != want {
			t.Errorf("%s: status %d, redirect %q, want %q, stderr %q", name, status, redirect, want, errOut)
		}
	}
	// A file that does not exist gives the home page of the org.
	redirect = "unset"
	if _, _, status := run("org", "open", "-r", "-f", filepath.Join(dir, "Missing.cls")); status != 0 || redirect != "" {
		t.Errorf("missing file: status %d, redirect %q", status, redirect)
	}
	_, errOut, status := run("org", "open", "-r", "-f", file("Unknown.cls"))
	if status != 1 || !strings.Contains(errOut, "Error (ApexClassIdNotFoundError): ID not found for Apex class ") {
		t.Errorf("class that is not in the org: status %d, stderr %q", status, errOut)
	}

	var opened []string
	previous := openInBrowser
	openInBrowser = func(browser, url string) error {
		opened = []string{browser, url}
		return nil
	}
	defer func() { openInBrowser = previous }()
	expectRun(t, 0, "Opening org 00D000000000000 as user me@example.com\n", "org", "open", "-b", "firefox")
	if len(opened) != 2 || opened[0] != "firefox" || opened[1] != "https://example.my.salesforce.com/secur/frontdoor.jsp?otp=abc" {
		t.Errorf("opened %q", opened)
	}
	var private string
	previousPrivate := openPrivately
	openPrivately = func(url string) error {
		private = url
		return nil
	}
	defer func() { openPrivately = previousPrivate }()
	expectRun(t, 0, "Opening org 00D000000000000 as user me@example.com\n", "org", "open", "--private")
	if private != "https://example.my.salesforce.com/secur/frontdoor.jsp?otp=abc" {
		t.Errorf("opened privately %q", private)
	}
	if _, errOut, status := run("org", "open", "--private", "-r"); status != 2 || !strings.Contains(errOut, "cannot also be provided when using --url-only") {
		t.Errorf("private with url-only: status %d, stderr %q", status, errOut)
	}
	if _, errOut, status := run("org", "open", "-b", "lynx"); status != 2 || !strings.Contains(errOut, "Expected --browser=lynx to be one of: chrome, edge, firefox") {
		t.Errorf("other browser: status %d, stderr %q", status, errOut)
	}
	if _, errOut, status := run("org", "open", "-b", "firefox", "-r"); status != 2 || !strings.Contains(errOut, "--browser=firefox cannot also be provided when using --url-only") {
		t.Errorf("browser with url-only: status %d, stderr %q", status, errOut)
	}
}

func TestOrgListLimits(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/data/v67.0/limits"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"DailyApiRequests":{"Max":15000,"Remaining":14998},"ActiveScratchOrgs":{"Max":3,"Remaining":3}}`)
	}
	expectRun(t, 0, `┌───────────────────┬───────────┬───────┐
│ Name              │ Remaining │ Max   │
├───────────────────┼───────────┼───────┤
│ DailyApiRequests  │ 14998     │ 15000 │
│ ActiveScratchOrgs │ 3         │ 3     │
└───────────────────┴───────────┴───────┘

`, "org", "list", "limits")
	expectRun(t, 0, `{
  "status": 0,
  "result": [
    {
      "name": "DailyApiRequests",
      "max": 15000,
      "remaining": 14998
    },
    {
      "name": "ActiveScratchOrgs",
      "max": 3,
      "remaining": 3
    }
  ],
  "warnings": [
    "The \"force limits api display\" command has been deprecated. Use \"org list limits\" instead."
  ]
}
`, "force:limits:api:display", "--json")
}

const soapEnvelope = `<?xml version="1.0" encoding="UTF-8"?><soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns="http://soap.sforce.com/2006/04/metadata"><soapenv:Body>%s</soapenv:Body></soapenv:Envelope>`

func TestOrgListMetadata(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/Soap/m/67.0"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/xml")
		if strings.Contains(string(body), "<type>ApexClass</type>") {
			io.WriteString(w, strings.Replace(soapEnvelope, "%s", `<listMetadataResponse>`+
				`<result><createdByName>Ada</createdByName><createdDate>2026-01-02T03:04:05.000Z</createdDate><fileName>classes/B.cls</fileName><fullName>B</fullName><id>01p000000000002AAA</id><lastModifiedByName>Ada</lastModifiedByName><lastModifiedDate>2026-02-03T00:00:00.000Z</lastModifiedDate><manageableState>unmanaged</manageableState><type>ApexClass</type></result>`+
				`<result><fileName>classes/A.cls</fileName><fullName>A</fullName><id>01p000000000001AAA</id><manageableState>installed</manageableState><namespacePrefix>ns</namespacePrefix><type>ApexClass</type></result>`+
				`</listMetadataResponse>`, 1))
			return
		}
		io.WriteString(w, strings.Replace(soapEnvelope, "%s", `<listMetadataResponse/>`, 1))
	}
	expectRun(t, 0, `ApexClass
┌────────────┬──────────────┬───────────┬────────────────────┬──────────────────┬───────────────┬──────────────────┬──────────────────┐
│ Created By │ Created Date │ Full Name │ Id                 │ Last Modified By │ Last Modified │ Manageable State │ Namespace Prefix │
├────────────┼──────────────┼───────────┼────────────────────┼──────────────────┼───────────────┼──────────────────┼──────────────────┤
│            │              │ A         │ 01p000000000001AAA │                  │               │ installed        │ ns               │
│ Ada        │ 2026-01-02   │ B         │ 01p000000000002AAA │ Ada              │ 2026-02-03    │ unmanaged        │                  │
└────────────┴──────────────┴───────────┴────────────────────┴──────────────────┴───────────────┴──────────────────┴──────────────────┘

`, "org", "list", "metadata", "-m", "ApexClass")

	out, _, status := run("org", "list", "metadata", "--metadata-type", "ApexClass", "--json")
	if status != 0 || !strings.Contains(out, `"fileName": "classes/A.cls",
      "fullName": "A",
      "id": "01p000000000001AAA",
      "manageableState": "installed",
      "namespacePrefix": "ns",
      "type": "ApexClass",
      "createdById": "",`) {
		t.Errorf("json: status %d\n%s", status, out)
	}

	out, errOut, status := run("org", "list", "metadata", "-m", "Flow")
	if status != 0 || out != "" || errOut != "Warning: No metadata found for type: Flow in org: me@example.com.\n" {
		t.Errorf("no results: status %d, stdout %q, stderr %q", status, out, errOut)
	}
	_, errOut, status = run("org", "list", "metadata")
	if status != 2 || errOut != "Error (2): The following error occurred:\n  Missing required flag metadata-type\nSee more help with --help\n\n\n" {
		t.Errorf("missing flag: status %d, stderr %q", status, errOut)
	}
}

func TestOrgListMetadataTypes(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/Soap/m/67.0"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		io.WriteString(w, strings.Replace(soapEnvelope, "%s", `<describeMetadataResponse><result>`+
			`<metadataObjects><childXmlNames>CustomField</childXmlNames><childXmlNames>ListView</childXmlNames><directoryName>objects</directoryName><inFolder>false</inFolder><metaFile>false</metaFile><suffix>object</suffix><xmlName>CustomObject</xmlName></metadataObjects>`+
			`<metadataObjects><directoryName>classes</directoryName><inFolder>false</inFolder><metaFile>true</metaFile><suffix>cls</suffix><xmlName>ApexClass</xmlName></metadataObjects>`+
			`<organizationNamespace>ns</organizationNamespace><partialSaveAllowed>true</partialSaveAllowed><testRequired>false</testRequired>`+
			`</result></describeMetadataResponse>`, 1))
	}
	expectRun(t, 0, `Metadata
┌──────────────┬──────────────────────────┬────────────────┬───────────┬───────────┬────────┐
│ Xml Names    │ Child Xml Names          │ Directory Name │ In Folder │ Meta File │ Suffix │
├──────────────┼──────────────────────────┼────────────────┼───────────┼───────────┼────────┤
│ ApexClass    │                          │ classes        │ false     │ true      │ cls    │
│ CustomObject │ [ CustomField ListView ] │ objects        │ false     │ false     │ object │
└──────────────┴──────────────────────────┴────────────────┴───────────┴───────────┴────────┘

Organizational Namespace: ns
Partial Save Allowed: true
Test Required: false
`, "org", "list", "metadata-types")
	out, _, status := run("org", "list", "metadata-types", "--json")
	if status != 0 || !strings.Contains(out, `"organizationNamespace": "ns"`) || strings.Contains(out, `"metadataVersion"`) || !strings.Contains(out, "\"result\": {\n    \"metadataObjects\": [") ||
		!strings.Contains(out, "\"xmlName\": \"CustomObject\",\n        \"childXmlNames\": [\n          \"CustomField\",\n          \"ListView\"\n        ]") {
		t.Errorf("json: status %d\n%s", status, out)
	}
}

func TestPrivateBrowserCommandUsesTheDefaultBrowser(t *testing.T) {
	name, args, err := privateBrowserCommand("darwin", "org.mozilla.firefox", "https://x")
	if err != nil || name != "open" || strings.Join(args, " ") != "-n -a firefox https://x --args --private-window" {
		t.Errorf("macOS firefox: %s %v, %v", name, args, err)
	}
	name, args, err = privateBrowserCommand("windows", "MSEdgeHTM", "https://x")
	if err != nil || name != "cmd" || strings.Join(args, " ") != "/c start  msedge --inPrivate https://x" {
		t.Errorf("Windows edge: %s %v, %v", name, args, err)
	}
	if _, args, err = privateBrowserCommand("windows", "FirefoxURL-308046B0AF4A39CB", "https://x"); err != nil || args[4] != "--private-window" {
		t.Errorf("Windows firefox: %v, %v", args, err)
	}
	if _, _, err = privateBrowserCommand("linux", "epiphany.desktop", "https://x"); err == nil || err.Error() != "epiphany.desktop is not supported as a default browser" {
		t.Errorf("other browser: %v", err)
	}
}
