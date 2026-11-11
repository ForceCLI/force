package lib

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type toolingRequest struct {
	Method string
	Path   string
	Query  string
	Body   map[string]interface{}
}

// fakeTooling answers Tooling API requests for DebugLevel and TraceFlag
// records and records each request.
type fakeTooling struct {
	debugLevels string
	traceFlags  string
	patchStatus func(n int) int
	patches     int
	requests    []toolingRequest
}

func (f *fakeTooling) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	req := toolingRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query().Get("q")}
	json.Unmarshal(body, &req.Body)
	f.requests = append(f.requests, req)
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.Contains(req.Query, "FROM DebugLevel"):
		fmt.Fprintf(w, `{"totalSize":1,"done":true,"records":[%s]}`, f.debugLevels)
	case strings.Contains(req.Query, "FROM TraceFlag"):
		fmt.Fprintf(w, `{"totalSize":1,"done":true,"records":[%s]}`, f.traceFlags)
	case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/sobjects/DebugLevel"):
		w.WriteHeader(201)
		fmt.Fprint(w, `{"id":"7dl000000000NEW","success":true,"errors":[]}`)
	case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/sobjects/TraceFlag"):
		w.WriteHeader(201)
		fmt.Fprint(w, `{"id":"7tf000000000NEW","success":true,"errors":[]}`)
	case r.Method == "PATCH":
		f.patches++
		status := 204
		if f.patchStatus != nil {
			status = f.patchStatus(f.patches)
		}
		w.WriteHeader(status)
		if status != 204 {
			fmt.Fprint(w, `[{"message":"Expiration date cannot be in the past","errorCode":"FIELD_INTEGRITY_EXCEPTION"}]`)
		}
	case r.Method == "DELETE":
		w.WriteHeader(204)
	default:
		w.WriteHeader(404)
		fmt.Fprint(w, `[{"message":"not found","errorCode":"NOT_FOUND"}]`)
	}
}

func (f *fakeTooling) only(method string) []toolingRequest {
	var result []toolingRequest
	for _, r := range f.requests {
		if r.Method == method {
			result = append(result, r)
		}
	}
	return result
}

func newToolingForce(t *testing.T, fake *fakeTooling) *Force {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(server.Close)
	return NewForce(&ForceSession{
		AccessToken:    "token",
		InstanceUrl:    server.URL,
		UserInfo:       &UserInfo{OrgId: "00D000000000001", UserId: "005000000000001"},
		SessionOptions: &SessionOptions{ApiVersion: "62.0"},
	})
}

func TestReplayDebugLevelCreatesFinestDebugLevel(t *testing.T) {
	fake := &fakeTooling{}
	force := newToolingForce(t, fake)
	id, err := force.ReplayDebugLevel()
	if err != nil {
		t.Fatal(err)
	}
	if id != "7dl000000000NEW" {
		t.Errorf("id = %q", id)
	}
	posts := fake.only("POST")
	if len(posts) != 1 || posts[0].Body["ApexCode"] != "Finest" || posts[0].Body["DeveloperName"] != ReplayDebugLevelName {
		t.Errorf("posts = %+v", posts)
	}
}

func TestReplayDebugLevelUsesExistingDebugLevel(t *testing.T) {
	fake := &fakeTooling{debugLevels: `{"Id":"7dl000000000OLD"}`}
	force := newToolingForce(t, fake)
	id, err := force.ReplayDebugLevel()
	if err != nil {
		t.Fatal(err)
	}
	if id != "7dl000000000OLD" || len(fake.only("POST")) != 0 {
		t.Errorf("id = %q, requests = %+v", id, fake.requests)
	}
}

func TestEnableReplayTraceFlagCreatesAndDeletesTraceFlag(t *testing.T) {
	fake := &fakeTooling{debugLevels: `{"Id":"7dl000000000OLD"}`}
	force := newToolingForce(t, fake)
	restore, err := force.EnableReplayTraceFlag()
	if err != nil {
		t.Fatal(err)
	}
	posts := fake.only("POST")
	if len(posts) != 1 || posts[0].Body["DebugLevelId"] != "7dl000000000OLD" || posts[0].Body["TracedEntityId"] != "005000000000001" || posts[0].Body["LogType"] != "DEVELOPER_LOG" {
		t.Fatalf("posts = %+v", posts)
	}
	if err := restore(); err != nil {
		t.Fatal(err)
	}
	deletes := fake.only("DELETE")
	if len(deletes) != 1 || !strings.HasSuffix(deletes[0].Path, "/TraceFlag/7tf000000000NEW") {
		t.Errorf("deletes = %+v", deletes)
	}
}

func TestEnableReplayTraceFlagChangesAndRestoresExistingTraceFlag(t *testing.T) {
	expiration := time.Now().UTC().Add(2 * time.Hour).Format(traceFlagTimeFormat)
	fake := &fakeTooling{
		debugLevels: `{"Id":"7dl000000000RPL"}`,
		traceFlags:  `{"Id":"7tf000000000OLD","DebugLevelId":"7dl000000000USR","StartDate":null,"ExpirationDate":"` + expiration + `"}`,
	}
	force := newToolingForce(t, fake)
	restore, err := force.EnableReplayTraceFlag()
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.only("POST")) != 0 {
		t.Errorf("created a record: %+v", fake.only("POST"))
	}
	patches := fake.only("PATCH")
	if len(patches) != 1 || patches[0].Body["DebugLevelId"] != "7dl000000000RPL" {
		t.Fatalf("patches = %+v", patches)
	}
	if _, ok := patches[0].Body["StartDate"]; ok {
		t.Errorf("StartDate set on a trace flag without one: %+v", patches[0].Body)
	}
	if err := restore(); err != nil {
		t.Fatal(err)
	}
	patches = fake.only("PATCH")
	if len(patches) != 2 || patches[1].Body["DebugLevelId"] != "7dl000000000USR" || patches[1].Body["ExpirationDate"] != expiration {
		t.Errorf("restore patch = %+v", patches)
	}
}

func TestEnableReplayTraceFlagExpiresAnExpiredTraceFlagAgain(t *testing.T) {
	fake := &fakeTooling{
		debugLevels: `{"Id":"7dl000000000RPL"}`,
		traceFlags:  `{"Id":"7tf000000000OLD","DebugLevelId":"7dl000000000USR","StartDate":null,"ExpirationDate":"2020-01-01T00:00:00.000+0000"}`,
		patchStatus: func(n int) int {
			if n == 2 {
				return 400
			}
			return 204
		},
	}
	force := newToolingForce(t, fake)
	restore, err := force.EnableReplayTraceFlag()
	if err != nil {
		t.Fatal(err)
	}
	if err := restore(); err != nil {
		t.Fatal(err)
	}
	patches := fake.only("PATCH")
	if len(patches) != 3 {
		t.Fatalf("patches = %+v", patches)
	}
	last := patches[2].Body
	expires, err := time.Parse(traceFlagTimeFormat, last["ExpirationDate"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if last["DebugLevelId"] != "7dl000000000USR" || time.Until(expires) > 2*time.Minute || time.Until(expires) < 0 {
		t.Errorf("last patch = %+v", last)
	}
}
