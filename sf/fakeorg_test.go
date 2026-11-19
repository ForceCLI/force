package sf

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// fakeOrg is an HTTP server that answers the Salesforce API requests the
// sf commands make.
type fakeOrg struct {
	*httptest.Server
	t *testing.T
	// queries maps a substring of a SOQL query to the records returned.
	// A query that matches no entry gets a 400 response.
	queries map[string][]map[string]interface{}
	// rawQueries maps a substring of a SOQL query to a raw JSON response
	// body, for tests that depend on the field order of records.
	rawQueries map[string]string
	// columnMetadata maps a substring of a SOQL query to the raw JSON
	// response to a request for the columns of the query.  A query that
	// matches no entry gets the records, without column metadata.
	columnMetadata map[string]string
	// handlers maps a URL path to a handler that replaces the default.
	handlers map[string]http.HandlerFunc
	requests []string
	// username is the user the userinfo resource reports.
	username string
	// refreshRequests holds the form values of each request to the OAuth
	// token endpoint.  The endpoint answers with fakeToken.
	refreshRequests []url.Values
}

const fakeToken = "00D000000000000!faketoken"

func newFakeOrg(t *testing.T) *fakeOrg {
	t.Helper()
	org := &fakeOrg{
		t: t,
		queries: map[string][]map[string]interface{}{
			"FROM Organization": {{
				"Name":                "Fake Organization",
				"InstanceName":        "NA1",
				"IsSandbox":           false,
				"TrialExpirationDate": nil,
				"NamespacePrefix":     nil,
				"OrganizationType":    "Developer Edition",
			}},
			"FROM User WHERE Username": {{
				"Id":      "005000000000001AAA",
				"Profile": map[string]interface{}{"Name": "System Administrator"},
			}},
		},
		username:   "me@example.com",
		rawQueries: map[string]string{},
		handlers:   map[string]http.HandlerFunc{},
	}
	org.Server = httptest.NewServer(http.HandlerFunc(org.serve))
	t.Cleanup(org.Close)
	return org
}

func (o *fakeOrg) serve(w http.ResponseWriter, r *http.Request) {
	o.requests = append(o.requests, r.Method+" "+r.URL.RequestURI())
	if r.URL.Path == "/services/oauth2/token" {
		r.ParseForm()
		o.refreshRequests = append(o.refreshRequests, r.PostForm)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"access_token": fakeToken, "instance_url": o.URL, "refresh_token": "new-refresh-token"})
		return
	}
	// SOAP requests carry the session ID in the request body.
	// Bulk API 1.0 requests carry it in the X-SFDC-Session header.
	isSOAP := strings.HasPrefix(r.URL.Path, "/services/Soap/")
	isBulkV1 := strings.HasPrefix(r.URL.Path, "/services/async/") && r.Header.Get("X-SFDC-Session") == fakeToken
	if !isSOAP && !isBulkV1 && r.Header.Get("Authorization") != "Bearer "+fakeToken {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode([]map[string]string{{"errorCode": "INVALID_SESSION_ID", "message": "Session expired or invalid"}})
		return
	}
	if handler, ok := o.handlers[r.URL.Path]; ok {
		handler(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.URL.Path == "/services/data/v67.0" || r.URL.Path == "/services/data/v67.0/":
		// The resources of the version, which an org answers only with a
		// valid access token.
		json.NewEncoder(w).Encode(map[string]string{"sobjects": "/services/data/v67.0/sobjects"})
	case r.URL.Path == "/services/data" || r.URL.Path == "/services/data/":
		json.NewEncoder(w).Encode([]map[string]string{
			{"label": "Earlier", "url": "/services/data/v66.0", "version": "66.0"},
			{"label": "Latest", "url": "/services/data/v67.0", "version": "67.0"},
		})
	case r.URL.Path == "/services/data/v67.0":
		json.NewEncoder(w).Encode(map[string]string{"sobjects": "/services/data/v67.0/sobjects"})
	case r.URL.Path == "/services/oauth2/userinfo":
		json.NewEncoder(w).Encode(map[string]string{
			"preferred_username": o.username,
			"organization_id":    "00D000000000000",
			"user_id":            "005000000000001AAA",
		})
	case strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/query"):
		soql := r.URL.Query().Get("q")
		if r.URL.Query().Get("columns") == "true" {
			for fragment, body := range o.columnMetadata {
				if strings.Contains(soql, fragment) {
					w.Write([]byte(body))
					return
				}
			}
		}
		for fragment, body := range o.rawQueries {
			if strings.Contains(soql, fragment) {
				w.Write([]byte(body))
				return
			}
		}
		for fragment, records := range o.queries {
			if strings.Contains(soql, fragment) {
				json.NewEncoder(w).Encode(map[string]interface{}{"done": true, "totalSize": len(records), "records": records})
				return
			}
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode([]map[string]string{{"errorCode": "INVALID_TYPE", "message": "sObject type is not supported: " + soql}})
	default:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode([]map[string]string{{"errorCode": "NOT_FOUND", "message": "The requested resource does not exist"}})
	}
}

// login authorizes the fake org through "org login access-token" with the
// given extra arguments.
func (o *fakeOrg) login(t *testing.T, args ...string) {
	t.Helper()
	t.Setenv("SF_ACCESS_TOKEN", fakeToken)
	full := append([]string{"org", "login", "access-token", "--instance-url", o.URL, "--no-prompt"}, args...)
	if out, errOut, status := run(full...); status != 0 {
		t.Fatalf("login failed: status %d, stdout %s, stderr %s", status, out, errOut)
	}
	t.Setenv("SF_ACCESS_TOKEN", "")
}
