package lib

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ForceCLI/force/lib/internal"
)

type BatchResult []Result

type BatchResultChunk struct {
	HasCSVHeader bool
	Data         []byte
}

type BatchInfo struct {
	Id                     string `xml:"id" json:"id"`
	JobId                  string `xml:"jobId" json:"jobId"`
	State                  string `xml:"state" json:"state"`
	StateMessage           string `xml:"stateMessage" json:"stateMessage"`
	CreatedDate            string `xml:"createdDate" json:"createdDate"`
	SystemModstamp         string `xml:"systemModstamp" json:"systemModstamp"`
	NumberRecordsProcessed int    `xml:"numberRecordsProcessed" json:"numberRecordsProcessed"`
	NumberRecordsFailed    int    `xml:"numberRecordsFailed" json:"numberRecordsFailed"`
}

type JobInfo struct {
	XMLName                 xml.Name `xml:"http://www.force.com/2009/06/asyncapi/dataload jobInfo"`
	Id                      string   `xml:"id,omitempty"`
	Operation               string   `xml:"operation,omitempty"`
	Object                  string   `xml:"object,omitempty"`
	ExternalIdFieldName     string   `xml:"externalIdFieldName,omitempty"`
	CreatedById             string   `xml:"createdById,omitempty"`
	CreatedDate             string   `xml:"createdDate,omitempty"`
	SystemModStamp          string   `xml:"systemModstamp,omitempty"`
	State                   string   `xml:"state,omitempty"`
	ConcurrencyMode         string   `xml:"concurrencyMode,omitempty"`
	ContentType             string   `xml:"contentType,omitempty"`
	NumberBatchesQueued     int      `xml:"numberBatchesQueued,omitempty"`
	NumberBatchesInProgress int      `xml:"numberBatchesInProgress,omitempty"`
	NumberBatchesCompleted  int      `xml:"numberBatchesCompleted,omitempty"`
	NumberBatchesFailed     int      `xml:"numberBatchesFailed,omitempty"`
	NumberBatchesTotal      int      `xml:"numberBatchesTotal,omitempty"`
	NumberRecordsProcessed  int      `xml:"numberRecordsProcessed,omitempty"`
	NumberRetries           int      `xml:"numberRetries,omitempty"`
	ApiVersion              string   `xml:"apiVersion,omitempty"`
	NumberRecordsFailed     int      `xml:"numberRecordsFailed,omitempty"`
	TotalProcessingTime     int      `xml:"totalProcessingTime,omitempty"`
	ApiActiveProcessingTime int      `xml:"apiActiveProcessingTime,omitempty"`
	ApexProcessingTime      int      `xml:"apexProcessingTime,omitempty"`
}

func (ji JobInfo) JobContentType() (JobContentType, error) {
	switch ji.ContentType {
	case "JSON":
		return JobContentTypeJson, nil
	case "CSV":
		return JobContentTypeCsv, nil
	case "XML":
		return JobContentTypeXml, nil
	default:
		return "", fmt.Errorf("Invalid content type for bulk API: " + ji.ContentType)
	}
}

func (ji JobInfo) HttpContentType() (ContentType, error) {
	jct, err := ji.JobContentType()
	if err != nil {
		return "", err
	}
	return httpContentTypeForJobContentType[jct], nil
}

type JobContentType string

const (
	JobContentTypeCsv  JobContentType = "CSV"
	JobContentTypeXml  JobContentType = "XML"
	JobContentTypeJson JobContentType = "JSON"
)

var httpContentTypeForJobContentType = map[JobContentType]ContentType{
	JobContentTypeCsv:  ContentTypeCsv,
	JobContentTypeXml:  ContentTypeXml,
	JobContentTypeJson: ContentTypeJson,
}

var httpResponseUnmarshalerForJobContentType = map[JobContentType]internal.Unmarshaler{
	JobContentTypeCsv:  internal.XmlUnmarshal,
	JobContentTypeXml:  internal.XmlUnmarshal,
	JobContentTypeJson: internal.JsonUnmarshal,
}

var InvalidBulkObject = errors.New("Object Does Not Support Bulk API")

func (f *Force) CreateBulkJobWithContext(ctx context.Context, jobInfo JobInfo, requestOptions ...func(*http.Request)) (JobInfo, error) {
	done := make(chan struct{})
	var result JobInfo
	var err error
	go func() {
		defer close(done)
		result, err = f.CreateBulkJob(jobInfo, requestOptions...)
	}()
	select {
	case <-ctx.Done():
		return JobInfo{}, fmt.Errorf("CreateBulkJob canceled: %w", ctx.Err())
	case <-done:
		return result, err
	}
}

func (f *Force) CreateBulkJob(jobInfo JobInfo, requestOptions ...func(*http.Request)) (JobInfo, error) {
	xmlbody, err := internal.XmlMarshal(jobInfo)
	if err != nil {
		return JobInfo{}, fmt.Errorf("Could not create job request: %s", err.Error())
	}
	url := fmt.Sprintf("%s/services/async/%s/job", f.Credentials.InstanceUrl, apiVersionNumber)
	body, err := f.httpPostPatchWithRetry(url, string(xmlbody), ContentTypeXml, HttpMethodPost, requestOptions...)
	if err != nil {
		if fault, ok := err.(LoginFault); ok && fault.ExceptionCode == "InvalidEntity" {
			return JobInfo{}, InvalidBulkObject
		}
		return JobInfo{}, err
	}
	var result JobInfo
	if err := internal.XmlUnmarshal(body, &result); err != nil {
		return result, err
	}
	return result, nil
}

func (f *Force) CloseBulkJobWithContext(ctx context.Context, jobId string) (JobInfo, error) {
	done := make(chan struct{})
	var jobInfo JobInfo
	var err error
	go func() {
		defer close(done)
		jobInfo, err = f.CloseBulkJob(jobId)
	}()
	select {
	case <-ctx.Done():
		return JobInfo{}, fmt.Errorf("CloseBulkJob canceled: %w", ctx.Err())
	case <-done:
		return jobInfo, err
	}
}

func (f *Force) CloseBulkJob(jobId string) (JobInfo, error) {
	jobInfo := JobInfo{
		State: "Closed",
	}
	xmlbody, err := internal.XmlMarshal(jobInfo)
	if err != nil {
		return JobInfo{}, err
	}
	url := fmt.Sprintf("%s/services/async/%s/job/%s", f.Credentials.InstanceUrl, apiVersionNumber, jobId)
	body, err := f.httpPostPatchWithRetry(url, string(xmlbody), ContentTypeXml, HttpMethodPost)
	if err != nil {
		return JobInfo{}, err
	}
	var result JobInfo
	if err := internal.XmlUnmarshal(body, &result); err != nil {
		return result, err
	}
	return result, nil
}

func (f *Force) GetBulkJobs() ([]JobInfo, error) {
	url := fmt.Sprintf("%s/services/async/%s/jobs", f.Credentials.InstanceUrl, apiVersionNumber)
	resp, err := f.httpGetBulk(url)
	if err != nil {
		return nil, err
	}
	var result []JobInfo
	if err := internal.XmlUnmarshal(resp.ReadResponseBody, &result); err != nil {
		return nil, err
	}
	return result, err
}

func (f *Force) httpGetBulk(url string) (*Response, error) {
	req := NewRequest("GET").AbsoluteUrl(url).WithContent(ContentTypeXml).ReadResponseBody()
	return f.ExecuteRequest(req)
}

func (f *Force) BulkQueryWithContext(ctx context.Context, soql string, jobId string, contentType string, requestOptions ...func(*http.Request)) (BatchInfo, error) {
	done := make(chan struct{})
	var batchInfo BatchInfo
	var err error
	go func() {
		defer close(done)
		batchInfo, err = f.BulkQuery(soql, jobId, contentType, requestOptions...)
	}()
	select {
	case <-ctx.Done():
		return BatchInfo{}, fmt.Errorf("BulkQuery canceled: %w", ctx.Err())
	case <-done:
		return batchInfo, err
	}
}

func (f *Force) BulkQuery(soql string, jobId string, contentType string, requestOptions ...func(*http.Request)) (BatchInfo, error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch", f.Credentials.InstanceUrl, apiVersionNumber, jobId)

	jct, err := JobInfo{ContentType: contentType}.JobContentType()
	if err != nil {
		return BatchInfo{}, err
	}
	httpCt := httpContentTypeForJobContentType[jct]
	unmarshal := httpResponseUnmarshalerForJobContentType[jct]

	body, err := f.httpPostPatchWithRetry(url, soql, httpCt, HttpMethodPost, requestOptions...)
	if err != nil {
		return BatchInfo{}, err
	}
	var result BatchInfo
	if err := unmarshal(body, &result); err != nil {
		return BatchInfo{}, err
	}
	return result, nil
}

func (f *Force) AddBatchToJob(content string, job JobInfo) (BatchInfo, error) {
	jct, err := job.JobContentType()
	if err != nil {
		return BatchInfo{}, err
	}
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch", f.Credentials.InstanceUrl, apiVersionNumber, job.Id)
	body, err := f.httpPostPatchWithRetry(url, content, httpContentTypeForJobContentType[jct], HttpMethodPost)
	if err != nil {
		return BatchInfo{}, err
	}
	unmarshal := httpResponseUnmarshalerForJobContentType[jct]
	var result BatchInfo
	if err := unmarshal(body, &result); err != nil {
		return BatchInfo{}, err
	}
	return result, nil
}

func (f *Force) GetBatchInfoWithContext(ctx context.Context, jobId string, batchId string) (BatchInfo, error) {
	done := make(chan struct{})
	var batchInfo BatchInfo
	var err error
	go func() {
		defer close(done)
		batchInfo, err = f.GetBatchInfo(jobId, batchId)
	}()
	select {
	case <-ctx.Done():
		return BatchInfo{}, fmt.Errorf("GetBatchInfo canceled: %w", ctx.Err())
	case <-done:
		return batchInfo, err
	}
}

func (f *Force) GetBatchInfo(jobId string, batchId string) (BatchInfo, error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s", f.Credentials.InstanceUrl, apiVersionNumber, jobId, batchId)

	resp, err := f.httpGetBulk(url)
	if err != nil {
		return BatchInfo{}, err
	}

	var unmarshal internal.Unmarshaler
	if resp.ContentType == ContentTypeJson {
		unmarshal = internal.JsonUnmarshal
	} else {
		unmarshal = internal.XmlUnmarshal
	}
	var result BatchInfo
	if err := unmarshal(resp.ReadResponseBody, &result); err != nil {
		return BatchInfo{}, err
	}
	return result, nil
}

func (f *Force) GetBatchesWithContext(ctx context.Context, jobId string) (result []BatchInfo, err error) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		result, err = f.GetBatches(jobId)
	}()
	select {
	case <-ctx.Done():
		return result, fmt.Errorf("GetBatches canceled: %w", ctx.Err())
	case <-done:
		return result, err
	}
}

func (f *Force) GetBatches(jobId string) (result []BatchInfo, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch", f.Credentials.InstanceUrl, apiVersionNumber, jobId)
	resp, err := f.httpGetBulk(url)
	if err != nil {
		return nil, err
	}

	var batchInfoList struct {
		BatchInfos []BatchInfo `xml:"batchInfo" json:"batchInfo"`
	}

	var unmarshal internal.Unmarshaler
	if resp.ContentType == ContentTypeJson {
		unmarshal = internal.JsonUnmarshal
	} else {
		unmarshal = internal.XmlUnmarshal
	}
	if err := unmarshal(resp.ReadResponseBody, &batchInfoList); err != nil {
		return nil, err
	}
	return batchInfoList.BatchInfos, nil
}

func (f *Force) GetJobInfoWithContext(ctx context.Context, jobId string) (JobInfo, error) {
	done := make(chan struct{})
	var jobInfo JobInfo
	var err error
	go func() {
		defer close(done)
		jobInfo, err = f.GetJobInfo(jobId)
	}()
	select {
	case <-ctx.Done():
		return JobInfo{}, fmt.Errorf("GetJobInfo canceled: %w", ctx.Err())
	case <-done:
		return jobInfo, err
	}
}

func (f *Force) GetJobInfo(jobId string) (JobInfo, error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s", f.Credentials.InstanceUrl, apiVersionNumber, jobId)
	resp, err := f.httpGetBulk(url)
	if err != nil {
		return JobInfo{}, err
	}
	var result JobInfo
	if err := internal.XmlUnmarshal(resp.ReadResponseBody, &result); err != nil {
		return JobInfo{}, err
	}
	return result, nil
}

func (f *Force) RetrieveBulkQueryResultList(job JobInfo, batchId string) ([]byte, error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s/result", f.Credentials.InstanceUrl, apiVersionNumber, job.Id, batchId)
	ct, err := job.HttpContentType()
	if err != nil {
		return nil, err
	}
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url).WithContent(ct))
	return body, err
}

func (f *Force) RetrieveBulkQueryWithContext(ctx context.Context, jobId string, batchId string) ([]byte, error) {
	done := make(chan struct{})
	var result []byte
	var err error
	go func() {
		defer close(done)
		result, err = f.RetrieveBulkQuery(jobId, batchId)
	}()
	select {
	case <-ctx.Done():
		return result, fmt.Errorf("RetrieveBulkQuery canceled: %w", ctx.Err())
	case <-done:
		return result, err
	}
}

func (f *Force) RetrieveBulkQuery(jobId string, batchId string) ([]byte, error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s/result", f.Credentials.InstanceUrl, apiVersionNumber, jobId, batchId)
	resp, err := f.httpGetBulk(url)
	if err != nil {
		return nil, err
	}
	return resp.ReadResponseBody, nil
}

func (f *Force) RetrieveBulkRequest(jobId string, batchId string) ([]byte, error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s/request", f.Credentials.InstanceUrl, apiVersionNumber, jobId, batchId)
	resp, err := f.httpGetBulk(url)
	if err != nil {
		return nil, err
	}
	return resp.ReadResponseBody, nil
}

func (f *Force) RetrieveBulkQueryResults(jobId string, batchId string, resultId string) ([]byte, error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s/result/%s", f.Credentials.InstanceUrl, apiVersionNumber, jobId, batchId, resultId)
	resp, err := f.httpGetBulk(url)
	if err != nil {
		return nil, err
	}
	return resp.ReadResponseBody, nil
}

func (f *Force) RetrieveBulkJobQueryResults(job JobInfo, batchId string, resultId string) ([]byte, error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s/result/%s", f.Credentials.InstanceUrl, apiVersionNumber, job.Id, batchId, resultId)
	ct, err := job.HttpContentType()
	if err != nil {
		return nil, err
	}
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url).WithContent(ct))
	return body, err
}

func (f *Force) RetrieveBulkJobQueryResultsWithCallbackWithContext(ctx context.Context, job JobInfo, batchId string, resultId string, callback HttpCallback) error {
	done := make(chan struct{})
	var err error
	go func() {
		defer close(done)
		err = f.RetrieveBulkJobQueryResultsWithCallback(job, batchId, resultId, callback)
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("RetrieveBulkJobQueryResultsWithCallback canceled: %w", ctx.Err())
	case <-done:
		return err
	}
}

func (f *Force) RetrieveBulkJobQueryResultsWithCallback(job JobInfo, batchId string, resultId string, callback HttpCallback) error {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s/result/%s", f.Credentials.InstanceUrl, apiVersionNumber, job.Id, batchId, resultId)
	ct, err := job.HttpContentType()
	if err != nil {
		return err
	}
	req := NewRequest("GET").AbsoluteUrl(url).WithContent(ct).WithResponseCallback(callback)
	_, err = f.ExecuteRequest(req)
	return err
}

// Deprecated: Use RetrieveBulkJobQueryResultsWithCallback
func (f *Force) RetrieveBulkJobQueryResultsAndSend(job JobInfo, batchId string, resultId string, results chan<- BatchResultChunk) error {
	return f.RetrieveBulkJobQueryResultsWithCallback(job, batchId, resultId, NewBatchResultChannelHttpCallback(results, 0))
}

func (f *Force) RetrieveBulkBatchResults(jobId string, batchId string) (BatchResult, error) {
	var result BatchResult
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s/result", f.Credentials.InstanceUrl, apiVersionNumber, jobId, batchId)
	resp, err := f.httpGetBulk(url)
	if err != nil {
		return result, err
	}
	var unmarshal internal.Unmarshaler
	if resp.ContentType == ContentTypeJson {
		unmarshal = internal.JsonUnmarshal
	} else {
		unmarshal = internal.XmlUnmarshal
	}
	if err := unmarshal(resp.ReadResponseBody, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// NewBatchResultChannelHttpCallback returns a new reporter that will send chunks of a read body into
// the results channel. You can control the size of the chunks via bufferSize (defaults to 50mb).
func NewBatchResultChannelHttpCallback(results chan<- BatchResultChunk, bufferSize int) HttpCallback {
	if bufferSize <= 1 {
		bufferSize = 50 * 1024 * 1024
	}
	return (&batchResultChannelHttpCallback{results: results, bufferSize: bufferSize}).Report
}

type batchResultChannelHttpCallback struct {
	bufferSize int
	results    chan<- BatchResultChunk
}

func (c *batchResultChannelHttpCallback) Report(res *http.Response) error {
	defer res.Body.Close()
	buf := make([]byte, c.bufferSize)
	contentType := res.Header.Get("Content-Type")
	firstChunk := true
	isCSV := strings.Contains(contentType, "text/csv")
	for {
		n, err := io.ReadFull(res.Body, buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			c.results <- BatchResultChunk{
				HasCSVHeader: firstChunk && isCSV,
				Data:         data,
			}
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		} else if err != nil {
			return err
		}
		firstChunk = false
	}
}
