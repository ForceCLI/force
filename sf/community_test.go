package sf

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCommunityListTemplate(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/data/v67.0/connect/communities/templates"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"templates":[{"publisher":"Salesforce","templateName":"Customer Service"},{"publisher":"Salesforce","templateName":"Build Your Own"}],"total":2}`)
	}
	expectRun(t, 0, `Site list template result
┌──────────────────┬────────────┐
│ Template Name    │ Publisher  │
├──────────────────┼────────────┤
│ Customer Service │ Salesforce │
│ Build Your Own   │ Salesforce │
└──────────────────┴────────────┘

Total 2
`, "community", "list", "template")
	out, _, _ := run("force:community:template:list", "--json")
	if !strings.Contains(out, `"templateName": "Customer Service"`) || !strings.Contains(out, `"total": 2`) {
		t.Errorf("json:\n%s", out)
	}
}

func TestCommunityCreate(t *testing.T) {
	org := loggedInFakeOrg(t)
	request := ""
	org.handlers["/services/data/v67.0/connect/communities"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		request = r.Method + " " + string(body)
		io.WriteString(w, `{"name":"Help","jobId":"08P000000000001AAA"}`)
	}
	out, errOut, status := run("community", "create", "-n", "Help", "-t", "Customer Service", "-p", "help", "-d", "Not sent", "templateParams.AuthenticationType=UNAUTHENTICATED")
	if status != 0 || !strings.HasPrefix(out, "Create Site Result\n") || !strings.Contains(out, "│ Your Site is being created. │ Help │ 08P000000000001AAA │") {
		t.Errorf("create: status %d, stderr %q\n%s", status, errOut, out)
	}
	if request != `POST {"name":"Help","urlPathPrefix":"help","templateName":"Customer Service","templateParams":{"AuthenticationType":"UNAUTHENTICATED"}}` {
		t.Errorf("request = %s", request)
	}
	out, _, _ = run("community", "create", "-n", "Help", "-t", "Customer Service", "--json")
	if !strings.Contains(out, `"jobId": "08P000000000001AAA"`) || !strings.Contains(out, `"action": "Site creation is under way. To track its status, query the BackgroundOperation object and include the jobId."`) ||
		request != `POST {"name":"Help","urlPathPrefix":"","templateName":"Customer Service","templateParams":{}}` {
		t.Errorf("json: request %s\n%s", request, out)
	}
	_, errOut, status = run("community", "create", "-n", "Help", "-t", "Customer Service", "color=blue")
	if status != 1 || errOut != "Error (InvalidArgument): Invalid argument(s): color\n\n\nTry this:\n\nArguments should start with templateParams, like templateParams.AuthenticationType=UNAUTHENTICATED\n\n" {
		t.Errorf("other argument: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("community", "create", "-n", "Help")
	if status != 2 || !strings.Contains(errOut, "Missing required flag template-name") {
		t.Errorf("no template: status %d, stderr %q", status, errOut)
	}
}

func TestCommunityPublish(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.queries["FROM NETWORK WHERE NAME = 'Help'"] = []map[string]interface{}{{"Id": "0DB000000000001AAA", "Status": "UnderConstruction"}}
	org.queries["FROM NETWORK WHERE NAME = 'Missing'"] = nil
	method := ""
	org.handlers["/services/data/v67.0/connect/communities/0DB000000000001AAA/publish"] = func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		io.WriteString(w, `{"id":"0DB000000000001AAA","name":"Help","url":"https://example.my.site.com"}`)
	}
	out, errOut, status := run("community", "publish", "-n", "Help")
	if status != 0 || method != "POST" || !strings.HasPrefix(out, "Publish Site Result\n") || !strings.Contains(out, "│ Help │ UnderConstruction │ https://example.my.site.com/ │") {
		t.Errorf("publish: status %d, method %s, stderr %q\n%s", status, method, errOut, out)
	}
	out, _, _ = run("community", "publish", "-n", "Help", "--json")
	if !strings.Contains(out, `"status": "UnderConstruction"`) || !strings.Contains(out, `"url": "https://example.my.site.com/"`) || strings.Contains(out, `"jobId"`) {
		t.Errorf("json:\n%s", out)
	}
	_, errOut, status = run("community", "publish", "-n", "Missing")
	if status != 1 || errOut != "Error (CommunityNotExistsError): The Missing site doesn't exist. Verify the site name and try publishing it again.\n\n\n" {
		t.Errorf("unknown site: status %d, stderr %q", status, errOut)
	}
}
