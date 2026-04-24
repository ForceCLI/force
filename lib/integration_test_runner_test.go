package lib

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseRunTestsAsyncResponse(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "documented_object_form",
			body: `{"root":"707xx0000000bnbh"}`,
			want: "707xx0000000bnbh",
		},
		{
			name: "bare_quoted_string_form",
			body: `"707aer000000001AAA"`,
			want: "707aer000000001AAA",
		},
		{
			name: "object_with_whitespace",
			body: "  {\"root\": \"707abc\"}  ",
			want: "707abc",
		},
		{
			name:    "empty",
			body:    "",
			wantErr: true,
		},
		{
			name:    "malformed_object",
			body:    `{"root": `,
			wantErr: true,
		},
		{
			name:    "malformed_string",
			body:    `not json`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRunTestsAsyncResponse(tc.body)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEscapeSoqlLiteral(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"plain", "plain"},
		{"O'Brien", `O\'Brien`},
		{`'leading and trailing'`, `\'leading and trailing\'`},
	}
	for _, tc := range cases {
		if got := escapeSoqlLiteral(tc.in); got != tc.want {
			t.Errorf("escapeSoqlLiteral(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRunIntegrationTest_EndToEnd verifies that RunIntegrationTest:
//   - POSTs the documented "specific tests" request body
//   - polls ApexTestRunResult until Completed
//   - fetches ApexTestResult rows and returns them
func TestRunIntegrationTest_EndToEnd(t *testing.T) {
	var postBody []byte
	var postCount int32
	var pollCount int32

	handler := http.NewServeMux()

	handler.HandleFunc("/services/data/v55.0/tooling/runTestsAsynchronous", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		postBody = body
		atomic.AddInt32(&postCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"root":"707aer000000001AAA"}`))
	})

	handler.HandleFunc("/services/data/v55.0/tooling/query", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch {
		case strings.Contains(q, "FROM ApexTestRunResult"):
			n := atomic.AddInt32(&pollCount, 1)
			if n < 2 {
				// First poll: still queued
				_, _ = w.Write([]byte(`{"records":[{"Id":"05maer000000001AAA","AsyncApexJobId":"707aer000000001AAA","Status":"Queued","MethodsEnqueued":1}]}`))
				return
			}
			// Second poll: completed
			_, _ = w.Write([]byte(`{"records":[{
				"Id":"05maer000000001AAA",
				"AsyncApexJobId":"707aer000000001AAA",
				"Status":"Completed",
				"ClassesEnqueued":1,
				"ClassesCompleted":1,
				"MethodsEnqueued":1,
				"MethodsCompleted":1,
				"MethodsFailed":0,
				"TestTime":1234
			}]}`))
		case strings.Contains(q, "FROM ApexTestResult"):
			_, _ = w.Write([]byte(`{"records":[{
				"ApexClass":{"Name":"SampleIntegrationTest"},
				"ApexClassId":"01paer000000001AAA",
				"MethodName":"happy_path",
				"Outcome":"Pass",
				"Message":null,
				"StackTrace":null,
				"RunTime":42
			}]}`))
		default:
			t.Fatalf("unexpected query: %s", q)
		}
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()

	f := &Force{Credentials: &ForceSession{InstanceUrl: ts.URL, AccessToken: "token"}}
	result, err := f.RunIntegrationTest("SampleIntegrationTest", 10*time.Millisecond, 5*time.Second)
	if err != nil {
		t.Fatalf("RunIntegrationTest failed: %v", err)
	}

	// Assert request body matches the documented "specific tests" form.
	var parsed IntegrationTestRequest
	if err := json.Unmarshal(postBody, &parsed); err != nil {
		t.Fatalf("failed to parse request body %s: %v", postBody, err)
	}
	if len(parsed.Tests) != 1 {
		t.Fatalf("expected 1 test node, got %d (body=%s)", len(parsed.Tests), postBody)
	}
	if parsed.Tests[0].ClassName != "SampleIntegrationTest" {
		t.Errorf("expected className SampleIntegrationTest, got %q", parsed.Tests[0].ClassName)
	}
	if len(parsed.Tests[0].TestMethods) != 0 {
		t.Errorf("expected no testMethods for class-only invocation, got %v", parsed.Tests[0].TestMethods)
	}
	// The unsupported "maxFailedTests: -1" field must not appear in the body.
	if strings.Contains(string(postBody), "maxFailedTests") {
		t.Errorf("request body should not include maxFailedTests: %s", postBody)
	}

	if atomic.LoadInt32(&postCount) != 1 {
		t.Errorf("expected one POST, got %d", postCount)
	}
	if got := atomic.LoadInt32(&pollCount); got < 2 {
		t.Errorf("expected at least 2 polls, got %d", got)
	}

	if result.Status != "Completed" {
		t.Errorf("expected status Completed, got %q", result.Status)
	}
	if result.AsyncApexJobID != "707aer000000001AAA" {
		t.Errorf("expected AsyncApexJobID, got %q", result.AsyncApexJobID)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 method result, got %d", len(result.Results))
	}
	m := result.Results[0]
	if m.ClassName != "SampleIntegrationTest" || m.MethodName != "happy_path" || m.Outcome != "Pass" || m.RunTime != 42 {
		t.Errorf("unexpected method result: %+v", m)
	}
}

// TestRunIntegrationTest_ClassDotMethod verifies that class.method input is
// delivered as a testMethods entry rather than a bare class name.
func TestRunIntegrationTest_ClassDotMethod(t *testing.T) {
	var postBody []byte
	handler := http.NewServeMux()
	handler.HandleFunc("/services/data/v55.0/tooling/runTestsAsynchronous", func(w http.ResponseWriter, r *http.Request) {
		postBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"root":"707aer000000001AAA"}`))
	})
	handler.HandleFunc("/services/data/v55.0/tooling/query", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(q, "FROM ApexTestRunResult") {
			_, _ = w.Write([]byte(`{"records":[{"AsyncApexJobId":"707aer000000001AAA","Status":"Completed","MethodsEnqueued":1,"MethodsCompleted":1}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"records":[]}`))
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()

	f := &Force{Credentials: &ForceSession{InstanceUrl: ts.URL, AccessToken: "token"}}
	if _, err := f.RunIntegrationTest("MyTest.happy_path", 10*time.Millisecond, 5*time.Second); err != nil {
		t.Fatalf("RunIntegrationTest failed: %v", err)
	}
	var parsed IntegrationTestRequest
	if err := json.Unmarshal(postBody, &parsed); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if len(parsed.Tests) != 1 {
		t.Fatalf("expected 1 test node, got %d", len(parsed.Tests))
	}
	if parsed.Tests[0].ClassName != "MyTest" {
		t.Errorf("expected className MyTest, got %q", parsed.Tests[0].ClassName)
	}
	if want := []string{"happy_path"}; len(parsed.Tests[0].TestMethods) != 1 || parsed.Tests[0].TestMethods[0] != want[0] {
		t.Errorf("expected testMethods %v, got %v", want, parsed.Tests[0].TestMethods)
	}
}

func TestRunIntegrationTest_Timeout(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/services/data/v55.0/tooling/runTestsAsynchronous", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"root":"707aer000000001AAA"}`))
	})
	handler.HandleFunc("/services/data/v55.0/tooling/query", func(w http.ResponseWriter, r *http.Request) {
		// Always return a Queued status so the poll loop never completes.
		_, _ = w.Write([]byte(`{"records":[{"AsyncApexJobId":"707aer000000001AAA","Status":"Queued"}]}`))
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	f := &Force{Credentials: &ForceSession{InstanceUrl: ts.URL, AccessToken: "token"}}
	_, err := f.RunIntegrationTest("Sample", 5*time.Millisecond, 25*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error message, got: %v", err)
	}
}

func TestRunIntegrationTest_EmptyClass(t *testing.T) {
	f := &Force{Credentials: &ForceSession{}}
	_, err := f.RunIntegrationTest("   ", time.Second, time.Second)
	if err == nil {
		t.Fatal("expected error for empty class, got nil")
	}
}

// Sanity check that the tooling URL includes the current API version prefix.
func TestRunIntegrationTest_URLIncludesApiVersion(t *testing.T) {
	var gotPath string
	handler := http.NewServeMux()
	handler.HandleFunc("/services/data/v55.0/tooling/runTestsAsynchronous", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"root":"707aer000000001AAA"}`))
	})
	handler.HandleFunc("/services/data/v55.0/tooling/query", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"records":[{"AsyncApexJobId":"707aer000000001AAA","Status":"Completed"}]}`))
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()
	f := &Force{Credentials: &ForceSession{InstanceUrl: ts.URL, AccessToken: "token"}}
	if _, err := f.RunIntegrationTest("Sample", 10*time.Millisecond, 2*time.Second); err != nil {
		t.Fatalf("RunIntegrationTest failed: %v", err)
	}
	wantSuffix := "/services/data/v55.0/tooling/runTestsAsynchronous"
	if !strings.HasSuffix(gotPath, wantSuffix) {
		t.Errorf("expected path ending in %q, got %q", wantSuffix, gotPath)
	}
	// The URL must NOT be doubled (e.g., https://host/http://host/...)
	if u, err := url.Parse(ts.URL + gotPath); err == nil {
		if strings.Count(u.Path, "http://") > 0 || strings.Count(u.Path, "https://") > 0 {
			t.Errorf("URL path contains nested scheme: %s", gotPath)
		}
	}
}
