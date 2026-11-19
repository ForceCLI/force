package sf

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestOrgListShape(t *testing.T) {
	useTempHome(t)
	_, errOut, status := run("org", "list", "shape")
	if status != 1 || errOut != "Error (NoAuthFoundError): No authenticated orgs found.\n\n\n" {
		t.Errorf("no orgs: status %d, stderr %q", status, errOut)
	}
	org := newFakeOrg(t)
	org.login(t, "--alias", "source")
	org.queries["FROM ShapeRepresentation WHERE Status IN"] = nil
	// Compared with sf.
	expectRun(t, 0, "\nNo org shapes found.\n", "org", "list", "shape")

	org.queries["FROM ShapeRepresentation WHERE Status IN"] = []map[string]interface{}{{
		"Id": "3SR000000000001AAA", "Status": "Active", "CreatedBy": map[string]interface{}{"Username": "me@example.com"}, "CreatedDate": "2026-01-02T03:04:05.000+0000",
	}}
	out, _, status := run("org", "list", "shape")
	if status != 0 || !strings.HasPrefix(out, "Org Shapes\n") ||
		!strings.Contains(out, "│ Org Id ") || !strings.Contains(out, "│ Username       │ Shape Id           │ Status │ Created By     │ Created Date                 │ Alias  │") ||
		!strings.Contains(out, "│ me@example.com │ 3SR000000000001AAA │ Active │ me@example.com │ 2026-01-02T03:04:05.000+0000 │ source │") {
		t.Errorf("list: status %d\n%s", status, out)
	}
	out, _, _ = run("force:org:shape:list", "--json")
	for _, want := range []string{`"username": "me@example.com"`, `"shapeId": "3SR000000000001AAA"`, `"status": "Active"`, `"createdBy": "me@example.com"`, `"alias": "source"`} {
		if !strings.Contains(out, want) {
			t.Errorf("json lacks %s:\n%s", want, out)
		}
	}

	// An org without the ShapeRepresentation object has no shapes.  Another
	// error is reported as a warning.
	failure := `[{"message":"sObject type 'ShapeRepresentation' is not supported.","errorCode":"INVALID_TYPE"}]`
	org.handlers["/services/data/v67.0/query"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, failure)
	}
	out, errOut, status = run("org", "list", "shape")
	if status != 0 || out != "\nNo org shapes found.\n" || errOut != "" {
		t.Errorf("unsupported: status %d, stdout %q, stderr %q", status, out, errOut)
	}
	failure = `[{"message":"The org is locked.","errorCode":"ORG_LOCKED"}]`
	out, errOut, status = run("org", "list", "shape")
	if status != 0 || out != "\nNo org shapes found.\n" || errOut != "Warning: Org: me@example.com: The org is locked.\n" {
		t.Errorf("failing org: status %d, stdout %q, stderr %q", status, out, errOut)
	}
}

func TestOrgCreateAndDeleteShape(t *testing.T) {
	useTempHome(t)
	org := newFakeOrg(t)
	org.login(t, "--alias", "source", "--set-default")
	enabled := false
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		if enabled {
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"IsShapeExportPrefEnabled":true}]}`)
			return
		}
		io.WriteString(w, `{"done":true,"totalSize":0,"records":[]}`)
	}
	_, errOut, status := run("org", "create", "shape")
	if status != 1 || errOut != "Error (ShapeRepresentationNoAccessError): The org needs to be enabled for org shape before one can be created. me@example.com\n\n\n" {
		t.Errorf("create without access: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("org", "delete", "shape", "--no-prompt")
	if status != 1 || errOut != "Error (NoAccessError): The org with name: me@example.com needs to be enabled for org shape before shapes can be deleted.\n\n\n" {
		t.Errorf("delete without access: status %d, stderr %q", status, errOut)
	}

	enabled = true
	created := ""
	org.handlers["/services/data/v67.0/sobjects/ShapeRepresentation"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		created = r.Method + " " + string(body)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"3SR000000000001AAA","success":true,"errors":[]}`)
	}
	expectRun(t, 0, "Successfully created org shape for 3SR000000000001AAA.\n", "org", "create", "shape")
	if created != `POST {"Description":""}` {
		t.Errorf("create request = %q", created)
	}

	org.queries["SELECT Id FROM ShapeRepresentation"] = nil
	out, _, status := run("org", "delete", "shape", "--no-prompt")
	if status != 0 || !strings.HasPrefix(out, "Can't delete org shape. No org shape found for org ") {
		t.Errorf("delete without shapes: status %d\n%s", status, out)
	}
	// Without --no-prompt the command asks, and an empty answer is "no".
	if out, errOut, status := run("org", "delete", "shape"); status != 0 || out != "" || !strings.Contains(errOut, "Delete shapes for org with name: me@example.com? Are you sure (y/n)?") {
		t.Errorf("declined: status %d, stdout %q, stderr %q", status, out, errOut)
	}

	org.queries["SELECT Id FROM ShapeRepresentation"] = []map[string]interface{}{{"Id": "3SR000000000001AAA"}, {"Id": "3SR000000000002AAA"}}
	var deleted []string
	org.handlers["/services/data/v67.0/sobjects/ShapeRepresentation/3SR000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		deleted = append(deleted, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}
	org.handlers["/services/data/v67.0/sobjects/ShapeRepresentation/3SR000000000002AAA"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `[{"message":"The shape is in use.","errorCode":"DELETE_FAILED"}]`)
	}
	out, _, status = run("org", "delete", "shape", "--no-prompt")
	if status != 68 || len(deleted) != 1 || deleted[0] != "DELETE" || !strings.HasPrefix(out, "Successfully deleted org shape for ") ||
		!strings.Contains(out, "\n\nFailures\n") || !strings.Contains(out, "│ 3SR000000000002AAA │ The shape is in use. │") {
		t.Errorf("delete: status %d, requests %v\n%s", status, deleted, out)
	}
	out, _, status = run("org", "delete", "shape", "--no-prompt", "--json")
	if status != 68 || !strings.Contains(out, "\"shapeIds\": [\n      \"3SR000000000001AAA\"\n    ]") || !strings.Contains(out, `"message": "The shape is in use."`) || !strings.Contains(out, `"status": 68`) {
		t.Errorf("delete json: status %d\n%s", status, out)
	}
}
