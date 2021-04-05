package lib

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type BatchResult struct {
	Results []Result
}

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

var InvalidBulkObject = errors.New("Object Does Not Support Bulk API")

func (f *Force) CreateBulkJob(jobInfo JobInfo, requestOptions ...func(*http.Request)) (result JobInfo, err error) {
	xmlbody, err := xml.Marshal(jobInfo)
	if err != nil {
		err = fmt.Errorf("Could not create job request: %s", err.Error())
		return
	}
	url := fmt.Sprintf("%s/services/async/%s/job", f.Credentials.InstanceUrl, apiVersionNumber)
	body, err := f.httpPostXML(url, string(xmlbody), requestOptions...)
	xml.Unmarshal(body, &result)
	if len(result.Id) == 0 {
		var fault LoginFault
		xml.Unmarshal(body, &fault)
		if fault.ExceptionCode == "InvalidEntity" {
			err = InvalidBulkObject
		} else {
			err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
		}
	}
	return
}

func (f *Force) CloseBulkJob(jobId string) (result JobInfo, err error) {
	jobInfo := JobInfo{
		State: "Closed",
	}
	xmlbody, _ := xml.Marshal(jobInfo)
	url := fmt.Sprintf("%s/services/async/%s/job/%s", f.Credentials.InstanceUrl, apiVersionNumber, jobId)
	body, err := f.httpPostXML(url, string(xmlbody))
	xml.Unmarshal(body, &result)
	if len(result.Id) == 0 {
		var fault LoginFault
		xml.Unmarshal(body, &fault)
		err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
	}
	return
}

func (f *Force) GetBulkJobs() ([]JobInfo, error) {
	url := fmt.Sprintf("%s/services/async/%s/jobs", f.Credentials.InstanceUrl, apiVersionNumber)
	resp, err := f.httpGetBulk(url)
	if err != nil {
		return nil, err
	}
	var result []JobInfo
	xml.Unmarshal(resp.ReadResponseBody, &result)
	if len(result[0].Id) == 0 {
		var fault LoginFault
		xml.Unmarshal(resp.ReadResponseBody, &fault)
		err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
	}
	return result, err
}

func (f *Force) httpGetBulk(url string) (*Response, error) {
	req := NewRequest("GET").AbsoluteUrl(url).WithContent(ContentTypeXml).ReadResponseBody()
	return f.ExecuteRequest(req)
}

func (f *Force) BulkQuery(soql string, jobId string, contentType string, requestOptions ...func(*http.Request)) (BatchInfo, error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch", f.Credentials.InstanceUrl, apiVersionNumber, jobId)
	var body []byte

	jct, err := JobInfo{ContentType: contentType}.JobContentType()
	if err != nil {
		return BatchInfo{}, err
	}

	var result BatchInfo
	switch jct {
	case JobContentTypeCsv:
		body, err = f.httpPostCSV(url, soql, requestOptions...)
		xml.Unmarshal(body, &result)
	case JobContentTypeJson:
		body, err = f.httpPostJSON(url, soql, requestOptions...)
		json.Unmarshal(body, &result)
	case JobContentTypeXml:
		body, err = f.httpPostXML(url, soql, requestOptions...)
		xml.Unmarshal(body, &result)
	default:
		panic("unreachable")
	}
	if len(result.Id) == 0 {
		var fault LoginFault
		xml.Unmarshal(body, &fault)
		return BatchInfo{}, errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
	}
	return result, nil
}

func (f *Force) addCSVBatchToJob(content string, job JobInfo) (result BatchInfo, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch", f.Credentials.InstanceUrl, apiVersionNumber, job.Id)
	body, err := f.httpPostCSV(url, content)
	if err != nil {
		err = fmt.Errorf("Failed to add batch: " + err.Error())
		return
	}
	err = xml.Unmarshal(body, &result)
	if len(result.Id) == 0 {
		var fault LoginFault
		xml.Unmarshal(body, &fault)
		err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
	}
	return
}

func (f *Force) addXMLBatchToJob(content string, job JobInfo) (result BatchInfo, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch", f.Credentials.InstanceUrl, apiVersionNumber, job.Id)
	body, err := f.httpPostXML(url, content)
	if err != nil {
		err = fmt.Errorf("Failed to add batch: " + err.Error())
		return
	}
	err = xml.Unmarshal(body, &result)
	if len(result.Id) == 0 {
		var fault LoginFault
		xml.Unmarshal(body, &fault)
		err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
	}
	return
}

func (f *Force) addJSONBatchToJob(content string, job JobInfo) (result BatchInfo, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch", f.Credentials.InstanceUrl, apiVersionNumber, job.Id)
	body, err := f.httpPostJSON(url, content)
	if err != nil {
		err = fmt.Errorf("Failed to add batch: " + err.Error())
		return
	}
	err = json.Unmarshal(body, &result)
	return
}

func (f *Force) AddBatchToJob(content string, job JobInfo) (BatchInfo, error) {
	jct, err := job.JobContentType()
	if err != nil {
		return BatchInfo{}, err
	}
	switch jct {
	case JobContentTypeCsv:
		return f.addCSVBatchToJob(content, job)
	case JobContentTypeJson:
		return f.addJSONBatchToJob(content, job)
	case JobContentTypeXml:
		return f.addXMLBatchToJob(content, job)
	default:
		panic("unreachable")
	}
}

func (f *Force) GetBatchInfo(jobId string, batchId string) (BatchInfo, error) {
	var result BatchInfo
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s", f.Credentials.InstanceUrl, apiVersionNumber, jobId, batchId)

	resp, err := f.httpGetBulk(url)
	if err != nil {
		return result, err
	}

	if resp.ContentType == ContentTypeJson {
		json.Unmarshal(resp.ReadResponseBody, &result)
		if len(result.Id) == 0 {
			var fault LoginFault
			json.Unmarshal(resp.ReadResponseBody, &fault)
			err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
		}
	} else {
		xml.Unmarshal(resp.ReadResponseBody, &result)
		if len(result.Id) == 0 {
			var fault LoginFault
			xml.Unmarshal(resp.ReadResponseBody, &fault)
			err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
		}
	}

	return result, err
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

	if resp.ContentType == ContentTypeJson {
		json.Unmarshal(resp.ReadResponseBody, &batchInfoList)
		result = batchInfoList.BatchInfos
	} else {
		xml.Unmarshal(resp.ReadResponseBody, &batchInfoList)
		result = batchInfoList.BatchInfos
		if len(result) == 0 {
			var fault LoginFault
			xml.Unmarshal(resp.ReadResponseBody, &fault)
			err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
		}
	}
	return
}

func (f *Force) GetJobInfo(jobId string) (JobInfo, error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s", f.Credentials.InstanceUrl, apiVersionNumber, jobId)
	resp, err := f.httpGetBulk(url)
	if err != nil {
		return JobInfo{}, err
	}
	var result JobInfo
	xml.Unmarshal(resp.ReadResponseBody, &result)
	if len(result.Id) == 0 {
		var fault LoginFault
		xml.Unmarshal(resp.ReadResponseBody, &fault)
		err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
	}
	return result, err
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

func (f *Force) RetrieveBulkQuery(jobId string, batchId string) ([]byte, error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s/result", f.Credentials.InstanceUrl, apiVersionNumber, jobId, batchId)
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
	err := errors.New("this method was never working right, please report an issue in GitHub if you were using it")
	return BatchResult{}, err
}

// NewChannelChunkBatchResultsReporter returns a new reporter that will send chunks of a read body into
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
