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
	debugLevels      string
	traceFlags       string
	patchStatus      func(n int) int
	patches          int
	overlayFailAfter int
	overlays         int
	requests         []toolingRequest
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
	case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/sobjects/ApexExecutionOverlayAction"):
		f.overlays++
		if f.overlayFailAfter > 0 && f.overlays > f.overlayFailAfter {
			w.WriteHeader(400)
			fmt.Fprint(w, `[{"message":"Maximum number of checkpoints reached","errorCode":"FIELD_INTEGRITY_EXCEPTION"}]`)
			return
		}
		w.WriteHeader(201)
		fmt.Fprintf(w, `{"id":"1do00000000000%d","success":true,"errors":[]}`, f.overlays)
	case r.Method == "GET" && strings.Contains(r.URL.Path, "/sobjects/ApexExecutionOverlayResult/07n000000000001"):
		fmt.Fprint(w, `{"Id":"07n000000000001AAA","HeapDump":{"className":"Calc","namespace":"none","extents":[]}}`)
	case r.Method == "GET" && strings.Contains(r.URL.Path, "/sobjects/ApexExecutionOverlayResult/"):
		fmt.Fprint(w, `{"Id":"07n000000000002AAA","HeapDump":null}`)
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

func TestStartReplayTraceUsesTheReplayDebugLevel(t *testing.T) {
	fake := &fakeTooling{debugLevels: `{"Id":"7dl000000000RPL"}`}
	force := newToolingForce(t, fake)
	result, err, _ := force.StartReplayTrace()
	if err != nil {
		t.Fatal(err)
	}
	if result.Id != "7tf000000000NEW" {
		t.Errorf("result = %+v", result)
	}
	posts := fake.only("POST")
	if len(posts) != 1 || posts[0].Body["DebugLevelId"] != "7dl000000000RPL" || posts[0].Body["TracedEntityId"] != "005000000000001" || posts[0].Body["LogType"] != "DEVELOPER_LOG" {
		t.Errorf("posts = %+v", posts)
	}
}

func TestCreateCheckpointsCreatesAndDeletesOverlayActions(t *testing.T) {
	fake := &fakeTooling{}
	force := newToolingForce(t, fake)
	remove, err := force.CreateCheckpoints([]Checkpoint{{Name: "Calc", Line: 9}, {Name: "CalcTest", Line: 12}})
	if err != nil {
		t.Fatal(err)
	}
	posts := fake.only("POST")
	if len(posts) != 2 || posts[0].Body["ExecutableEntityName"] != "Calc" || posts[0].Body["Line"] != float64(9) ||
		posts[0].Body["IsDumpingHeap"] != true || posts[0].Body["Iteration"] != float64(1) || posts[0].Body["ActionScriptType"] != "None" {
		t.Fatalf("posts = %+v", posts)
	}
	if err := remove(); err != nil {
		t.Fatal(err)
	}
	deletes := fake.only("DELETE")
	if len(deletes) != 2 || !strings.HasSuffix(deletes[0].Path, "/ApexExecutionOverlayAction/1do000000000001") {
		t.Errorf("deletes = %+v", deletes)
	}
}

func TestCreateCheckpointsDeletesCreatedOnesWhenOneFails(t *testing.T) {
	fake := &fakeTooling{overlayFailAfter: 1}
	force := newToolingForce(t, fake)
	_, err := force.CreateCheckpoints([]Checkpoint{{Name: "Calc", Line: 9}, {Name: "CalcTest", Line: 12}})
	if err == nil || !strings.Contains(err.Error(), "CalcTest:12") {
		t.Fatalf("err = %v", err)
	}
	deletes := fake.only("DELETE")
	if len(deletes) != 1 || !strings.HasSuffix(deletes[0].Path, "/ApexExecutionOverlayAction/1do000000000001") {
		t.Errorf("deletes = %+v", deletes)
	}
}

func TestHeapDumpReturnsTheHeapDumpField(t *testing.T) {
	fake := &fakeTooling{}
	force := newToolingForce(t, fake)
	raw, err := force.HeapDump("07n000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"className":"Calc"`) {
		t.Errorf("raw = %s", raw)
	}
	if !strings.Contains(fake.requests[0].Path, "ApexExecutionOverlayResult/07n000000000001") {
		t.Errorf("request = %+v", fake.requests[0])
	}
	if _, err := force.HeapDump("07n000000000002"); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("err for empty heap dump = %v", err)
	}
}
