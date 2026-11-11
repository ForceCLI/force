package webdebug

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ForceCLI/force/lib/apexlog"
)

const calcID = "01p5g00000ABCDE"

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	f, err := os.Open("../lib/apexlog/testdata/anonymous.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	log, err := apexlog.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	session, err := apexlog.NewSession(log)
	if err != nil {
		t.Fatal(err)
	}
	calc := make([]string, 16)
	for i := range calc {
		calc[i] = "// line " + string(rune('a'+i))
	}
	server, err := NewServer(session, map[string]Source{
		apexlog.AnonymousKey: {Name: "anonymous Apex", Lines: log.Header.AnonymousSource},
		calcID:               {Name: "Calc", Lines: calc},
	})
	if err != nil {
		t.Fatal(err)
	}
	http := httptest.NewServer(server.Handler())
	t.Cleanup(http.Close)
	return server, http
}

func call(t *testing.T, server *Server, base *httptest.Server, method, path string, body string, result any) int {
	t.Helper()
	req, err := http.NewRequest(method, base.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Debug-Token", server.Token())
	resp, err := base.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode
}

func TestAPIRequiresTheToken(t *testing.T) {
	_, base := newTestServer(t)
	for _, token := range []string{"", "wrong"} {
		req, _ := http.NewRequest("POST", base.URL+"/api/step?kind=in", nil)
		if token != "" {
			req.Header.Set("X-Debug-Token", token)
		}
		resp, err := base.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("token %q: status = %d", token, resp.StatusCode)
		}
	}
	resp, err := base.Client().Get(base.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Errorf("page: status=%d type=%q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}

func TestStateStepsAndOutput(t *testing.T) {
	server, base := newTestServer(t)
	var st stateJSON
	call(t, server, base, "GET", "/api/state", "", &st)
	if st.Status != "Paused" || len(st.Frames) != 1 || st.Frames[0].Source != apexlog.AnonymousKey || st.Frames[0].Line != 1 {
		t.Fatalf("state = %+v", st)
	}
	if len(st.Sources) != 2 || st.Sources[1].Name != "Calc" {
		t.Errorf("sources = %+v", st.Sources)
	}

	call(t, server, base, "POST", "/api/step?kind=in", "", &st)
	if len(st.Frames) != 2 || st.Frames[0].Name != "Calc.add(Integer, Integer)" || st.Frames[0].Line != 5 || st.Frames[1].Index != 0 {
		t.Fatalf("after step in: %+v", st.Frames)
	}
	call(t, server, base, "POST", "/api/step?kind=out", "", &st)
	call(t, server, base, "POST", "/api/step?kind=over", "", &st)
	if !st.Done || st.Status != "End of log" || len(st.Output) != 1 || st.Output[0].Message != "total: 5\nsecond line|with pipe" {
		t.Errorf("at end: %+v", st)
	}
	call(t, server, base, "POST", "/api/step?kind=back", "", &st)
	if st.Done || len(st.Output) != 0 || !strings.HasPrefix(st.Status, "Stepped back") {
		t.Errorf("after back: %+v", st)
	}
	if code := call(t, server, base, "POST", "/api/step?kind=sideways", "", nil); code != http.StatusBadRequest {
		t.Errorf("unknown step: status = %d", code)
	}
}

func TestBreakpointsAndSource(t *testing.T) {
	server, base := newTestServer(t)
	var src sourceJSON
	call(t, server, base, "GET", "/api/source?key="+calcID, "", &src)
	if src.Name != "Calc" || len(src.Lines) != 16 || len(src.Executed) == 0 {
		t.Fatalf("source = %+v", src)
	}
	executed := map[int]bool{}
	for _, n := range src.Executed {
		executed[n] = true
	}
	if !executed[6] || executed[1] {
		t.Errorf("executed = %v", src.Executed)
	}

	var st stateJSON
	call(t, server, base, "POST", "/api/breakpoint", `{"source":"`+calcID+`","line":6}`, &st)
	if len(st.Breakpoints) != 1 || st.Status != "Breakpoint set at Calc line 6" {
		t.Fatalf("state = %+v", st)
	}
	call(t, server, base, "POST", "/api/step?kind=continue", "", &st)
	if st.Status != "Breakpoint at Calc line 6" {
		t.Errorf("status = %q", st.Status)
	}
	call(t, server, base, "POST", "/api/breakpoint", `{"source":"`+calcID+`","line":1}`, &st)
	if !strings.Contains(st.Status, "will not stop") {
		t.Errorf("status = %q", st.Status)
	}
	call(t, server, base, "POST", "/api/breakpoint", `{"source":"`+calcID+`","line":6}`, &st)
	if len(st.Breakpoints) != 1 || !strings.HasPrefix(st.Status, "Breakpoint removed") {
		t.Errorf("state = %+v", st)
	}

	call(t, server, base, "GET", "/api/source?key=missing", "", &src)
	if src.Missing == "" || len(src.Lines) != 0 {
		t.Errorf("missing source = %+v", src)
	}
}

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestVariablesExpandByPath(t *testing.T) {
	server, base := newTestServer(t)
	var st stateJSON
	call(t, server, base, "POST", "/api/breakpoint", `{"source":"`+calcID+`","line":8}`, &st)
	call(t, server, base, "POST", "/api/step?kind=continue", "", &st)
	frame := st.Frames[0].Index

	var vars variablesJSON
	call(t, server, base, "GET", "/api/variables?frame="+jsonString(frame), "", &vars)
	if len(vars.Groups) != 2 || vars.Groups[0].Title != "Locals" || vars.Groups[1].Title != "Statics (Calc)" {
		t.Fatalf("groups = %+v", vars.Groups)
	}
	var names variableJSON
	for _, v := range vars.Groups[0].Variables {
		if v.Name == "names" {
			names = v
		}
	}
	if !names.Expandable || names.Note != "as of line 7" || jsonString(names.Path) != `["local","names"]` {
		t.Fatalf("names = %+v", names)
	}
	var children variablesJSON
	path := url.QueryEscape(jsonString(names.Path))
	call(t, server, base, "GET", "/api/variables?frame="+jsonString(frame)+"&path="+path, "", &children)
	if len(children.Variables) != 2 || children.Variables[0].Value != "'x|y'" || jsonString(children.Variables[0].Path) != `["local","names","[0]"]` {
		t.Errorf("children = %+v", children.Variables)
	}
	if code := call(t, server, base, "GET", "/api/variables?frame=0&path="+url.QueryEscape(`["local","nope"]`), "", nil); code != http.StatusBadRequest {
		t.Errorf("unknown variable: status = %d", code)
	}
}

func TestRunStopsOnQuit(t *testing.T) {
	server, _ := newTestServer(t)
	urls := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- server.Run(context.Background(), func(u string) { urls <- u })
	}()
	u := <-urls
	if !strings.HasPrefix(u, "http://127.0.0.1:") || !strings.HasSuffix(u, "/?token="+server.Token()) {
		t.Errorf("url = %q", u)
	}
	req, _ := http.NewRequest("POST", strings.TrimSuffix(u, "/?token="+server.Token())+"/api/quit", nil)
	req.Header.Set("X-Debug-Token", server.Token())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after quit")
	}
}
