package lib

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Job State Tests
// =============================================================================

func TestBulk2IngestJobInfo_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		state    Bulk2JobState
		expected bool
	}{
		{"Open is not terminal", Bulk2JobStateOpen, false},
		{"UploadComplete is not terminal", Bulk2JobStateUploadComplete, false},
		{"InProgress is not terminal", Bulk2JobStateInProgress, false},
		{"JobComplete is terminal", Bulk2JobStateJobComplete, true},
		{"Failed is terminal", Bulk2JobStateFailed, true},
		{"Aborted is terminal", Bulk2JobStateAborted, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Bulk2IngestJobInfo{State: tt.state}
			if got := job.IsTerminal(); got != tt.expected {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBulk2QueryJobInfo_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		state    Bulk2JobState
		expected bool
	}{
		{"Open is not terminal", Bulk2JobStateOpen, false},
		{"InProgress is not terminal", Bulk2JobStateInProgress, false},
		{"JobComplete is terminal", Bulk2JobStateJobComplete, true},
		{"Failed is terminal", Bulk2JobStateFailed, true},
		{"Aborted is terminal", Bulk2JobStateAborted, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Bulk2QueryJobInfo{State: tt.state}
			if got := job.IsTerminal(); got != tt.expected {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// =============================================================================
// Display Function Tests
// =============================================================================

func TestDisplayBulk2IngestJobInfo(t *testing.T) {
	jobInfo := Bulk2IngestJobInfo{
		Id:                      "7501234567890ABCDEF",
		State:                   Bulk2JobStateJobComplete,
		Operation:               Bulk2OperationInsert,
		Object:                  "Account",
		ApiVersion:              55.0,
		CreatedById:             "0051234567890ABCDEF",
		CreatedDate:             "2024-01-15T10:30:00.000Z",
		SystemModstamp:          "2024-01-15T10:35:00.000Z",
		ContentType:             "CSV",
		ConcurrencyMode:         "Parallel",
		NumberRecordsProcessed:  100,
		NumberRecordsFailed:     5,
		Retries:                 0,
		TotalProcessingTime:     5000,
		ApiActiveProcessingTime: 4500,
		ApexProcessingTime:      500,
	}

	var buf bytes.Buffer
	DisplayBulk2IngestJobInfo(jobInfo, &buf)
	output := buf.String()

	expectedParts := []string{
		"7501234567890ABCDEF",
		"JobComplete",
		"insert",
		"Account",
		"55.0",
		"100",
		"5",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("Output should contain %q, got:\n%s", part, output)
		}
	}
}

func TestDisplayBulk2IngestJobInfo_WithErrorMessage(t *testing.T) {
	jobInfo := Bulk2IngestJobInfo{
		Id:           "7501234567890ABCDEF",
		State:        Bulk2JobStateFailed,
		Operation:    Bulk2OperationInsert,
		Object:       "Account",
		ErrorMessage: "InvalidBatch: Field Name not found",
	}

	var buf bytes.Buffer
	DisplayBulk2IngestJobInfo(jobInfo, &buf)
	output := buf.String()

	if !strings.Contains(output, "InvalidBatch: Field Name not found") {
		t.Errorf("Output should contain error message, got:\n%s", output)
	}
}

func TestDisplayBulk2QueryJobInfo(t *testing.T) {
	jobInfo := Bulk2QueryJobInfo{
		Id:                     "7501234567890ABCDEF",
		State:                  Bulk2JobStateJobComplete,
		Operation:              Bulk2OperationQuery,
		Object:                 "Contact",
		ApiVersion:             55.0,
		CreatedById:            "0051234567890ABCDEF",
		CreatedDate:            "2024-01-15T10:30:00.000Z",
		SystemModstamp:         "2024-01-15T10:35:00.000Z",
		ContentType:            "CSV",
		ConcurrencyMode:        "Parallel",
		NumberRecordsProcessed: 500,
		Retries:                1,
		TotalProcessingTime:    10000,
	}

	var buf bytes.Buffer
	DisplayBulk2QueryJobInfo(jobInfo, &buf)
	output := buf.String()

	expectedParts := []string{
		"7501234567890ABCDEF",
		"JobComplete",
		"query",
		"Contact",
		"55.0",
		"500",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("Output should contain %q, got:\n%s", part, output)
		}
	}
}

func TestDisplayBulk2QueryJobInfo_WithErrorMessage(t *testing.T) {
	jobInfo := Bulk2QueryJobInfo{
		Id:           "7501234567890ABCDEF",
		State:        Bulk2JobStateFailed,
		Operation:    Bulk2OperationQuery,
		Object:       "Account",
		ErrorMessage: "MALFORMED_QUERY: unexpected token",
	}

	var buf bytes.Buffer
	DisplayBulk2QueryJobInfo(jobInfo, &buf)
	output := buf.String()

	if !strings.Contains(output, "MALFORMED_QUERY: unexpected token") {
		t.Errorf("Output should contain error message, got:\n%s", output)
	}
}

func TestDisplayBulk2IngestJobList(t *testing.T) {
	jobs := []Bulk2IngestJobInfo{
		{
			Id:                     "750000000000001",
			State:                  Bulk2JobStateJobComplete,
			Operation:              Bulk2OperationInsert,
			Object:                 "Account",
			NumberRecordsProcessed: 100,
			NumberRecordsFailed:    5,
		},
		{
			Id:                     "750000000000002",
			State:                  Bulk2JobStateInProgress,
			Operation:              Bulk2OperationUpdate,
			Object:                 "Contact",
			NumberRecordsProcessed: 50,
			NumberRecordsFailed:    0,
		},
	}

	var buf bytes.Buffer
	DisplayBulk2IngestJobList(jobs, &buf)
	output := buf.String()

	expectedParts := []string{
		"750000000000001",
		"750000000000002",
		"JobComplete",
		"InProgress",
		"insert",
		"update",
		"Account",
		"Contact",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("Output should contain %q, got:\n%s", part, output)
		}
	}
}

func TestDisplayBulk2IngestJobList_Empty(t *testing.T) {
	var buf bytes.Buffer
	DisplayBulk2IngestJobList([]Bulk2IngestJobInfo{}, &buf)
	output := buf.String()

	if !strings.Contains(output, "ID") {
		t.Errorf("Output should contain header even for empty list, got:\n%s", output)
	}
}

func TestDisplayBulk2QueryJobList(t *testing.T) {
	jobs := []Bulk2QueryJobInfo{
		{
			Id:                     "750000000000001",
			State:                  Bulk2JobStateJobComplete,
			Operation:              Bulk2OperationQuery,
			Object:                 "Account",
			NumberRecordsProcessed: 1000,
		},
		{
			Id:                     "750000000000002",
			State:                  Bulk2JobStateFailed,
			Operation:              Bulk2OperationQueryAll,
			Object:                 "Lead",
			NumberRecordsProcessed: 0,
		},
	}

	var buf bytes.Buffer
	DisplayBulk2QueryJobList(jobs, &buf)
	output := buf.String()

	expectedParts := []string{
		"750000000000001",
		"750000000000002",
		"JobComplete",
		"Failed",
		"query",
		"queryAll",
		"Account",
		"Lead",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("Output should contain %q, got:\n%s", part, output)
		}
	}
}

// =============================================================================
// Ingest Job Creation Tests
// =============================================================================

func TestCreateBulk2IngestJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/jobs/ingest") {
			t.Errorf("Unexpected URL path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"object":"Account"`) {
			t.Errorf("Request body should contain object field, got: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "7501234567890ABCDEF",
			"operation": "insert",
			"object": "Account",
			"state": "Open",
			"contentType": "CSV",
			"apiVersion": 55.0
		}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	request := Bulk2IngestJobRequest{
		Object:    "Account",
		Operation: Bulk2OperationInsert,
	}

	jobInfo, err := force.CreateBulk2IngestJob(request)
	if err != nil {
		t.Fatalf("CreateBulk2IngestJob failed: %v", err)
	}

	if jobInfo.Id != "7501234567890ABCDEF" {
		t.Errorf("Expected job ID '7501234567890ABCDEF', got '%s'", jobInfo.Id)
	}
	if jobInfo.State != Bulk2JobStateOpen {
		t.Errorf("Expected state 'Open', got '%s'", jobInfo.State)
	}
}

func TestCreateBulk2IngestJob_AllOperations(t *testing.T) {
	operations := []struct {
		operation      Bulk2Operation
		expectedInBody string
	}{
		{Bulk2OperationInsert, `"operation":"insert"`},
		{Bulk2OperationUpdate, `"operation":"update"`},
		{Bulk2OperationUpsert, `"operation":"upsert"`},
		{Bulk2OperationDelete, `"operation":"delete"`},
		{Bulk2OperationHardDelete, `"operation":"hardDelete"`},
	}

	for _, op := range operations {
		t.Run(string(op.operation), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(body), op.expectedInBody) {
					t.Errorf("Request body should contain %s, got: %s", op.expectedInBody, body)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"id": "750test", "state": "Open"}`))
			}))
			defer server.Close()

			force := &Force{
				Credentials: &ForceSession{
					InstanceUrl: server.URL,
					AccessToken: "test-token",
				},
			}

			request := Bulk2IngestJobRequest{
				Object:    "Account",
				Operation: op.operation,
			}

			_, err := force.CreateBulk2IngestJob(request)
			if err != nil {
				t.Fatalf("CreateBulk2IngestJob failed for %s: %v", op.operation, err)
			}
		})
	}
}

func TestCreateBulk2IngestJob_WithExternalId(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"externalIdFieldName":"External_Id__c"`) {
			t.Errorf("Request body should contain externalIdFieldName, got: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "750test", "state": "Open", "externalIdFieldName": "External_Id__c"}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	request := Bulk2IngestJobRequest{
		Object:              "Account",
		Operation:           Bulk2OperationUpsert,
		ExternalIdFieldName: "External_Id__c",
	}

	jobInfo, err := force.CreateBulk2IngestJob(request)
	if err != nil {
		t.Fatalf("CreateBulk2IngestJob failed: %v", err)
	}

	if jobInfo.ExternalIdFieldName != "External_Id__c" {
		t.Errorf("Expected externalIdFieldName 'External_Id__c', got '%s'", jobInfo.ExternalIdFieldName)
	}
}

func TestCreateBulk2IngestJob_WithDelimiterAndLineEnding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"columnDelimiter":"TAB"`) {
			t.Errorf("Request body should contain columnDelimiter, got: %s", body)
		}
		if !strings.Contains(string(body), `"lineEnding":"CRLF"`) {
			t.Errorf("Request body should contain lineEnding, got: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "750test", "state": "Open", "columnDelimiter": "TAB", "lineEnding": "CRLF"}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	request := Bulk2IngestJobRequest{
		Object:          "Account",
		Operation:       Bulk2OperationInsert,
		ColumnDelimiter: Bulk2DelimiterTab,
		LineEnding:      Bulk2LineEndingCRLF,
	}

	jobInfo, err := force.CreateBulk2IngestJob(request)
	if err != nil {
		t.Fatalf("CreateBulk2IngestJob failed: %v", err)
	}

	if jobInfo.ColumnDelimiter != Bulk2DelimiterTab {
		t.Errorf("Expected columnDelimiter 'TAB', got '%s'", jobInfo.ColumnDelimiter)
	}
	if jobInfo.LineEnding != Bulk2LineEndingCRLF {
		t.Errorf("Expected lineEnding 'CRLF', got '%s'", jobInfo.LineEnding)
	}
}

func TestCreateBulk2IngestJob_WithContext_Canceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "750test", "state": "Open"}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	request := Bulk2IngestJobRequest{
		Object:    "Account",
		Operation: Bulk2OperationInsert,
	}

	_, err := force.CreateBulk2IngestJobWithContext(ctx, request)
	if err == nil {
		t.Fatal("Expected error for canceled context")
	}
	if !strings.Contains(err.Error(), "canceled") {
		t.Errorf("Expected canceled error, got: %v", err)
	}
}

// =============================================================================
// Upload Data Tests
// =============================================================================

func TestUploadBulk2JobData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT request, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/batches") {
			t.Errorf("Unexpected URL path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != string(ContentTypeCsv) {
			t.Errorf("Expected Content-Type %s, got %s", ContentTypeCsv, r.Header.Get("Content-Type"))
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "Name") {
			t.Errorf("Request body should contain CSV header, got: %s", body)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	csvData := "Name,Description\nTest Account,A test account"
	err := force.UploadBulk2JobData("7501234567890ABCDEF", strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("UploadBulk2JobData failed: %v", err)
	}
}

func TestUploadBulk2JobData_LargeCSV(t *testing.T) {
	var receivedSize int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedSize = int64(len(body))
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	var csvBuilder strings.Builder
	csvBuilder.WriteString("Name,Description\n")
	for range 10000 {
		csvBuilder.WriteString("Test Account,A test account description that is reasonably long\n")
	}
	csvData := csvBuilder.String()

	err := force.UploadBulk2JobData("7501234567890ABCDEF", strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("UploadBulk2JobData failed: %v", err)
	}

	if receivedSize != int64(len(csvData)) {
		t.Errorf("Expected to receive %d bytes, got %d", len(csvData), receivedSize)
	}
}

func TestUploadBulk2JobData_WithContext_Canceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	csvData := "Name,Description\nTest Account,A test account"
	err := force.UploadBulk2JobDataWithContext(ctx, "7501234567890ABCDEF", strings.NewReader(csvData))
	if err == nil {
		t.Fatal("Expected error for canceled context")
	}
}

// =============================================================================
// Close/Abort Job Tests
// =============================================================================

func TestCloseBulk2IngestJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Expected PATCH request, got %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"state": "UploadComplete"`) {
			t.Errorf("Request body should contain state=UploadComplete, got: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "7501234567890ABCDEF",
			"state": "UploadComplete"
		}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	jobInfo, err := force.CloseBulk2IngestJob("7501234567890ABCDEF")
	if err != nil {
		t.Fatalf("CloseBulk2IngestJob failed: %v", err)
	}

	if jobInfo.State != Bulk2JobStateUploadComplete {
		t.Errorf("Expected state 'UploadComplete', got '%s'", jobInfo.State)
	}
}

func TestAbortBulk2IngestJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Expected PATCH request, got %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"state": "Aborted"`) {
			t.Errorf("Request body should contain state=Aborted, got: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "7501234567890ABCDEF",
			"state": "Aborted"
		}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	jobInfo, err := force.AbortBulk2IngestJob("7501234567890ABCDEF")
	if err != nil {
		t.Fatalf("AbortBulk2IngestJob failed: %v", err)
	}

	if jobInfo.State != Bulk2JobStateAborted {
		t.Errorf("Expected state 'Aborted', got '%s'", jobInfo.State)
	}
}

// =============================================================================
// Get Job Info Tests
// =============================================================================

func TestGetBulk2IngestJobInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/jobs/ingest/") {
			t.Errorf("Unexpected URL path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "7501234567890ABCDEF",
			"operation": "insert",
			"object": "Account",
			"state": "JobComplete",
			"numberRecordsProcessed": 100,
			"numberRecordsFailed": 5
		}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	jobInfo, err := force.GetBulk2IngestJobInfo("7501234567890ABCDEF")
	if err != nil {
		t.Fatalf("GetBulk2IngestJobInfo failed: %v", err)
	}

	if jobInfo.State != Bulk2JobStateJobComplete {
		t.Errorf("Expected state 'JobComplete', got '%s'", jobInfo.State)
	}
	if jobInfo.NumberRecordsProcessed != 100 {
		t.Errorf("Expected 100 records processed, got %d", jobInfo.NumberRecordsProcessed)
	}
	if jobInfo.NumberRecordsFailed != 5 {
		t.Errorf("Expected 5 records failed, got %d", jobInfo.NumberRecordsFailed)
	}
}

func TestGetBulk2IngestJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"done": true,
			"records": [
				{"id": "750000000000001", "state": "JobComplete", "operation": "insert"},
				{"id": "750000000000002", "state": "InProgress", "operation": "update"}
			]
		}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	jobs, err := force.GetBulk2IngestJobs()
	if err != nil {
		t.Fatalf("GetBulk2IngestJobs failed: %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(jobs))
	}
}

func TestGetBulk2IngestJobs_WithPagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if callCount == 1 {
			w.Write([]byte(`{
				"done": false,
				"nextRecordsUrl": "/services/data/v55.0/jobs/ingest?locator=abc",
				"records": [
					{"id": "750000000000001", "state": "JobComplete"}
				]
			}`))
		} else {
			w.Write([]byte(`{
				"done": true,
				"records": [
					{"id": "750000000000002", "state": "InProgress"}
				]
			}`))
		}
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	jobs, err := force.GetBulk2IngestJobs()
	if err != nil {
		t.Fatalf("GetBulk2IngestJobs failed: %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs after pagination, got %d", len(jobs))
	}
	if callCount != 2 {
		t.Errorf("Expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestDeleteBulk2IngestJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE request, got %s", r.Method)
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

	err := force.DeleteBulk2IngestJob("7501234567890ABCDEF")
	if err != nil {
		t.Fatalf("DeleteBulk2IngestJob failed: %v", err)
	}
}

// =============================================================================
// Results Tests
// =============================================================================

func TestGetBulk2SuccessfulResults(t *testing.T) {
	expectedCSV := "sf__Id,sf__Created,Name\n001000000000001AAA,true,Test Account\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/successfulResults") {
			t.Errorf("Unexpected URL path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedCSV))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	results, err := force.GetBulk2SuccessfulResults("7501234567890ABCDEF")
	if err != nil {
		t.Fatalf("GetBulk2SuccessfulResults failed: %v", err)
	}

	if string(results) != expectedCSV {
		t.Errorf("Expected results '%s', got '%s'", expectedCSV, string(results))
	}
}

func TestGetBulk2FailedResults(t *testing.T) {
	expectedCSV := "sf__Id,sf__Error,Name\n,REQUIRED_FIELD_MISSING:Name required,\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/failedResults") {
			t.Errorf("Unexpected URL path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedCSV))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	results, err := force.GetBulk2FailedResults("7501234567890ABCDEF")
	if err != nil {
		t.Fatalf("GetBulk2FailedResults failed: %v", err)
	}

	if string(results) != expectedCSV {
		t.Errorf("Expected results '%s', got '%s'", expectedCSV, string(results))
	}
}

func TestGetBulk2UnprocessedRecords(t *testing.T) {
	expectedCSV := "Name,Description\nUnprocessed Account,Pending\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/unprocessedrecords") {
			t.Errorf("Unexpected URL path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedCSV))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	results, err := force.GetBulk2UnprocessedRecords("7501234567890ABCDEF")
	if err != nil {
		t.Fatalf("GetBulk2UnprocessedRecords failed: %v", err)
	}

	if string(results) != expectedCSV {
		t.Errorf("Expected results '%s', got '%s'", expectedCSV, string(results))
	}
}

func TestGetBulk2SuccessfulResults_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	results, err := force.GetBulk2SuccessfulResults("7501234567890ABCDEF")
	if err != nil {
		t.Fatalf("GetBulk2SuccessfulResults failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected empty results, got '%s'", string(results))
	}
}

// =============================================================================
// Query Job Tests
// =============================================================================

func TestCreateBulk2QueryJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/jobs/query") {
			t.Errorf("Unexpected URL path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"query":"SELECT Id, Name FROM Account"`) {
			t.Errorf("Request body should contain query field, got: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "7501234567890QUERY",
			"operation": "query",
			"object": "Account",
			"state": "UploadComplete",
			"apiVersion": 55.0
		}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	request := Bulk2QueryJobRequest{
		Operation: Bulk2OperationQuery,
		Query:     "SELECT Id, Name FROM Account",
	}

	jobInfo, err := force.CreateBulk2QueryJob(request)
	if err != nil {
		t.Fatalf("CreateBulk2QueryJob failed: %v", err)
	}

	if jobInfo.Id != "7501234567890QUERY" {
		t.Errorf("Expected job ID '7501234567890QUERY', got '%s'", jobInfo.Id)
	}
}

func TestCreateBulk2QueryJob_QueryAll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"operation":"queryAll"`) {
			t.Errorf("Request body should contain operation:queryAll, got: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "750test", "state": "UploadComplete", "operation": "queryAll"}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	request := Bulk2QueryJobRequest{
		Operation: Bulk2OperationQueryAll,
		Query:     "SELECT Id FROM Account",
	}

	jobInfo, err := force.CreateBulk2QueryJob(request)
	if err != nil {
		t.Fatalf("CreateBulk2QueryJob failed: %v", err)
	}

	if jobInfo.Operation != Bulk2OperationQueryAll {
		t.Errorf("Expected operation 'queryAll', got '%s'", jobInfo.Operation)
	}
}

func TestGetBulk2QueryJobInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "7501234567890QUERY",
			"operation": "query",
			"object": "Account",
			"state": "JobComplete",
			"numberRecordsProcessed": 1000
		}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	jobInfo, err := force.GetBulk2QueryJobInfo("7501234567890QUERY")
	if err != nil {
		t.Fatalf("GetBulk2QueryJobInfo failed: %v", err)
	}

	if jobInfo.State != Bulk2JobStateJobComplete {
		t.Errorf("Expected state 'JobComplete', got '%s'", jobInfo.State)
	}
	if jobInfo.NumberRecordsProcessed != 1000 {
		t.Errorf("Expected 1000 records processed, got %d", jobInfo.NumberRecordsProcessed)
	}
}

func TestGetBulk2QueryResults(t *testing.T) {
	expectedCSV := "Id,Name\n001000000000001AAA,Test Account\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/results") {
			t.Errorf("Unexpected URL path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Sforce-Locator", "null")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedCSV))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	results, err := force.GetBulk2QueryResults("7501234567890QUERY", "", 0)
	if err != nil {
		t.Fatalf("GetBulk2QueryResults failed: %v", err)
	}

	if string(results.Data) != expectedCSV {
		t.Errorf("Expected data '%s', got '%s'", expectedCSV, string(results.Data))
	}
	if results.Locator != "" {
		t.Errorf("Expected empty locator, got '%s'", results.Locator)
	}
}

func TestGetBulk2QueryResults_WithPagination(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/csv")

		if callCount == 1 {
			if strings.Contains(r.URL.RawQuery, "locator=") {
				t.Errorf("First call should not have locator, got: %s", r.URL.RawQuery)
			}
			w.Header().Set("Sforce-Locator", "ABC123")
			w.Write([]byte("Id,Name\n001000000000001AAA,Test 1\n"))
		} else {
			if !strings.Contains(r.URL.RawQuery, "locator=ABC123") {
				t.Errorf("Second call should have locator=ABC123, got: %s", r.URL.RawQuery)
			}
			w.Header().Set("Sforce-Locator", "null")
			w.Write([]byte("Id,Name\n001000000000002BBB,Test 2\n"))
		}
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	results, err := force.GetBulk2QueryResults("7501234567890QUERY", "", 0)
	if err != nil {
		t.Fatalf("GetBulk2QueryResults failed: %v", err)
	}
	if results.Locator != "ABC123" {
		t.Errorf("Expected locator 'ABC123', got '%s'", results.Locator)
	}

	results, err = force.GetBulk2QueryResults("7501234567890QUERY", "ABC123", 0)
	if err != nil {
		t.Fatalf("GetBulk2QueryResults failed: %v", err)
	}
	if results.Locator != "" {
		t.Errorf("Expected empty locator, got '%s'", results.Locator)
	}
}

func TestGetBulk2QueryResults_WithMaxRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "maxRecords=100") {
			t.Errorf("Expected maxRecords=100, got: %s", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Sforce-Locator", "null")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Id,Name\n001test,Test\n"))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	_, err := force.GetBulk2QueryResults("7501234567890QUERY", "", 100)
	if err != nil {
		t.Fatalf("GetBulk2QueryResults failed: %v", err)
	}
}

func TestAbortBulk2QueryJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Expected PATCH request, got %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"state": "Aborted"`) {
			t.Errorf("Request body should contain state=Aborted, got: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "750test", "state": "Aborted"}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	jobInfo, err := force.AbortBulk2QueryJob("750test")
	if err != nil {
		t.Fatalf("AbortBulk2QueryJob failed: %v", err)
	}

	if jobInfo.State != Bulk2JobStateAborted {
		t.Errorf("Expected state 'Aborted', got '%s'", jobInfo.State)
	}
}

func TestDeleteBulk2QueryJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE request, got %s", r.Method)
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

	err := force.DeleteBulk2QueryJob("750test")
	if err != nil {
		t.Fatalf("DeleteBulk2QueryJob failed: %v", err)
	}
}

func TestGetBulk2QueryJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"done": true,
			"records": [
				{"id": "750000000000001", "state": "JobComplete", "operation": "query"},
				{"id": "750000000000002", "state": "InProgress", "operation": "queryAll"}
			]
		}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	jobs, err := force.GetBulk2QueryJobs()
	if err != nil {
		t.Fatalf("GetBulk2QueryJobs failed: %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(jobs))
	}
}

// =============================================================================
// Wait/Polling Tests
// =============================================================================

func TestWaitForBulk2IngestJob(t *testing.T) {
	callCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		count := atomic.LoadInt32(&callCount)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if count < 3 {
			w.Write([]byte(`{"id": "750test", "state": "InProgress", "numberRecordsProcessed": 50}`))
		} else {
			w.Write([]byte(`{"id": "750test", "state": "JobComplete", "numberRecordsProcessed": 100}`))
		}
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	var callbackCount int
	callback := func(jobInfo any) {
		callbackCount++
	}

	jobInfo, err := force.WaitForBulk2IngestJob("750test", 10*time.Millisecond, callback)
	if err != nil {
		t.Fatalf("WaitForBulk2IngestJob failed: %v", err)
	}

	if jobInfo.State != Bulk2JobStateJobComplete {
		t.Errorf("Expected state 'JobComplete', got '%s'", jobInfo.State)
	}
	if callbackCount < 3 {
		t.Errorf("Expected at least 3 callback calls, got %d", callbackCount)
	}
}

func TestWaitForBulk2IngestJob_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "750test", "state": "Failed", "errorMessage": "Invalid data"}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	jobInfo, err := force.WaitForBulk2IngestJob("750test", 10*time.Millisecond, nil)
	if err != nil {
		t.Fatalf("WaitForBulk2IngestJob failed: %v", err)
	}

	if jobInfo.State != Bulk2JobStateFailed {
		t.Errorf("Expected state 'Failed', got '%s'", jobInfo.State)
	}
	if jobInfo.ErrorMessage != "Invalid data" {
		t.Errorf("Expected error message 'Invalid data', got '%s'", jobInfo.ErrorMessage)
	}
}

func TestWaitForBulk2IngestJobWithContext_Canceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "750test", "state": "InProgress"}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := force.WaitForBulk2IngestJobWithContext(ctx, "750test", 100*time.Millisecond, nil)
	if err == nil {
		t.Fatal("Expected error for canceled context")
	}
	if !strings.Contains(err.Error(), "canceled") {
		t.Errorf("Expected canceled error, got: %v", err)
	}
}

func TestWaitForBulk2QueryJob(t *testing.T) {
	callCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		count := atomic.LoadInt32(&callCount)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if count < 2 {
			w.Write([]byte(`{"id": "750test", "state": "InProgress"}`))
		} else {
			w.Write([]byte(`{"id": "750test", "state": "JobComplete", "numberRecordsProcessed": 500}`))
		}
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	jobInfo, err := force.WaitForBulk2QueryJob("750test", 10*time.Millisecond, nil)
	if err != nil {
		t.Fatalf("WaitForBulk2QueryJob failed: %v", err)
	}

	if jobInfo.State != Bulk2JobStateJobComplete {
		t.Errorf("Expected state 'JobComplete', got '%s'", jobInfo.State)
	}
}

// =============================================================================
// Warning Header Tests
// =============================================================================

func TestParseBulk2Warnings(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Warning": []string{
				"199 - Deprecated API version",
				"199 - Feature will be removed",
			},
		},
	}

	warnings := ParseBulk2Warnings(resp)
	if len(warnings) != 2 {
		t.Errorf("Expected 2 warnings, got %d", len(warnings))
	}
	if warnings[0] != "199 - Deprecated API version" {
		t.Errorf("Unexpected warning: %s", warnings[0])
	}
}

func TestParseBulk2Warnings_NoWarnings(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
	}

	warnings := ParseBulk2Warnings(resp)
	if len(warnings) != 0 {
		t.Errorf("Expected 0 warnings, got %d", len(warnings))
	}
}

// =============================================================================
// JSON Serialization Tests
// =============================================================================

func TestBulk2IngestJobRequest_JSON(t *testing.T) {
	request := Bulk2IngestJobRequest{
		Object:              "Account",
		Operation:           Bulk2OperationUpsert,
		ExternalIdFieldName: "External_Id__c",
		ColumnDelimiter:     Bulk2DelimiterTab,
		LineEnding:          Bulk2LineEndingCRLF,
	}

	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(data, &parsed)

	if parsed["object"] != "Account" {
		t.Errorf("Expected object=Account, got %v", parsed["object"])
	}
	if parsed["operation"] != "upsert" {
		t.Errorf("Expected operation=upsert, got %v", parsed["operation"])
	}
	if parsed["externalIdFieldName"] != "External_Id__c" {
		t.Errorf("Expected externalIdFieldName=External_Id__c, got %v", parsed["externalIdFieldName"])
	}
}

func TestBulk2QueryJobRequest_JSON(t *testing.T) {
	request := Bulk2QueryJobRequest{
		Operation:       Bulk2OperationQueryAll,
		Query:           "SELECT Id FROM Account WHERE IsDeleted = true",
		ColumnDelimiter: Bulk2DelimiterPipe,
	}

	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(data, &parsed)

	if parsed["operation"] != "queryAll" {
		t.Errorf("Expected operation=queryAll, got %v", parsed["operation"])
	}
	if parsed["columnDelimiter"] != "PIPE" {
		t.Errorf("Expected columnDelimiter=PIPE, got %v", parsed["columnDelimiter"])
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestCreateBulk2IngestJob_DefaultContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"contentType":"CSV"`) {
			t.Errorf("Request body should contain contentType:CSV, got: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "750test", "state": "Open"}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	request := Bulk2IngestJobRequest{
		Object:    "Account",
		Operation: Bulk2OperationInsert,
	}

	_, err := force.CreateBulk2IngestJob(request)
	if err != nil {
		t.Fatalf("CreateBulk2IngestJob failed: %v", err)
	}
}

func TestCreateBulk2QueryJob_DefaultOperation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"operation":"query"`) {
			t.Errorf("Request body should contain operation:query by default, got: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "750test", "state": "UploadComplete"}`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	request := Bulk2QueryJobRequest{
		Query: "SELECT Id FROM Account",
	}

	_, err := force.CreateBulk2QueryJob(request)
	if err != nil {
		t.Fatalf("CreateBulk2QueryJob failed: %v", err)
	}
}

func TestAllBulk2Delimiters(t *testing.T) {
	delimiters := []Bulk2ColumnDelimiter{
		Bulk2DelimiterComma,
		Bulk2DelimiterTab,
		Bulk2DelimiterPipe,
		Bulk2DelimiterSemicolon,
		Bulk2DelimiterCaret,
		Bulk2DelimiterBackquote,
	}

	for _, d := range delimiters {
		t.Run(string(d), func(t *testing.T) {
			request := Bulk2IngestJobRequest{
				Object:          "Account",
				Operation:       Bulk2OperationInsert,
				ColumnDelimiter: d,
			}
			data, err := json.Marshal(request)
			if err != nil {
				t.Fatalf("Failed to marshal with delimiter %s: %v", d, err)
			}
			if !strings.Contains(string(data), string(d)) {
				t.Errorf("JSON should contain delimiter %s, got: %s", d, data)
			}
		})
	}
}

func TestAllBulk2LineEndings(t *testing.T) {
	lineEndings := []Bulk2LineEnding{
		Bulk2LineEndingLF,
		Bulk2LineEndingCRLF,
	}

	for _, le := range lineEndings {
		t.Run(string(le), func(t *testing.T) {
			request := Bulk2IngestJobRequest{
				Object:     "Account",
				Operation:  Bulk2OperationInsert,
				LineEnding: le,
			}
			data, err := json.Marshal(request)
			if err != nil {
				t.Fatalf("Failed to marshal with line ending %s: %v", le, err)
			}
			if !strings.Contains(string(data), string(le)) {
				t.Errorf("JSON should contain line ending %s, got: %s", le, data)
			}
		})
	}
}
