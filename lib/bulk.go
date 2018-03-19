package lib

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
)

type BatchResult struct {
	Results []Result
}

type BatchInfo struct {
	Id                     string `xml:"id" json:"id"`
	JobId                  string `xml:"jobId" json:"jobId"`
	State                  string `xml:"state" json:"state"`
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

var InvalidBulkObject = errors.New("Object Does Not Support Bulk API")

func (f *Force) CreateBulkJob(jobInfo JobInfo) (result JobInfo, err error) {
	xmlbody, err := xml.Marshal(jobInfo)
	if err != nil {
		err = fmt.Errorf("Could not create job request: %s", err.Error())
		return
	}
	url := fmt.Sprintf("%s/services/async/%s/job", f.Credentials.InstanceUrl, apiVersionNumber)
	body, err := f.httpPostXML(url, string(xmlbody))
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

func (f *Force) GetBulkJobs() (result []JobInfo, err error) {
	url := fmt.Sprintf("%s/services/async/%s/jobs", f.Credentials.InstanceUrl, apiVersionNumber)
	body, err := f.httpGetBulk(url)
	xml.Unmarshal(body, &result)
	if len(result[0].Id) == 0 {
		var fault LoginFault
		xml.Unmarshal(body, &fault)
		err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
	}
	return
}

func (f *Force) BulkQuery(soql string, jobId string, contettype string) (result BatchInfo, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch", f.Credentials.InstanceUrl, apiVersionNumber, jobId)
	var body []byte

	if contettype == "CSV" {
		body, err = f.httpPostCSV(url, soql)
		xml.Unmarshal(body, &result)
	} else if contettype == "JSON" {
		body, err = f.httpPostJSON(url, soql)
		json.Unmarshal(body, &result)
	} else {
		body, err = f.httpPostXML(url, soql)
		xml.Unmarshal(body, &result)
	}
	if len(result.Id) == 0 {
		var fault LoginFault
		xml.Unmarshal(body, &fault)
		err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
	}
	return
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

func (f *Force) AddBatchToJob(content string, job JobInfo) (result BatchInfo, err error) {
	switch job.ContentType {
	case "CSV":
		return f.addCSVBatchToJob(content, job)
	case "JSON":
		return f.addJSONBatchToJob(content, job)
	case "XML":
		return f.addXMLBatchToJob(content, job)
	default:
		err = fmt.Errorf("Invalid content type for bulk API: " + job.ContentType)
	}
	return
}

func (f *Force) GetBatchInfo(jobId string, batchId string) (result BatchInfo, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s", f.Credentials.InstanceUrl, apiVersionNumber, jobId, batchId)
	body, err := f.httpGetBulk(url)
	xml.Unmarshal(body, &result)
	if len(result.Id) == 0 {
		var fault LoginFault
		xml.Unmarshal(body, &fault)
		err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
	}
	return
}

func (f *Force) GetBatches(jobId string) (result []BatchInfo, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch", f.Credentials.InstanceUrl, apiVersionNumber, jobId)
	body, err := f.httpGetBulk(url)

	var batchInfoList struct {
		BatchInfos []BatchInfo `xml:"batchInfo"`
	}

	xml.Unmarshal(body, &batchInfoList)
	result = batchInfoList.BatchInfos
	if len(result) == 0 {
		var fault LoginFault
		xml.Unmarshal(body, &fault)
		err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
	}
	return
}

func (f *Force) GetJobInfo(jobId string) (result JobInfo, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s", f.Credentials.InstanceUrl, apiVersionNumber, jobId)
	body, err := f.httpGetBulk(url)
	xml.Unmarshal(body, &result)
	if len(result.Id) == 0 {
		var fault LoginFault
		xml.Unmarshal(body, &fault)
		err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
	}
	return
}

func (f *Force) retrieveBulkResult(url string, contentType string) (result []byte, err error) {
	switch contentType {
	case "JSON":
		return f.httpGetBulkJSON(url)
	case "CSV":
		fallthrough
	case "XML":
		return f.httpGetBulk(url)
	default:
		err = fmt.Errorf("Invalid content type for bulk API: " + contentType)
	}
	return nil, err
}

func (f *Force) RetrieveBulkQueryResultList(job JobInfo, batchId string) ([]byte, error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s/result", f.Credentials.InstanceUrl, apiVersionNumber, job.Id, batchId)
	return f.retrieveBulkResult(url, job.ContentType)
}

func (f *Force) RetrieveBulkQuery(jobId string, batchId string) (result []byte, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s/result", f.Credentials.InstanceUrl, apiVersionNumber, jobId, batchId)
	result, err = f.httpGetBulk(url)
	return
}

func (f *Force) RetrieveBulkQueryResults(jobId string, batchId string, resultId string) (result []byte, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s/result/%s", f.Credentials.InstanceUrl, apiVersionNumber, jobId, batchId, resultId)
	result, err = f.httpGetBulk(url)
	return
}

func (f *Force) RetrieveBulkJobQueryResults(job JobInfo, batchId string, resultId string) ([]byte, error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s/result/%s", f.Credentials.InstanceUrl, apiVersionNumber, job.Id, batchId, resultId)
	return f.retrieveBulkResult(url, job.ContentType)
}

func (f *Force) RetrieveBulkBatchResults(jobId string, batchId string) (results BatchResult, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch/%s/result", f.Credentials.InstanceUrl, apiVersionNumber, jobId, batchId)
	result, err := f.httpGetBulk(url)
	if len(result) == 0 {
		var fault LoginFault
		xml.Unmarshal(result, &fault)
		err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
	}
	//	sreader = Reader.NewReader(result);
	return
}
