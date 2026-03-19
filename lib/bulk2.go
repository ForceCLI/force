package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ForceCLI/force/lib/internal"
)

// Bulk2Operation represents the type of operation for a Bulk API 2.0 job
type Bulk2Operation string

const (
	Bulk2OperationInsert     Bulk2Operation = "insert"
	Bulk2OperationUpdate     Bulk2Operation = "update"
	Bulk2OperationUpsert     Bulk2Operation = "upsert"
	Bulk2OperationDelete     Bulk2Operation = "delete"
	Bulk2OperationHardDelete Bulk2Operation = "hardDelete"
	Bulk2OperationQuery      Bulk2Operation = "query"
	Bulk2OperationQueryAll   Bulk2Operation = "queryAll"
)

// Bulk2JobState represents the state of a Bulk API 2.0 job
type Bulk2JobState string

const (
	Bulk2JobStateOpen           Bulk2JobState = "Open"
	Bulk2JobStateUploadComplete Bulk2JobState = "UploadComplete"
	Bulk2JobStateInProgress     Bulk2JobState = "InProgress"
	Bulk2JobStateJobComplete    Bulk2JobState = "JobComplete"
	Bulk2JobStateFailed         Bulk2JobState = "Failed"
	Bulk2JobStateAborted        Bulk2JobState = "Aborted"
)

// Bulk2ColumnDelimiter represents the column delimiter for CSV data
type Bulk2ColumnDelimiter string

const (
	Bulk2DelimiterComma     Bulk2ColumnDelimiter = "COMMA"
	Bulk2DelimiterTab       Bulk2ColumnDelimiter = "TAB"
	Bulk2DelimiterPipe      Bulk2ColumnDelimiter = "PIPE"
	Bulk2DelimiterSemicolon Bulk2ColumnDelimiter = "SEMICOLON"
	Bulk2DelimiterCaret     Bulk2ColumnDelimiter = "CARET"
	Bulk2DelimiterBackquote Bulk2ColumnDelimiter = "BACKQUOTE"
)

// Bulk2LineEnding represents the line ending for CSV data
type Bulk2LineEnding string

const (
	Bulk2LineEndingLF   Bulk2LineEnding = "LF"
	Bulk2LineEndingCRLF Bulk2LineEnding = "CRLF"
)

// Bulk2IngestJobRequest is the request body for creating an ingest job
type Bulk2IngestJobRequest struct {
	Object              string               `json:"object"`
	Operation           Bulk2Operation       `json:"operation"`
	ExternalIdFieldName string               `json:"externalIdFieldName,omitempty"`
	ColumnDelimiter     Bulk2ColumnDelimiter `json:"columnDelimiter,omitempty"`
	LineEnding          Bulk2LineEnding      `json:"lineEnding,omitempty"`
	ContentType         string               `json:"contentType,omitempty"`
}

// Bulk2IngestJobInfo is the response for an ingest job
type Bulk2IngestJobInfo struct {
	Id                      string               `json:"id"`
	Operation               Bulk2Operation       `json:"operation"`
	Object                  string               `json:"object"`
	CreatedById             string               `json:"createdById"`
	CreatedDate             string               `json:"createdDate"`
	SystemModstamp          string               `json:"systemModstamp"`
	State                   Bulk2JobState        `json:"state"`
	ExternalIdFieldName     string               `json:"externalIdFieldName,omitempty"`
	ConcurrencyMode         string               `json:"concurrencyMode"`
	ContentType             string               `json:"contentType"`
	ApiVersion              float64              `json:"apiVersion"`
	ContentUrl              string               `json:"contentUrl,omitempty"`
	LineEnding              Bulk2LineEnding      `json:"lineEnding"`
	ColumnDelimiter         Bulk2ColumnDelimiter `json:"columnDelimiter"`
	JobType                 string               `json:"jobType,omitempty"`
	NumberRecordsProcessed  int                  `json:"numberRecordsProcessed"`
	NumberRecordsFailed     int                  `json:"numberRecordsFailed"`
	Retries                 int                  `json:"retries"`
	TotalProcessingTime     int                  `json:"totalProcessingTime"`
	ApiActiveProcessingTime int                  `json:"apiActiveProcessingTime"`
	ApexProcessingTime      int                  `json:"apexProcessingTime"`
	ErrorMessage            string               `json:"errorMessage,omitempty"`
}

// IsTerminal returns true if the job state is a terminal state
func (j *Bulk2IngestJobInfo) IsTerminal() bool {
	return j.State == Bulk2JobStateJobComplete ||
		j.State == Bulk2JobStateFailed ||
		j.State == Bulk2JobStateAborted
}

// Bulk2IngestJobList is the response for listing ingest jobs
type Bulk2IngestJobList struct {
	Done           bool                 `json:"done"`
	Records        []Bulk2IngestJobInfo `json:"records"`
	NextRecordsUrl string               `json:"nextRecordsUrl,omitempty"`
}

// Bulk2QueryJobRequest is the request body for creating a query job
type Bulk2QueryJobRequest struct {
	Operation       Bulk2Operation       `json:"operation"`
	Query           string               `json:"query"`
	ColumnDelimiter Bulk2ColumnDelimiter `json:"columnDelimiter,omitempty"`
	LineEnding      Bulk2LineEnding      `json:"lineEnding,omitempty"`
}

// Bulk2QueryJobInfo is the response for a query job
type Bulk2QueryJobInfo struct {
	Id                     string               `json:"id"`
	Operation              Bulk2Operation       `json:"operation"`
	Object                 string               `json:"object"`
	CreatedById            string               `json:"createdById"`
	CreatedDate            string               `json:"createdDate"`
	SystemModstamp         string               `json:"systemModstamp"`
	State                  Bulk2JobState        `json:"state"`
	ConcurrencyMode        string               `json:"concurrencyMode"`
	ContentType            string               `json:"contentType"`
	ApiVersion             float64              `json:"apiVersion"`
	LineEnding             Bulk2LineEnding      `json:"lineEnding"`
	ColumnDelimiter        Bulk2ColumnDelimiter `json:"columnDelimiter"`
	JobType                string               `json:"jobType,omitempty"`
	NumberRecordsProcessed int                  `json:"numberRecordsProcessed"`
	Retries                int                  `json:"retries"`
	TotalProcessingTime    int                  `json:"totalProcessingTime"`
	ErrorMessage           string               `json:"errorMessage,omitempty"`
}

// IsTerminal returns true if the job state is a terminal state
func (j *Bulk2QueryJobInfo) IsTerminal() bool {
	return j.State == Bulk2JobStateJobComplete ||
		j.State == Bulk2JobStateFailed ||
		j.State == Bulk2JobStateAborted
}

// Bulk2QueryJobList is the response for listing query jobs
type Bulk2QueryJobList struct {
	Done           bool                `json:"done"`
	Records        []Bulk2QueryJobInfo `json:"records"`
	NextRecordsUrl string              `json:"nextRecordsUrl,omitempty"`
}

// Bulk2QueryResults holds query results with pagination info
type Bulk2QueryResults struct {
	Data    []byte
	Locator string
}

// bulk2IngestUrl returns the URL for the Bulk API 2.0 ingest endpoint
func (f *Force) bulk2IngestUrl() string {
	return fmt.Sprintf("%s/services/data/%s/jobs/ingest", f.Credentials.InstanceUrl, apiVersion)
}

// bulk2QueryUrl returns the URL for the Bulk API 2.0 query endpoint
func (f *Force) bulk2QueryUrl() string {
	return fmt.Sprintf("%s/services/data/%s/jobs/query", f.Credentials.InstanceUrl, apiVersion)
}

// CreateBulk2IngestJob creates a new Bulk API 2.0 ingest job
func (f *Force) CreateBulk2IngestJob(request Bulk2IngestJobRequest) (Bulk2IngestJobInfo, error) {
	if request.ContentType == "" {
		request.ContentType = "CSV"
	}
	url := f.bulk2IngestUrl()
	body, err := json.Marshal(request)
	if err != nil {
		return Bulk2IngestJobInfo{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req := NewRequest("POST").
		AbsoluteUrl(url).
		WithContent(ContentTypeJson).
		WithBody(strings.NewReader(string(body))).
		ReadResponseBody()

	resp, err := f.ExecuteRequest(req)
	if err != nil {
		return Bulk2IngestJobInfo{}, err
	}

	var result Bulk2IngestJobInfo
	if err := internal.JsonUnmarshal(resp.ReadResponseBody, &result); err != nil {
		return Bulk2IngestJobInfo{}, err
	}
	return result, nil
}

// CreateBulk2IngestJobWithContext creates a new Bulk API 2.0 ingest job with context
func (f *Force) CreateBulk2IngestJobWithContext(ctx context.Context, request Bulk2IngestJobRequest) (Bulk2IngestJobInfo, error) {
	done := make(chan struct{})
	var result Bulk2IngestJobInfo
	var err error
	go func() {
		defer close(done)
		result, err = f.CreateBulk2IngestJob(request)
	}()
	select {
	case <-ctx.Done():
		return Bulk2IngestJobInfo{}, fmt.Errorf("CreateBulk2IngestJob canceled: %w", ctx.Err())
	case <-done:
		return result, err
	}
}

// UploadBulk2JobData uploads CSV data to a Bulk API 2.0 job
func (f *Force) UploadBulk2JobData(jobId string, csvReader io.Reader) error {
	url := fmt.Sprintf("%s/%s/batches", f.bulk2IngestUrl(), jobId)

	req := NewRequest("PUT").
		AbsoluteUrl(url).
		WithContent(ContentTypeCsv).
		WithBody(csvReader).
		ReadResponseBody()

	_, err := f.ExecuteRequest(req)
	return err
}

// UploadBulk2JobDataWithContext uploads CSV data to a Bulk API 2.0 job with context
func (f *Force) UploadBulk2JobDataWithContext(ctx context.Context, jobId string, csvReader io.Reader) error {
	done := make(chan struct{})
	var err error
	go func() {
		defer close(done)
		err = f.UploadBulk2JobData(jobId, csvReader)
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("UploadBulk2JobData canceled: %w", ctx.Err())
	case <-done:
		return err
	}
}

// CloseBulk2IngestJob closes a Bulk API 2.0 ingest job (sets state to UploadComplete)
func (f *Force) CloseBulk2IngestJob(jobId string) (Bulk2IngestJobInfo, error) {
	return f.patchBulk2IngestJobState(jobId, Bulk2JobStateUploadComplete)
}

// CloseBulk2IngestJobWithContext closes a Bulk API 2.0 ingest job with context
func (f *Force) CloseBulk2IngestJobWithContext(ctx context.Context, jobId string) (Bulk2IngestJobInfo, error) {
	done := make(chan struct{})
	var result Bulk2IngestJobInfo
	var err error
	go func() {
		defer close(done)
		result, err = f.CloseBulk2IngestJob(jobId)
	}()
	select {
	case <-ctx.Done():
		return Bulk2IngestJobInfo{}, fmt.Errorf("CloseBulk2IngestJob canceled: %w", ctx.Err())
	case <-done:
		return result, err
	}
}

// AbortBulk2IngestJob aborts a Bulk API 2.0 ingest job
func (f *Force) AbortBulk2IngestJob(jobId string) (Bulk2IngestJobInfo, error) {
	return f.patchBulk2IngestJobState(jobId, Bulk2JobStateAborted)
}

// AbortBulk2IngestJobWithContext aborts a Bulk API 2.0 ingest job with context
func (f *Force) AbortBulk2IngestJobWithContext(ctx context.Context, jobId string) (Bulk2IngestJobInfo, error) {
	done := make(chan struct{})
	var result Bulk2IngestJobInfo
	var err error
	go func() {
		defer close(done)
		result, err = f.AbortBulk2IngestJob(jobId)
	}()
	select {
	case <-ctx.Done():
		return Bulk2IngestJobInfo{}, fmt.Errorf("AbortBulk2IngestJob canceled: %w", ctx.Err())
	case <-done:
		return result, err
	}
}

func (f *Force) patchBulk2IngestJobState(jobId string, state Bulk2JobState) (Bulk2IngestJobInfo, error) {
	url := fmt.Sprintf("%s/%s", f.bulk2IngestUrl(), jobId)
	body := fmt.Sprintf(`{"state": "%s"}`, state)

	req := NewRequest("PATCH").
		AbsoluteUrl(url).
		WithContent(ContentTypeJson).
		WithBody(strings.NewReader(body)).
		ReadResponseBody()

	resp, err := f.ExecuteRequest(req)
	if err != nil {
		return Bulk2IngestJobInfo{}, err
	}

	var result Bulk2IngestJobInfo
	if err := internal.JsonUnmarshal(resp.ReadResponseBody, &result); err != nil {
		return Bulk2IngestJobInfo{}, err
	}
	return result, nil
}

// GetBulk2IngestJobInfo retrieves information about a Bulk API 2.0 ingest job
func (f *Force) GetBulk2IngestJobInfo(jobId string) (Bulk2IngestJobInfo, error) {
	url := fmt.Sprintf("%s/%s", f.bulk2IngestUrl(), jobId)

	req := NewRequest("GET").
		AbsoluteUrl(url).
		WithContent(ContentTypeJson).
		ReadResponseBody()

	resp, err := f.ExecuteRequest(req)
	if err != nil {
		return Bulk2IngestJobInfo{}, err
	}

	var result Bulk2IngestJobInfo
	if err := internal.JsonUnmarshal(resp.ReadResponseBody, &result); err != nil {
		return Bulk2IngestJobInfo{}, err
	}
	return result, nil
}

// GetBulk2IngestJobInfoWithContext retrieves information about a Bulk API 2.0 ingest job with context
func (f *Force) GetBulk2IngestJobInfoWithContext(ctx context.Context, jobId string) (Bulk2IngestJobInfo, error) {
	done := make(chan struct{})
	var result Bulk2IngestJobInfo
	var err error
	go func() {
		defer close(done)
		result, err = f.GetBulk2IngestJobInfo(jobId)
	}()
	select {
	case <-ctx.Done():
		return Bulk2IngestJobInfo{}, fmt.Errorf("GetBulk2IngestJobInfo canceled: %w", ctx.Err())
	case <-done:
		return result, err
	}
}

// GetBulk2IngestJobs retrieves all Bulk API 2.0 ingest jobs
func (f *Force) GetBulk2IngestJobs() ([]Bulk2IngestJobInfo, error) {
	url := f.bulk2IngestUrl()

	req := NewRequest("GET").
		AbsoluteUrl(url).
		WithContent(ContentTypeJson).
		ReadResponseBody()

	resp, err := f.ExecuteRequest(req)
	if err != nil {
		return nil, err
	}

	var result Bulk2IngestJobList
	if err := internal.JsonUnmarshal(resp.ReadResponseBody, &result); err != nil {
		return nil, err
	}

	jobs := result.Records
	for !result.Done && result.NextRecordsUrl != "" {
		nextUrl := fmt.Sprintf("%s%s", f.Credentials.InstanceUrl, result.NextRecordsUrl)
		req := NewRequest("GET").
			AbsoluteUrl(nextUrl).
			WithContent(ContentTypeJson).
			ReadResponseBody()

		resp, err := f.ExecuteRequest(req)
		if err != nil {
			return nil, err
		}

		if err := internal.JsonUnmarshal(resp.ReadResponseBody, &result); err != nil {
			return nil, err
		}
		jobs = append(jobs, result.Records...)
	}

	return jobs, nil
}

// DeleteBulk2IngestJob deletes a Bulk API 2.0 ingest job
func (f *Force) DeleteBulk2IngestJob(jobId string) error {
	url := fmt.Sprintf("%s/%s", f.bulk2IngestUrl(), jobId)

	req := NewRequest("DELETE").
		AbsoluteUrl(url).
		ReadResponseBody()

	_, err := f.ExecuteRequest(req)
	return err
}

// GetBulk2SuccessfulResults retrieves successful results for a Bulk API 2.0 ingest job
func (f *Force) GetBulk2SuccessfulResults(jobId string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/successfulResults", f.bulk2IngestUrl(), jobId)
	return f.getBulk2Results(url)
}

// GetBulk2FailedResults retrieves failed results for a Bulk API 2.0 ingest job
func (f *Force) GetBulk2FailedResults(jobId string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/failedResults", f.bulk2IngestUrl(), jobId)
	return f.getBulk2Results(url)
}

// GetBulk2UnprocessedRecords retrieves unprocessed records for a Bulk API 2.0 ingest job
func (f *Force) GetBulk2UnprocessedRecords(jobId string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/unprocessedrecords", f.bulk2IngestUrl(), jobId)
	return f.getBulk2Results(url)
}

func (f *Force) getBulk2Results(url string) ([]byte, error) {
	req := NewRequest("GET").
		AbsoluteUrl(url).
		WithHeader("Accept", "text/csv").
		ReadResponseBody()

	resp, err := f.ExecuteRequest(req)
	if err != nil {
		return nil, err
	}
	return resp.ReadResponseBody, nil
}

// GetBulk2ResultsWithCallback retrieves results for a Bulk API 2.0 job using a callback
func (f *Force) GetBulk2ResultsWithCallback(url string, callback HttpCallback) error {
	req := NewRequest("GET").
		AbsoluteUrl(url).
		WithHeader("Accept", "text/csv").
		WithResponseCallback(callback)

	_, err := f.ExecuteRequest(req)
	return err
}

// CreateBulk2QueryJob creates a new Bulk API 2.0 query job
func (f *Force) CreateBulk2QueryJob(request Bulk2QueryJobRequest) (Bulk2QueryJobInfo, error) {
	if request.Operation == "" {
		request.Operation = Bulk2OperationQuery
	}
	url := f.bulk2QueryUrl()
	body, err := json.Marshal(request)
	if err != nil {
		return Bulk2QueryJobInfo{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req := NewRequest("POST").
		AbsoluteUrl(url).
		WithContent(ContentTypeJson).
		WithBody(strings.NewReader(string(body))).
		ReadResponseBody()

	resp, err := f.ExecuteRequest(req)
	if err != nil {
		return Bulk2QueryJobInfo{}, err
	}

	var result Bulk2QueryJobInfo
	if err := internal.JsonUnmarshal(resp.ReadResponseBody, &result); err != nil {
		return Bulk2QueryJobInfo{}, err
	}
	return result, nil
}

// CreateBulk2QueryJobWithContext creates a new Bulk API 2.0 query job with context
func (f *Force) CreateBulk2QueryJobWithContext(ctx context.Context, request Bulk2QueryJobRequest) (Bulk2QueryJobInfo, error) {
	done := make(chan struct{})
	var result Bulk2QueryJobInfo
	var err error
	go func() {
		defer close(done)
		result, err = f.CreateBulk2QueryJob(request)
	}()
	select {
	case <-ctx.Done():
		return Bulk2QueryJobInfo{}, fmt.Errorf("CreateBulk2QueryJob canceled: %w", ctx.Err())
	case <-done:
		return result, err
	}
}

// GetBulk2QueryJobInfo retrieves information about a Bulk API 2.0 query job
func (f *Force) GetBulk2QueryJobInfo(jobId string) (Bulk2QueryJobInfo, error) {
	url := fmt.Sprintf("%s/%s", f.bulk2QueryUrl(), jobId)

	req := NewRequest("GET").
		AbsoluteUrl(url).
		WithContent(ContentTypeJson).
		ReadResponseBody()

	resp, err := f.ExecuteRequest(req)
	if err != nil {
		return Bulk2QueryJobInfo{}, err
	}

	var result Bulk2QueryJobInfo
	if err := internal.JsonUnmarshal(resp.ReadResponseBody, &result); err != nil {
		return Bulk2QueryJobInfo{}, err
	}
	return result, nil
}

// GetBulk2QueryJobInfoWithContext retrieves information about a Bulk API 2.0 query job with context
func (f *Force) GetBulk2QueryJobInfoWithContext(ctx context.Context, jobId string) (Bulk2QueryJobInfo, error) {
	done := make(chan struct{})
	var result Bulk2QueryJobInfo
	var err error
	go func() {
		defer close(done)
		result, err = f.GetBulk2QueryJobInfo(jobId)
	}()
	select {
	case <-ctx.Done():
		return Bulk2QueryJobInfo{}, fmt.Errorf("GetBulk2QueryJobInfo canceled: %w", ctx.Err())
	case <-done:
		return result, err
	}
}

// GetBulk2QueryResults retrieves results for a Bulk API 2.0 query job
// locator is used for pagination, pass empty string for the first page
// maxRecords limits the number of records returned (0 for default)
func (f *Force) GetBulk2QueryResults(jobId string, locator string, maxRecords int) (Bulk2QueryResults, error) {
	url := fmt.Sprintf("%s/%s/results", f.bulk2QueryUrl(), jobId)
	if locator != "" {
		url = fmt.Sprintf("%s?locator=%s", url, locator)
		if maxRecords > 0 {
			url = fmt.Sprintf("%s&maxRecords=%d", url, maxRecords)
		}
	} else if maxRecords > 0 {
		url = fmt.Sprintf("%s?maxRecords=%d", url, maxRecords)
	}

	req := NewRequest("GET").
		AbsoluteUrl(url).
		WithHeader("Accept", "text/csv").
		ReadResponseBody()

	resp, err := f.ExecuteRequest(req)
	if err != nil {
		return Bulk2QueryResults{}, err
	}

	result := Bulk2QueryResults{
		Data:    resp.ReadResponseBody,
		Locator: resp.HttpResponse.Header.Get("Sforce-Locator"),
	}
	if result.Locator == "null" {
		result.Locator = ""
	}
	return result, nil
}

// GetBulk2QueryResultsWithContext retrieves results for a Bulk API 2.0 query job with context
func (f *Force) GetBulk2QueryResultsWithContext(ctx context.Context, jobId string, locator string, maxRecords int) (Bulk2QueryResults, error) {
	done := make(chan struct{})
	var result Bulk2QueryResults
	var err error
	go func() {
		defer close(done)
		result, err = f.GetBulk2QueryResults(jobId, locator, maxRecords)
	}()
	select {
	case <-ctx.Done():
		return Bulk2QueryResults{}, fmt.Errorf("GetBulk2QueryResults canceled: %w", ctx.Err())
	case <-done:
		return result, err
	}
}

// AbortBulk2QueryJob aborts a Bulk API 2.0 query job
func (f *Force) AbortBulk2QueryJob(jobId string) (Bulk2QueryJobInfo, error) {
	url := fmt.Sprintf("%s/%s", f.bulk2QueryUrl(), jobId)
	body := fmt.Sprintf(`{"state": "%s"}`, Bulk2JobStateAborted)

	req := NewRequest("PATCH").
		AbsoluteUrl(url).
		WithContent(ContentTypeJson).
		WithBody(strings.NewReader(body)).
		ReadResponseBody()

	resp, err := f.ExecuteRequest(req)
	if err != nil {
		return Bulk2QueryJobInfo{}, err
	}

	var result Bulk2QueryJobInfo
	if err := internal.JsonUnmarshal(resp.ReadResponseBody, &result); err != nil {
		return Bulk2QueryJobInfo{}, err
	}
	return result, nil
}

// AbortBulk2QueryJobWithContext aborts a Bulk API 2.0 query job with context
func (f *Force) AbortBulk2QueryJobWithContext(ctx context.Context, jobId string) (Bulk2QueryJobInfo, error) {
	done := make(chan struct{})
	var result Bulk2QueryJobInfo
	var err error
	go func() {
		defer close(done)
		result, err = f.AbortBulk2QueryJob(jobId)
	}()
	select {
	case <-ctx.Done():
		return Bulk2QueryJobInfo{}, fmt.Errorf("AbortBulk2QueryJob canceled: %w", ctx.Err())
	case <-done:
		return result, err
	}
}

// DeleteBulk2QueryJob deletes a Bulk API 2.0 query job
func (f *Force) DeleteBulk2QueryJob(jobId string) error {
	url := fmt.Sprintf("%s/%s", f.bulk2QueryUrl(), jobId)

	req := NewRequest("DELETE").
		AbsoluteUrl(url).
		ReadResponseBody()

	_, err := f.ExecuteRequest(req)
	return err
}

// GetBulk2QueryJobs retrieves all Bulk API 2.0 query jobs
func (f *Force) GetBulk2QueryJobs() ([]Bulk2QueryJobInfo, error) {
	url := f.bulk2QueryUrl()

	req := NewRequest("GET").
		AbsoluteUrl(url).
		WithContent(ContentTypeJson).
		ReadResponseBody()

	resp, err := f.ExecuteRequest(req)
	if err != nil {
		return nil, err
	}

	var result Bulk2QueryJobList
	if err := internal.JsonUnmarshal(resp.ReadResponseBody, &result); err != nil {
		return nil, err
	}

	jobs := result.Records
	for !result.Done && result.NextRecordsUrl != "" {
		nextUrl := fmt.Sprintf("%s%s", f.Credentials.InstanceUrl, result.NextRecordsUrl)
		req := NewRequest("GET").
			AbsoluteUrl(nextUrl).
			WithContent(ContentTypeJson).
			ReadResponseBody()

		resp, err := f.ExecuteRequest(req)
		if err != nil {
			return nil, err
		}

		if err := internal.JsonUnmarshal(resp.ReadResponseBody, &result); err != nil {
			return nil, err
		}
		jobs = append(jobs, result.Records...)
	}

	return jobs, nil
}

// Bulk2JobStatusCallback is a function that receives job status updates
type Bulk2JobStatusCallback func(jobInfo any)

// WaitForBulk2IngestJob waits for a Bulk API 2.0 ingest job to complete
func (f *Force) WaitForBulk2IngestJob(jobId string, pollInterval time.Duration, callback Bulk2JobStatusCallback) (Bulk2IngestJobInfo, error) {
	for {
		jobInfo, err := f.GetBulk2IngestJobInfo(jobId)
		if err != nil {
			return Bulk2IngestJobInfo{}, err
		}
		if callback != nil {
			callback(jobInfo)
		}
		if jobInfo.IsTerminal() {
			return jobInfo, nil
		}
		time.Sleep(pollInterval)
	}
}

// WaitForBulk2IngestJobWithContext waits for a Bulk API 2.0 ingest job to complete with context
func (f *Force) WaitForBulk2IngestJobWithContext(ctx context.Context, jobId string, pollInterval time.Duration, callback Bulk2JobStatusCallback) (Bulk2IngestJobInfo, error) {
	for {
		select {
		case <-ctx.Done():
			return Bulk2IngestJobInfo{}, fmt.Errorf("WaitForBulk2IngestJob canceled: %w", ctx.Err())
		default:
		}

		jobInfo, err := f.GetBulk2IngestJobInfoWithContext(ctx, jobId)
		if err != nil {
			return Bulk2IngestJobInfo{}, err
		}
		if callback != nil {
			callback(jobInfo)
		}
		if jobInfo.IsTerminal() {
			return jobInfo, nil
		}

		select {
		case <-ctx.Done():
			return Bulk2IngestJobInfo{}, fmt.Errorf("WaitForBulk2IngestJob canceled: %w", ctx.Err())
		case <-time.After(pollInterval):
		}
	}
}

// WaitForBulk2QueryJob waits for a Bulk API 2.0 query job to complete
func (f *Force) WaitForBulk2QueryJob(jobId string, pollInterval time.Duration, callback Bulk2JobStatusCallback) (Bulk2QueryJobInfo, error) {
	for {
		jobInfo, err := f.GetBulk2QueryJobInfo(jobId)
		if err != nil {
			return Bulk2QueryJobInfo{}, err
		}
		if callback != nil {
			callback(jobInfo)
		}
		if jobInfo.IsTerminal() {
			return jobInfo, nil
		}
		time.Sleep(pollInterval)
	}
}

// WaitForBulk2QueryJobWithContext waits for a Bulk API 2.0 query job to complete with context
func (f *Force) WaitForBulk2QueryJobWithContext(ctx context.Context, jobId string, pollInterval time.Duration, callback Bulk2JobStatusCallback) (Bulk2QueryJobInfo, error) {
	for {
		select {
		case <-ctx.Done():
			return Bulk2QueryJobInfo{}, fmt.Errorf("WaitForBulk2QueryJob canceled: %w", ctx.Err())
		default:
		}

		jobInfo, err := f.GetBulk2QueryJobInfoWithContext(ctx, jobId)
		if err != nil {
			return Bulk2QueryJobInfo{}, err
		}
		if callback != nil {
			callback(jobInfo)
		}
		if jobInfo.IsTerminal() {
			return jobInfo, nil
		}

		select {
		case <-ctx.Done():
			return Bulk2QueryJobInfo{}, fmt.Errorf("WaitForBulk2QueryJob canceled: %w", ctx.Err())
		case <-time.After(pollInterval):
		}
	}
}

// DisplayBulk2IngestJobInfo displays information about a Bulk API 2.0 ingest job
func DisplayBulk2IngestJobInfo(jobInfo Bulk2IngestJobInfo, w io.Writer) {
	var msg = `
Id				%s
State 				%s
Operation			%s
Object 				%s
Api Version 			%.1f

Created By Id 			%s
Created Date 			%s
System Modstamp			%s
Content Type 			%s
Concurrency Mode 		%s

Number Records Processed 	%d
Number Records Failed 		%d
Retries 			%d

Total Processing Time 		%d
Api Active Processing Time 	%d
Apex Processing Time 		%d
`
	fmt.Fprintf(w, msg, jobInfo.Id, jobInfo.State, jobInfo.Operation, jobInfo.Object, jobInfo.ApiVersion,
		jobInfo.CreatedById, jobInfo.CreatedDate, jobInfo.SystemModstamp,
		jobInfo.ContentType, jobInfo.ConcurrencyMode,
		jobInfo.NumberRecordsProcessed, jobInfo.NumberRecordsFailed, jobInfo.Retries,
		jobInfo.TotalProcessingTime, jobInfo.ApiActiveProcessingTime, jobInfo.ApexProcessingTime)

	if jobInfo.ErrorMessage != "" {
		fmt.Fprintf(w, "Error Message:\t\t\t%s\n", jobInfo.ErrorMessage)
	}
}

// DisplayBulk2QueryJobInfo displays information about a Bulk API 2.0 query job
func DisplayBulk2QueryJobInfo(jobInfo Bulk2QueryJobInfo, w io.Writer) {
	var msg = `
Id				%s
State 				%s
Operation			%s
Object 				%s
Api Version 			%.1f

Created By Id 			%s
Created Date 			%s
System Modstamp			%s
Content Type 			%s
Concurrency Mode 		%s

Number Records Processed 	%d
Retries 			%d
Total Processing Time 		%d
`
	fmt.Fprintf(w, msg, jobInfo.Id, jobInfo.State, jobInfo.Operation, jobInfo.Object, jobInfo.ApiVersion,
		jobInfo.CreatedById, jobInfo.CreatedDate, jobInfo.SystemModstamp,
		jobInfo.ContentType, jobInfo.ConcurrencyMode,
		jobInfo.NumberRecordsProcessed, jobInfo.Retries, jobInfo.TotalProcessingTime)

	if jobInfo.ErrorMessage != "" {
		fmt.Fprintf(w, "Error Message:\t\t\t%s\n", jobInfo.ErrorMessage)
	}
}

// DisplayBulk2IngestJobList displays a list of Bulk API 2.0 ingest jobs
func DisplayBulk2IngestJobList(jobs []Bulk2IngestJobInfo, w io.Writer) {
	fmt.Fprintf(w, "%-18s %-12s %-15s %-15s %-12s %-10s\n", "ID", "State", "Operation", "Object", "Processed", "Failed")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 90))
	for _, job := range jobs {
		fmt.Fprintf(w, "%-18s %-12s %-15s %-15s %-12d %-10d\n",
			job.Id, job.State, job.Operation, job.Object, job.NumberRecordsProcessed, job.NumberRecordsFailed)
	}
}

// DisplayBulk2QueryJobList displays a list of Bulk API 2.0 query jobs
func DisplayBulk2QueryJobList(jobs []Bulk2QueryJobInfo, w io.Writer) {
	fmt.Fprintf(w, "%-18s %-12s %-15s %-15s %-12s\n", "ID", "State", "Operation", "Object", "Processed")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 75))
	for _, job := range jobs {
		fmt.Fprintf(w, "%-18s %-12s %-15s %-15s %-12d\n",
			job.Id, job.State, job.Operation, job.Object, job.NumberRecordsProcessed)
	}
}

// ParseBulk2Warnings parses warning headers from an HTTP response
func ParseBulk2Warnings(resp *http.Response) []string {
	var warnings []string
	for _, w := range resp.Header.Values("Warning") {
		warnings = append(warnings, w)
	}
	return warnings
}
