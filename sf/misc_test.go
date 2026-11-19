package sf

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The output formats in this file were captured from sf 2.138.6.

func TestSobjectList(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/data/v67.0/sobjects"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"encoding":"UTF-8","sobjects":[{"name":"Account","custom":false},{"name":"Invoice__c","custom":true},{"name":"Contact","custom":false}]}`)
	}
	expectRun(t, 0, "Account\nInvoice__c\nContact\n", "sobject", "list")
	expectRun(t, 0, "Account\nContact\n", "sobject", "list", "-s", "standard")
	expectRun(t, 0, `{
  "status": 0,
  "result": [
    "Invoice__c"
  ],
  "warnings": [
    "The \"force schema sobject list\" command has been deprecated. Use \"sobject list\" instead."
  ]
}
`, "force:schema:sobject:list", "--sobject", "CUSTOM", "--json")
	org.handlers["/services/data/v67.0/sobjects"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"sobjects":[]}`)
	}
	expectRun(t, 0, "No CUSTOM objects found.\n", "sobject", "list", "-s", "custom")
}

func TestSobjectDescribeKeepsFieldOrder(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/data/v67.0/sobjects/Account/describe"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"name":"Account","custom":false,"fields":[{"name":"Id","type":"id"}]}`)
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/ApexClass/describe"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"name":"ApexClass"}`)
	}
	expectRun(t, 0, `{
  "name": "Account",
  "custom": false,
  "fields": [
    {
      "name": "Id",
      "type": "id"
    }
  ]
}
`, "sobject", "describe", "-s", "Account")
	out, _, status := run("sobject", "describe", "-s", "ApexClass", "--use-tooling-api", "--json")
	if status != 0 || !strings.Contains(out, `"name": "ApexClass"`) || !strings.Contains(out, `"status": 0`) {
		t.Errorf("tooling describe: status %d\n%s", status, out)
	}
}

func TestApiRequestRest(t *testing.T) {
	org := loggedInFakeOrg(t)
	var method, body, contentType, custom string
	org.handlers["/services/data/v67.0/sobjects/Account"] = func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		method, body, contentType, custom = r.Method, string(data), r.Header.Get("Content-Type"), r.Header.Get("X-Custom")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"001000000000001AAA","success":true,"errors":[]}`)
	}
	org.handlers["/services/data/v67.0/limits"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"DailyApiRequests":{"Max":15000,"Remaining":14998}}`)
	}
	org.handlers["/plain"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "not json")
	}

	out, errOut, status := run("api", "request", "rest", "/services/data/v67.0/limits")
	want := "{\n  \"DailyApiRequests\": {\n    \"Max\": 15000,\n    \"Remaining\": 14998\n  }\n}\n"
	if status != 0 || out != want {
		t.Errorf("GET: status %d, stdout %q", status, out)
	}
	if errOut != "Warning: This command is currently in beta. Any aspect of this command can change without advanced notice. Don't use beta commands in your scripts.\n" {
		t.Errorf("stderr = %q", errOut)
	}

	out, _, status = run("api", "request", "rest", "services/data/v67.0/sobjects/Account", "-X", "POST", "-b", `{"Name":"ViaRest"}`, "-H", "X-Custom: yes")
	if status != 0 || method != "POST" || body != `{"Name":"ViaRest"}` || contentType != "application/json" || custom != "yes" {
		t.Errorf("POST: status %d, method %s, body %s, content type %s, header %s", status, method, body, contentType, custom)
	}
	if out != "{\n  \"id\": \"001000000000001AAA\",\n  \"success\": true,\n  \"errors\": []\n}\n" {
		t.Errorf("POST output = %q", out)
	}

	out, _, status = run("api", "request", "rest", "/services/data/v67.0/nosuch")
	if status != 1 || !strings.Contains(out, `"errorCode": "NOT_FOUND"`) {
		t.Errorf("error response: status %d, stdout %q", status, out)
	}
	expectRun(t, 0, "not json\n", "api", "request", "rest", "/plain")

	out, _, _ = run("api", "request", "rest", "/services/data/v67.0/limits", "--include")
	if !strings.HasPrefix(out, "HTTP/1.1 200\n") || !strings.Contains(out, "content-length: ") {
		t.Errorf("--include output = %q", out)
	}

	file := filepath.Join(t.TempDir(), "limits.json")
	expectRun(t, 0, "File saved to "+file+"\n", "api", "request", "rest", "/services/data/v67.0/limits", "-S", file)
	if saved, _ := os.ReadFile(file); !strings.Contains(string(saved), "DailyApiRequests") {
		t.Errorf("saved file = %q", saved)
	}

	_, errOut, status = run("api", "request", "rest", "/x", "-X", "FETCH")
	if status != 1 || !strings.Contains(errOut, `"FETCH" must be one of GET, POST, PUT, PATCH, HEAD, DELETE, OPTIONS, TRACE`) {
		t.Errorf("bad method: status %d, stderr %q", status, errOut)
	}
}

func TestTemplateGenerateApexClass(t *testing.T) {
	home := useTempHome(t)
	project := newProject(t, home)
	dir := filepath.Join(home, "gen")
	expectRun(t, 0, "target dir = "+dir+"\n  create ../gen/MyClass.cls\n  create ../gen/MyClass.cls-meta.xml\n\n",
		"template", "generate", "apex", "class", "-n", "MyClass", "-d", dir)
	class, _ := os.ReadFile(filepath.Join(dir, "MyClass.cls"))
	if string(class) != "public with sharing class MyClass {\n    public MyClass() {\n\n    }\n}" {
		t.Errorf("class = %q", class)
	}
	meta, _ := os.ReadFile(filepath.Join(dir, "MyClass.cls-meta.xml"))
	if string(meta) != "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <apiVersion>66.0</apiVersion>\n    <status>Active</status>\n</ApexClass>\n" {
		t.Errorf("meta = %q", meta)
	}

	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "outputDir": "`+project+`",
    "created": [
      "Other.cls",
      "Other.cls-meta.xml"
    ],
    "rawOutput": "target dir = `+project+`\n  create Other.cls\n  create Other.cls-meta.xml\n"
  },
  "warnings": [
    "The \"apex generate class\" command has been deprecated. Use \"template generate apex class\" instead."
  ]
}
`, "apex:generate:class", "--name", "Other", "--api-version", "60.0", "--json")
	meta, _ = os.ReadFile(filepath.Join(project, "Other.cls-meta.xml"))
	if !strings.Contains(string(meta), "<apiVersion>60.0</apiVersion>") {
		t.Errorf("--api-version is not written to the meta file: %s", meta)
	}
}

func TestTemplateGenerateApexClassFromOtherTemplates(t *testing.T) {
	home := useTempHome(t)
	chdir(t, home)
	if _, errOut, status := run("template", "generate", "apex", "class", "-n", "MyError", "-t", "ApexException"); status != 0 {
		t.Fatalf("status %d, %s", status, errOut)
	}
	class, _ := os.ReadFile("MyError.cls")
	if !strings.Contains(string(class), "class MyError extends Exception") {
		t.Errorf("MyError.cls = %q", class)
	}
	_, errOut, status := run("template", "generate", "apex", "class", "-n", "X", "-t", "NoSuchTemplate")
	if status != 1 || !strings.Contains(errOut, "Expected --template=NoSuchTemplate to be one of: ApexException, ApexUnitTest, BasicUnitTest, DefaultApexClass, InboundEmailService") {
		t.Errorf("unknown template: status %d, stderr %q", status, errOut)
	}
}

func TestTemplateGenerateTriggerAndVisualforce(t *testing.T) {
	home := useTempHome(t)
	chdir(t, home)
	if _, errOut, status := run("template", "generate", "apex", "trigger", "-n", "AccountTrigger", "-s", "Account", "-e", "before insert,after update"); status != 0 {
		t.Fatalf("status %d, %s", status, errOut)
	}
	trigger, _ := os.ReadFile("AccountTrigger.trigger")
	if string(trigger) != "trigger AccountTrigger on Account (before insert,after update) {\n\n}" {
		t.Errorf("trigger = %q", trigger)
	}
	meta, _ := os.ReadFile("AccountTrigger.trigger-meta.xml")
	if !strings.Contains(string(meta), "\n  <apiVersion>66.0</apiVersion>\n  <status>Active</status>\n</ApexTrigger>\n") {
		t.Errorf("trigger meta = %q", meta)
	}
	_, errOut, status := run("template", "generate", "apex", "trigger", "-n", "X", "-e", "sometimes")
	if status != 1 || !strings.Contains(errOut, "Expected --event=sometimes to be one of: before insert,") {
		t.Errorf("bad event: status %d, stderr %q", status, errOut)
	}

	expectRun(t, 0, "target dir = "+home+"\n  create MyPage.page\n  create MyPage.page-meta.xml\n\n",
		"template", "generate", "visualforce", "page", "-n", "MyPage", "-l", "My Page")
	page, _ := os.ReadFile("MyPage.page")
	if !strings.HasPrefix(string(page), "<apex:page >\n") || !strings.HasSuffix(string(page), "</apex:page>") {
		t.Errorf("page = %q", page)
	}
	meta, _ = os.ReadFile("MyPage.page-meta.xml")
	if !strings.Contains(string(meta), "<label>My Page</label>") {
		t.Errorf("page meta = %q", meta)
	}
	if _, _, status := run("template", "generate", "visualforce", "component", "-n", "MyComp", "-l", "My Comp"); status != 0 {
		t.Errorf("component: status %d", status)
	}
	component, _ := os.ReadFile("MyComp.component-meta.xml")
	if !strings.Contains(string(component), "<ApexComponent ") || !strings.Contains(string(component), "<label>My Comp</label>") {
		t.Errorf("component meta = %q", component)
	}
	_, errOut, status = run("template", "generate", "visualforce", "page", "-n", "NoLabel")
	if status != 2 || errOut != "Error (2): The following error occurred:\n  Missing required flag label\nSee more help with --help\n\n\n" {
		t.Errorf("missing label: status %d, stderr %q", status, errOut)
	}
}

func TestDataSearch(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/data/v67.0/search"] = func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Query().Get("q"), "Nothing") {
			io.WriteString(w, `{"searchRecords":[]}`)
			return
		}
		io.WriteString(w, `{"searchRecords":[`+
			`{"attributes":{"type":"Account","url":"/a"},"Id":"001000000000001AAA","Name":"Acme"},`+
			`{"attributes":{"type":"Contact","url":"/c"},"Id":"003000000000001AAA","LastName":"Lovelace"},`+
			`{"attributes":{"type":"Account","url":"/b"},"Id":"001000000000002AAA","Name":"Acme West"}]}`)
	}
	out, errOut, status := run("data", "search", "-q", "FIND {Acme}")
	want := `Account
┌────────────────────┬───────────┐
│ Id                 │ Name      │
├────────────────────┼───────────┤
│ 001000000000001AAA │ Acme      │
│ 001000000000002AAA │ Acme West │
└────────────────────┴───────────┘


Contact
┌────────────────────┬──────────┐
│ Id                 │ LastName │
├────────────────────┼──────────┤
│ 003000000000001AAA │ Lovelace │
└────────────────────┴──────────┘


`
	if status != 0 || out != want || errOut != "Querying Data... done\n" {
		t.Errorf("status %d, stdout\n%s\nstderr %q", status, out, errOut)
	}
	expectRun(t, 0, "No Records Found\n", "data", "search", "--query", "FIND {Nothing}")
	out, _, status = run("data", "search", "-q", "FIND {Acme}", "--json")
	if status != 0 || !strings.Contains(out, "\"result\": {\n    \"searchRecords\": [\n      {\n        \"attributes\": {") {
		t.Errorf("json: status %d\n%s", status, out)
	}

	chdir(t, t.TempDir())
	expectRun(t, 0, "Written to Account.csv\nWritten to Contact.csv\n", "data", "search", "-q", "FIND {Acme}", "-r", "csv")
	written, _ := os.ReadFile("Account.csv")
	if string(written) != "Id,Name\n001000000000001AAA,Acme\n001000000000002AAA,Acme West" {
		t.Errorf("Account.csv = %q", written)
	}
}

func TestUnknownNestedCommandNamesTheTopic(t *testing.T) {
	_, errOut, status := run("apex", "nosuch", "thing")
	want := " ›   Warning: apex nosuch thing is not a sf command.\n ›   Error: Run sf help apex for a list of available commands.\n"
	if status != 127 || errOut != want {
		t.Errorf("status %d, stderr %q", status, errOut)
	}
}
