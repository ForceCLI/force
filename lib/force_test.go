package lib

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ForceCLI/force/lib/query"
)

// TestCancelableQueryAndSend_Cancellation ensures that CancelableQueryAndSend stops sending after context cancellation
func TestCancelableQueryAndSend_Cancellation(t *testing.T) {
	// Stub forceQuery to emit two pages with a delay
	orig := forceQuery
	defer func() { forceQuery = orig }()
	forceQuery = func(cb query.PageCallback, opts ...query.Option) error {
		// first page
		r1 := query.Record{Fields: map[string]interface{}{"v": 1}}
		if !cb([]query.Record{r1}) {
			return nil
		}
		// delay before second page
		time.Sleep(50 * time.Millisecond)
		r2 := query.Record{Fields: map[string]interface{}{"v": 2}}
		cb([]query.Record{r2})
		return nil
	}
	f := &Force{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan ForceRecord, 1)
	// start sending
	var err error
	go func() {
		// cancel shortly after first page is sent
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	err = f.CancelableQueryAndSend(ctx, "", ch)
	if err == nil {
		t.Fatalf("expected error on cancellation, got nil")
	}
	// collect results
	var recs []ForceRecord
	for r := range ch {
		recs = append(recs, r)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record before cancel, got %d", len(recs))
	}
	if recs[0]["v"] != 1 {
		t.Fatalf("expected first record v=1, got %v", recs[0]["v"])
	}
}

// TestAbortableQueryAndSend_Abort ensures that AbortableQueryAndSend stops sending after abort signal
func TestAbortableQueryAndSend_Abort(t *testing.T) {
	orig := forceQuery
	defer func() { forceQuery = orig }()
	forceQuery = func(cb query.PageCallback, opts ...query.Option) error {
		// first page
		r1 := query.Record{Fields: map[string]interface{}{"v": 1}}
		if !cb([]query.Record{r1}) {
			return nil
		}
		// second page
		r2 := query.Record{Fields: map[string]interface{}{"v": 2}}
		cb([]query.Record{r2})
		return nil
	}
	f := &Force{}
	abortCh := make(chan bool)
	ch := make(chan ForceRecord, 1)
	// trigger abort after first record
	go func() {
		time.Sleep(10 * time.Millisecond)
		close(abortCh)
	}()
	err := f.AbortableQueryAndSend("", ch, abortCh)
	if err != nil {
		t.Fatalf("expected no error on abort, got %v", err)
	}
	var recs []ForceRecord
	for r := range ch {
		recs = append(recs, r)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record before abort, got %d", len(recs))
	}
	if recs[0]["v"] != 1 {
		t.Fatalf("expected first record v=1, got %v", recs[0]["v"])
	}
}

func TestUpsertRecord_Create(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Expected PATCH request, got %s", r.Method)
		}

		expectedPath := "/services/data/" + ApiVersion() + "/sobjects/Account/External_Id__c/ABC123"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "001xx000003DGbYAAW",
			"success": true,
			"created": true,
		})
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	result, err := force.UpsertRecord("Account", "External_Id__c", "ABC123", map[string]string{
		"Name": "Test Account",
	})

	if err != nil {
		t.Fatalf("UpsertRecord returned error: %v", err)
	}

	if !result.Created {
		t.Error("Expected Created to be true for new record")
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}

	if result.Id != "001xx000003DGbYAAW" {
		t.Errorf("Expected Id 001xx000003DGbYAAW, got %s", result.Id)
	}
}

func TestUpsertRecord_Update(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Expected PATCH request, got %s", r.Method)
		}

		expectedPath := "/services/data/" + ApiVersion() + "/sobjects/Contact/Email/test@example.com"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	result, err := force.UpsertRecord("Contact", "Email", "test@example.com", map[string]string{
		"FirstName": "John",
		"LastName":  "Doe",
	})

	if err != nil {
		t.Fatalf("UpsertRecord returned error: %v", err)
	}

	if result.Created {
		t.Error("Expected Created to be false for updated record")
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}
}

func TestUpsertRecord_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"message":   "Required fields are missing: [Name]",
				"errorCode": "REQUIRED_FIELD_MISSING",
			},
		})
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	_, err := force.UpsertRecord("Account", "External_Id__c", "ABC123", map[string]string{})

	if err == nil {
		t.Fatal("Expected error for bad request, got nil")
	}

	expectedMsg := "Required fields are missing: [Name]"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestCreateRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		expectedPath := "/services/data/" + ApiVersion() + "/sobjects/Account"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "001xx000003DGbZAAW",
			"success": true,
		})
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	id, err, _ := force.CreateRecord("Account", map[string]string{
		"Name": "New Account",
	})

	if err != nil {
		t.Fatalf("CreateRecord returned error: %v", err)
	}

	if id != "001xx000003DGbZAAW" {
		t.Errorf("Expected Id 001xx000003DGbZAAW, got %s", id)
	}
}

func TestUpdateRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Expected PATCH request, got %s", r.Method)
		}

		expectedPath := "/services/data/" + ApiVersion() + "/sobjects/Account/001xx000003DGbYAAW"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	err := force.UpdateRecord("Account", "001xx000003DGbYAAW", map[string]string{
		"Name": "Updated Account",
	})

	if err != nil {
		t.Fatalf("UpdateRecord returned error: %v", err)
	}
}

func TestDeleteRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE request, got %s", r.Method)
		}

		expectedPath := "/services/data/" + ApiVersion() + "/sobjects/Account/001xx000003DGbYAAW"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	err := force.DeleteRecord("Account", "001xx000003DGbYAAW")

	if err != nil {
		t.Fatalf("DeleteRecord returned error: %v", err)
	}
}

func TestGetRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		expectedPath := "/services/data/" + ApiVersion() + "/sobjects/Account/001xx000003DGbYAAW"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Id":   "001xx000003DGbYAAW",
			"Name": "Test Account",
			"attributes": map[string]interface{}{
				"type": "Account",
				"url":  "/services/data/v62.0/sobjects/Account/001xx000003DGbYAAW",
			},
		})
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	record, err := force.GetRecord("Account", "001xx000003DGbYAAW")

	if err != nil {
		t.Fatalf("GetRecord returned error: %v", err)
	}

	if record["Id"] != "001xx000003DGbYAAW" {
		t.Errorf("Expected Id 001xx000003DGbYAAW, got %v", record["Id"])
	}

	if record["Name"] != "Test Account" {
		t.Errorf("Expected Name 'Test Account', got %v", record["Name"])
	}
}

func TestCreateRecord_handles_empty_error_response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	_, err, _ := force.CreateRecord("Account", map[string]string{
		"Name": "Test Account",
	})

	if err == nil {
		t.Fatal("Expected error for server error response, got nil")
	}

	if err.Error() != "request failed with status 500: Internal Server Error" {
		t.Errorf("Expected error message with status and body, got %q", err.Error())
	}
}

func TestUpdateRecord_handles_empty_error_response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("Bad Gateway"))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	err := force.UpdateRecord("Account", "001xx000003DGbYAAW", map[string]string{
		"Name": "Updated Account",
	})

	if err == nil {
		t.Fatal("Expected error for server error response, got nil")
	}

	if err.Error() != "request failed with status 502: Bad Gateway" {
		t.Errorf("Expected error message with status and body, got %q", err.Error())
	}
}

func TestDeleteRecord_handles_empty_error_response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service Unavailable"))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	err := force.DeleteRecord("Account", "001xx000003DGbYAAW")

	if err == nil {
		t.Fatal("Expected error for server error response, got nil")
	}

	if err.Error() != "request failed with status 503: Service Unavailable" {
		t.Errorf("Expected error message with status and body, got %q", err.Error())
	}
}

func TestCreateRecord_returns_ScratchOrgExpiredError_on_420(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(420)
		w.Write([]byte("<html>Scratch org expired</html>"))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	_, err, _ := force.CreateRecord("Account", map[string]string{
		"Name": "Test Account",
	})

	if err != ScratchOrgExpiredError {
		t.Errorf("Expected ScratchOrgExpiredError, got %v", err)
	}
}

func TestUpdateRecord_returns_ScratchOrgExpiredError_on_420(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(420)
		w.Write([]byte("<html>Scratch org expired</html>"))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	err := force.UpdateRecord("Account", "001xx000003DGbYAAW", map[string]string{
		"Name": "Updated Account",
	})

	if err != ScratchOrgExpiredError {
		t.Errorf("Expected ScratchOrgExpiredError, got %v", err)
	}
}

func TestDeleteRecord_returns_ScratchOrgExpiredError_on_420(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(420)
		w.Write([]byte("<html>Scratch org expired</html>"))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	err := force.DeleteRecord("Account", "001xx000003DGbYAAW")

	if err != ScratchOrgExpiredError {
		t.Errorf("Expected ScratchOrgExpiredError, got %v", err)
	}
}

func TestGetRecord_returns_ScratchOrgExpiredError_on_420(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(420)
		w.Write([]byte("<html>Scratch org expired</html>"))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	_, err := force.GetRecord("Account", "001xx000003DGbYAAW")

	if err != ScratchOrgExpiredError {
		t.Errorf("Expected ScratchOrgExpiredError, got %v", err)
	}
}
