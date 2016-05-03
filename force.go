package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
)

const (
	ProductionClientId = "3MVG9JZ_r.QzrS7gAjO9uCs2VkO.hLOCrAG2XC8vlkhr652yEh8Y5VMiNsDzmCik.eryhf1C902FFULbk1m.i"
	PrereleaseClientId = "3MVG9lKcPoNINVBIRgC7lsz5tIhlg0mtoEqkA9ZjDAwEMbBy43gsnfkzzdTdhFLeNnWS8M4bnRnVv1Qj0k9MD"
	Mobile1ClientId    = "3MVG9Iu66FKeHhIPqCB9VWfYPxjfcb5Ube.v5L81BLhnJtDYVP2nkA.mDPwfm5FTLbvL6aMftfi8w0rL7Dv7f"
	RedirectUri        = "https://force-cli.herokuapp.com/auth/callback"
	RedirectUriStaging = "https://force-clistaging.herokuapp.com/auth/callback"
)

var CustomEndpoint = ``

const (
	EndpointProduction = iota
	EndpointTest       = iota
	EndpointPrerelease = iota
	EndpointMobile1    = iota
	EndpointCustom     = iota
)

/*const (
	apiVersion       = "v34.0"
	apiVersionNumber = "34.0"
)*/

type Force struct {
	Credentials ForceCredentials
	Metadata    *ForceMetadata
	Partner     *ForcePartner
}

type ForceCredentials struct {
	AccessToken   string
	Id            string
	UserId        string
	InstanceUrl   string
	IssuedAt      string
	Scope         string
	IsCustomEP    bool
	Namespace     string
	ApiVersion    string
	ForceEndpoint ForceEndpoint
}

type LoginFault struct {
	ExceptionCode    string `xml:"exceptionCode"`
	ExceptionMessage string `xml:"exceptionMessage"`
}

type SoapFault struct {
	FaultCode   string     `xml:"Body>Fault>faultcode"`
	FaultString string     `xml:"Body>Fault>faultstring"`
	Detail      LoginFault `xml:"Body>Fault>detail>LoginFault"`
}

type GenericForceError struct {
	Error_Description string
	Error             string
}

type ForceError struct {
	Message   string
	ErrorCode string
}

type ForceEndpoint int

type ForceRecord map[string]interface{}

type ForceSobject map[string]interface{}

type ForceCreateRecordResult struct {
	Errors  []string
	Id      string
	Success bool
}

type ForceLimits map[string]ForceLimit

type ForceLimit struct {
	Name      string
	Remaining int64
	Max       int64
}

type ForcePasswordStatusResult struct {
	IsExpired bool
}

type ForcePasswordResetResult struct {
	NewPassword string
}

type ForceQueryResult struct {
	Done           bool
	Records        []ForceRecord
	TotalSize      int
	NextRecordsUrl string
}

type ForceSobjectsResult struct {
	Encoding     string
	MaxBatchSize int
	Sobjects     []ForceSobject
}

type Result struct {
	Id      string
	Success bool
	Created bool
	Message string
}

type BatchResult struct {
	Results []Result
}

type BatchInfo struct {
	Id                     string `xml:"id"`
	JobId                  string `xml:"jobId"`
	State                  string `xml:"state"`
	CreatedDate            string `xml:"createdDate"`
	SystemModstamp         string `xml:"systemModstamp"`
	NumberRecordsProcessed int    `xml:"numberRecordsProcessed"`
}

type JobInfo struct {
	Id                      string `xml:"id"`
	Operation               string `xml:"operation"`
	Object                  string `xml:"object"`
	CreatedById             string `xml:"createdById"`
	CreatedDate             string `xml:"createdDate"`
	SystemModStamp          string `xml:"systemModstamp"`
	State                   string `xml:"state"`
	ContentType             string `xml:"contentType"`
	ConcurrencyMode         string `xml:"concurrencyMode"`
	NumberBatchesQueued     int    `xml:"numberBatchesQueued"`
	NumberBatchesInProgress int    `xml:"numberBatchesInProgress"`
	NumberBatchesCompleted  int    `xml:"numberBatchesCompleted"`
	NumberBatchesFailed     int    `xml:"numberBatchesFailed"`
	NumberBatchesTotal      int    `xml:"numberBatchesTotal"`
	NumberRecordsProcessed  int    `xml:"numberRecordsProcessed"`
	NumberRetries           int    `xml:"numberRetries"`
	ApiVersion              string `xml:"apiVersion"`
	NumberRecordsFailed     int    `xml:"numberRecordsFailed"`
	TotalProcessingTime     int    `xml:"totalProcessingTime"`
	ApiActiveProcessingTime int    `xml:"apiActiveProcessingTime"`
	ApexProcessingTime      int    `xml:"apexProcessingTime"`
}

type AuraDefinitionBundleResult struct {
	Done           bool
	Records        []ForceRecord
	TotalSize      int
	QueryLocator   string
	Size           int
	EntityTypeName string
	NextRecordsUrl string
}

type AuraDefinitionBundle struct {
	Id               string
	IsDeleted        bool
	DeveloperName    string
	Language         string
	MasterLabel      string
	NamespacePrefix  string
	CreatedDate      string
	CreatedById      string
	LastModifiedDate string
	LastModifiedById string
	SystemModstamp   string
	ApiVersion       int
	Description      string
}

type AuraDefinition struct {
	Id                     string
	IsDeleted              bool
	CreatedDate            string
	CreatedById            string
	LastModifiedDate       string
	LastModifiedById       string
	SystemModstamp         string
	AuraDefinitionBundleId string
	DefType                string
	Format                 string
	Source                 string
}

type ComponentFile struct {
	FileName    string
	ComponentId string
}

type BundleManifest struct {
	Name  string
	Id    string
	Files []ComponentFile
}

func NewForce(creds ForceCredentials) (force *Force) {
	force = new(Force)
	force.Credentials = creds
	force.Metadata = NewForceMetadata(force)
	force.Partner = NewForcePartner(force)
	return
}

func ForceSoapLogin(endpoint ForceEndpoint, username string, password string) (creds ForceCredentials, err error) {
	var surl string
	version := strings.Split(apiVersion, "v")[1]
	switch endpoint {
	case EndpointProduction:
		surl = fmt.Sprintf("https://login.salesforce.com/services/Soap/u/%s", version)
	case EndpointTest:
		surl = fmt.Sprintf("https://test.salesforce.com/services/Soap/u/%s", version)
	case EndpointPrerelease:
		surl = fmt.Sprintf("https://prerelna1.pre.salesforce.com/services/Soap/u/%s", version)
	case EndpointMobile1:
		surl = fmt.Sprintf("https://mobile1.t.salesforce.com/services/Soap/u/%s", version)
	case EndpointCustom:
		surl = fmt.Sprintf("%s/services/Soap/u/%s", CustomEndpoint, version)
	default:
		ErrorAndExit("Unable to login with SOAP. Unknown endpoint type")
	}

	soap := NewSoap(surl, "", "")
	response, err := soap.ExecuteLogin(username, password)
	var result struct {
		SessionId    string `xml:"Body>loginResponse>result>sessionId"`
		Id           string `xml:"Body>loginResponse>result>userId"`
		Instance_url string `xml:"Body>loginResponse>result>serverUrl"`
	}
	var fault SoapFault
	if err = xml.Unmarshal(response, &fault); fault.Detail.ExceptionMessage != "" {
		ErrorAndExit(fault.Detail.ExceptionCode + ": " + fault.Detail.ExceptionMessage)
	}
	if err = xml.Unmarshal(response, &result); err != nil {
		return
	}
	orgid := strings.Split(result.SessionId, "!")[0]
	u, err := url.Parse(result.Instance_url)
	if err != nil {
		return
	}
	instanceUrl := u.Scheme + "://" + u.Host
	identity := u.Scheme + "://" + u.Host + "/id/" + orgid + "/" + result.Id
	creds = ForceCredentials{result.SessionId, identity, result.Id, instanceUrl, "", "", endpoint == EndpointCustom, "", apiVersionNumber, endpoint}

	f := NewForce(creds)
	url := "https://force-cli"
	if version == "dev" {
		url = fmt.Sprintf("%sstaging.herokuapp.com/auth/soaplogin/?id=%s&access_token=%s&instance_url=%s", url, creds.Id, creds.AccessToken, creds.InstanceUrl)
	} else {
		url = fmt.Sprintf("https://force-cli.herokuapp.com/auth/soaplogin/?id=%s&access_token=%s&instance_url=%s", creds.Id, creds.AccessToken, creds.InstanceUrl)
	}

	body, err := f.httpGet(url)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("Save login was ", string(body))
	return
}

func ForceLogin(endpoint ForceEndpoint) (creds ForceCredentials, err error) {
	ch := make(chan ForceCredentials)
	port, err := startLocalHttpServer(ch)
	var url string

	Redir := RedirectUri
	if Version == "dev" {
		Redir = RedirectUriStaging
	}

	switch endpoint {
	case EndpointProduction:
		url = fmt.Sprintf("https://login.salesforce.com/services/oauth2/authorize?response_type=token&client_id=%s&redirect_uri=%s&state=%d&prompt=login", ProductionClientId, Redir, port)
	case EndpointTest:
		url = fmt.Sprintf("https://test.salesforce.com/services/oauth2/authorize?response_type=token&client_id=%s&redirect_uri=%s&state=%d&prompt=login", ProductionClientId, Redir, port)
	case EndpointPrerelease:
		url = fmt.Sprintf("https://prerellogin.pre.salesforce.com/services/oauth2/authorize?response_type=token&client_id=%s&redirect_uri=%s&state=%d&prompt=login", PrereleaseClientId, Redir, port)
	case EndpointMobile1:
		url = fmt.Sprintf("https://EndpointMobile1.t.salesforce.com/services/oauth2/authorize?response_type=token&client_id=%s&redirect_uri=%s&state=%d&prompt=login", Mobile1ClientId, Redir, port)
	case EndpointCustom:
		url = fmt.Sprintf("%s/services/oauth2/authorize?response_type=token&client_id=%s&redirect_uri=%s&state=%d&prompt=login", CustomEndpoint, ProductionClientId, Redir, port)
	default:
		ErrorAndExit("Unable to login with OAuth. Unknown endpoint type")
	}

	err = Open(url)
	creds = <-ch
	creds.ForceEndpoint = endpoint
	return
}

func (f *Force) GetCodeCoverage(classId string, className string) (err error) {
	url := fmt.Sprintf("%s/services/data/%s/query/?q=Select+Id+From+ApexClass+Where+Name+=+'%s'", f.Credentials.InstanceUrl, apiVersion, className)

	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	var result ForceQueryResult
	json.Unmarshal(body, &result)

	classId = result.Records[0]["Id"].(string)
	url = fmt.Sprintf("%s/services/data/%s/tooling/query/?q=Select+Coverage,+NumLinesCovered,+NumLinesUncovered,+ApexTestClassId,+ApexClassorTriggerId+From+ApexCodeCoverage+Where+ApexClassorTriggerId='%s'", f.Credentials.InstanceUrl, apiVersion, classId)

	body, err = f.httpGet(url)
	if err != nil {
		return
	}

	//var result ForceSobjectsResult
	json.Unmarshal(body, &result)
	fmt.Printf("\n%d lines covered\n%d lines not covered\n", int(result.Records[0]["NumLinesCovered"].(float64)), int(result.Records[0]["NumLinesUncovered"].(float64)))
	return
}

func (f *Force) DeleteDataPipeline(id string) (err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/DataPipeline/%s", f.Credentials.InstanceUrl, apiVersion, id)
	_, err = f.httpDelete(url)
	return
}

func (f *Force) UpdateDataPipeline(id string, masterLabel string, scriptContent string) (err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/DataPipeline/%s", f.Credentials.InstanceUrl, apiVersion, id)
	attrs := make(map[string]string)
	attrs["MasterLabel"] = masterLabel
	attrs["ScriptContent"] = scriptContent

	_, err = f.httpPatch(url, attrs)
	return
}

func (f *Force) CreateDataPipeline(name string, masterLabel string, apiVersionNumber string, scriptContent string, scriptType string) (result ForceCreateRecordResult, err error, emessages []ForceError) {
	aurl := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/DataPipeline", f.Credentials.InstanceUrl, apiVersion)

	attrs := make(map[string]string)
	attrs["DeveloperName"] = name
	attrs["ScriptType"] = scriptType
	attrs["MasterLabel"] = masterLabel
	attrs["ApiVersion"] = apiVersionNumber
	attrs["ScriptContent"] = scriptContent

	body, err, emessages := f.httpPost(aurl, attrs)
	if err != nil {
		return
	}
	json.Unmarshal(body, &result)

	return

}

func (f *Force) CreateDataPipelineJob(id string) (result ForceCreateRecordResult, err error, emessages []ForceError) {
	aurl := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/DataPipelineJob", f.Credentials.InstanceUrl, apiVersion)

	attrs := make(map[string]string)
	attrs["DataPipelineId"] = id

	body, err, emessages := f.httpPost(aurl, attrs)
	if err != nil {
		return
	}
	json.Unmarshal(body, &result)

	return

}

func (f *Force) GetDataPipeline(name string) (results ForceQueryResult, err error) {

	query := fmt.Sprintf("SELECT Id, MasterLabel, DeveloperName, ScriptContent, ScriptType FROM DataPipeline Where DeveloperName = '%s'", name)
	results, err = f.QueryDataPipeline(query)

	return

}

func (f *Force) QueryDataPipeline(soql string) (results ForceQueryResult, err error) {
	aurl := fmt.Sprintf("%s/services/data/%s/tooling/query?q=%s", f.Credentials.InstanceUrl, apiVersion,
		url.QueryEscape(soql))

	body, err := f.httpGet(aurl)
	if err != nil {
		return
	}
	json.Unmarshal(body, &results)

	return

}

func (f *Force) QueryDataPipelineJob(soql string) (results ForceQueryResult, err error) {
	aurl := fmt.Sprintf("%s/services/data/%s/tooling/query?q=%s", f.Credentials.InstanceUrl, apiVersion,
		url.QueryEscape(soql))

	body, err := f.httpGet(aurl)
	if err != nil {
		return
	}
	json.Unmarshal(body, &results)

	return

}

func (f *Force) GetAuraBundles() (bundles AuraDefinitionBundleResult, definitions AuraDefinitionBundleResult, err error) {
	bundles, err = f.GetAuraBundlesList()
	definitions, err = f.GetAuraBundleDefinitions()
	return
}

func (f *Force) GetAuraBundleDefinitions() (definitions AuraDefinitionBundleResult, err error) {
	aurl := fmt.Sprintf("%s/services/data/%s/tooling/query?q=%s", f.Credentials.InstanceUrl, apiVersion,
		url.QueryEscape("SELECT Id, Source, AuraDefinitionBundleId, DefType, Format FROM AuraDefinition"))

	body, err := f.httpGet(aurl)
	if err != nil {
		return
	}
	json.Unmarshal(body, &definitions)

	if !definitions.Done {
		f.GetMoreAuraBundleDefinitions(&definitions)
	}

	return
}

func (f *Force) GetMoreAuraBundleDefinitions(definitions *AuraDefinitionBundleResult) (err error) {

	isDone := definitions.Done
	nextRecordsUrl := definitions.NextRecordsUrl

	for !isDone {

		moreDefs := new(AuraDefinitionBundleResult)
		aurl := fmt.Sprintf("%s%s", f.Credentials.InstanceUrl, nextRecordsUrl)

		body, err := f.httpGet(aurl)
		if err != nil {
			return err
		}
		json.Unmarshal(body, &moreDefs)

		definitions.Done = moreDefs.Done
		definitions.Records = append(definitions.Records, moreDefs.Records...)

		isDone = moreDefs.Done

		if !isDone {
			nextRecordsUrl = moreDefs.NextRecordsUrl
		}
	}

	return
}

func (f *Force) GetAuraBundlesList() (bundles AuraDefinitionBundleResult, err error) {
	aurl := fmt.Sprintf("%s/services/data/%s/tooling/query?q=%s", f.Credentials.InstanceUrl, apiVersion,
		url.QueryEscape("SELECT Id, DeveloperName, NamespacePrefix, ApiVersion, Description FROM AuraDefinitionBundle"))
	body, err := f.httpGet(aurl)
	if err != nil {
		return
	}
	json.Unmarshal(body, &bundles)

	return
}

func (f *Force) GetAuraBundle(bundleName string) (bundles AuraDefinitionBundleResult, definitions AuraDefinitionBundleResult, err error) {
	bundles, err = f.GetAuraBundleByName(bundleName)
	if len(bundles.Records) == 0 {
		ErrorAndExit(fmt.Sprintf("There is no Aura bundle named %q", bundleName))
	}
	bundle := bundles.Records[0]
	definitions, err = f.GetAuraBundleDefinition(bundle["Id"].(string))
	return
}

func (f *Force) GetAuraBundleByName(bundleName string) (bundles AuraDefinitionBundleResult, err error) {
	criteria := fmt.Sprintf(" Where DeveloperName = '%s'", bundleName)

	aurl := fmt.Sprintf("%s/services/data/%s/tooling/query?q=%s", f.Credentials.InstanceUrl, apiVersion,
		url.QueryEscape(fmt.Sprintf("SELECT Id, DeveloperName, NamespacePrefix, ApiVersion, Description FROM AuraDefinitionBundle%s", criteria)))

	body, err := f.httpGet(aurl)
	if err != nil {
		return
	}
	json.Unmarshal(body, &bundles)

	return
}

func (f *Force) GetAuraBundleDefinition(id string) (definitions AuraDefinitionBundleResult, err error) {
	aurl := fmt.Sprintf("%s/services/data/%s/tooling/query?q=%s", f.Credentials.InstanceUrl, apiVersion,
		url.QueryEscape(fmt.Sprintf("SELECT Id, Source, AuraDefinitionBundleId, DefType, Format FROM AuraDefinition WHERE AuraDefinitionBundleId = '%s'", id)))

	body, err := f.httpGet(aurl)
	if err != nil {
		return
	}
	json.Unmarshal(body, &definitions)

	return
}

func (f *Force) CreateAuraBundle(bundleName string) (result ForceCreateRecordResult, err error, emessages []ForceError) {
	aurl := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/AuraDefinitionBundle", f.Credentials.InstanceUrl, apiVersion)
	attrs := make(map[string]string)
	attrs["DeveloperName"] = bundleName
	attrs["Description"] = "An Aura Bundle"
	attrs["MasterLabel"] = bundleName
	attrs["ApiVersion"] = apiVersionNumber
	body, err, emessages := f.httpPost(aurl, attrs)
	if err != nil {
		return
	}
	json.Unmarshal(body, &result)

	return
}

func (f *Force) CreateAuraComponent(attrs map[string]string) (result ForceCreateRecordResult, err error, emessages []ForceError) {
	aurl := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/AuraDefinition", f.Credentials.InstanceUrl, apiVersion)
	body, err, emessages := f.httpPost(aurl, attrs)
	if err != nil {
		return
	}
	json.Unmarshal(body, &result)

	return
}

func (f *Force) ListSobjects() (sobjects []ForceSobject, err error) {
	url := fmt.Sprintf("%s/services/data/%s/sobjects", f.Credentials.InstanceUrl, apiVersion)
	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	var result ForceSobjectsResult
	json.Unmarshal(body, &result)
	sobjects = result.Sobjects
	return
}

func (f *Force) GetSobject(name string) (sobject ForceSobject, err error) {
	url := fmt.Sprintf("%s/services/data/%s/sobjects/%s/describe", f.Credentials.InstanceUrl, apiVersion, name)
	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	json.Unmarshal(body, &sobject)
	return
}

func (f *Force) Query(query string, isTooling bool) (result ForceQueryResult, err error) {
	vurl := ""
	if isTooling == true {
		vurl = fmt.Sprintf("%s/services/data/%s/tooling/query?q=%s", f.Credentials.InstanceUrl, apiVersion, url.QueryEscape(query))
	} else {
		vurl = fmt.Sprintf("%s/services/data/%s/query?q=%s", f.Credentials.InstanceUrl, apiVersion, url.QueryEscape(query))
	}
	body, err := f.httpGet(vurl)
	if err != nil {
		return
	}
	json.Unmarshal(body, &result)
	if result.Done == false {
		var nextResult ForceQueryResult
		nextResult.NextRecordsUrl = result.NextRecordsUrl
		for nextResult.Done == false {
			nextUrl := fmt.Sprintf("%s%s", f.Credentials.InstanceUrl, nextResult.NextRecordsUrl)
			nextBody, nextErr := f.httpGet(nextUrl)
			if nextErr != nil {
				return
			}
			nextResult.Records = []ForceRecord{}
			json.Unmarshal(nextBody, &nextResult)

			result.Records = append(result.Records, nextResult.Records...)
		}
	}
	return
}

func (f *Force) Get(url string) (object ForceRecord, err error) {
	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &object)
	return
}

func (f *Force) GetLimits() (result map[string]ForceLimit, err error) {

	url := fmt.Sprintf("%s/services/data/%s/limits", f.Credentials.InstanceUrl, apiVersion)
	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(body), &result)
	return

}

func (f *Force) GetPasswordStatus(id string) (result ForcePasswordStatusResult, err error) {
	url := fmt.Sprintf("%s/services/data/%s/sobjects/User/%s/password", f.Credentials.InstanceUrl, apiVersion, id)
	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &result)
	return
}

func (f *Force) ResetPassword(id string) (result ForcePasswordResetResult, err error) {
	url := fmt.Sprintf("%s/services/data/%s/sobjects/User/%s/password", f.Credentials.InstanceUrl, apiVersion, id)
	body, err := f.httpDelete(url)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &result)
	return
}

func (f *Force) ChangePassword(id string, attrs map[string]string) (result string, err error, emessages []ForceError) {
	url := fmt.Sprintf("%s/services/data/%s/sobjects/User/%s/password", f.Credentials.InstanceUrl, apiVersion, id)
	_, err, emessages = f.httpPost(url, attrs)
	return
}

func (f *Force) GetRecord(sobject, id string) (object ForceRecord, err error) {
	fields := strings.Split(id, ":")
	var url string
	if len(fields) == 1 {
		url = fmt.Sprintf("%s/services/data/%s/sobjects/%s/%s", f.Credentials.InstanceUrl, apiVersion, sobject, id)
	} else {
		url = fmt.Sprintf("%s/services/data/%s/sobjects/%s/%s/%s", f.Credentials.InstanceUrl, apiVersion, sobject, fields[0], fields[1])
	}

	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &object)
	return
}

func (f *Force) CreateRecord(sobject string, attrs map[string]string) (id string, err error, emessages []ForceError) {
	url := fmt.Sprintf("%s/services/data/%s/sobjects/%s", f.Credentials.InstanceUrl, apiVersion, sobject)
	body, err, emessages := f.httpPost(url, attrs)
	var result ForceCreateRecordResult
	json.Unmarshal(body, &result)
	id = result.Id
	return
}

func (f *Force) CreateBulkJob(xmlbody string) (result JobInfo, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job", f.Credentials.InstanceUrl, apiVersionNumber)
	body, err := f.httpPostXML(url, xmlbody)
	xml.Unmarshal(body, &result)
	if len(result.Id) == 0 {
		var fault LoginFault
		xml.Unmarshal(body, &fault)
		err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
	}
	return
}

func (f *Force) CloseBulkJob(jobId string, xmlbody string) (result JobInfo, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s", f.Credentials.InstanceUrl, apiVersionNumber, jobId)
	body, err := f.httpPostXML(url, xmlbody)
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

func (f *Force) AddBatchToJob(xmlbody string, jobId string) (result BatchInfo, err error) {
	url := fmt.Sprintf("%s/services/async/%s/job/%s/batch", f.Credentials.InstanceUrl, apiVersionNumber, jobId)
	body, err := f.httpPostCSV(url, xmlbody)
	xml.Unmarshal(body, &result)
	if len(result.Id) == 0 {
		var fault LoginFault
		xml.Unmarshal(body, &fault)
		err = errors.New(fmt.Sprintf("%s: %s", fault.ExceptionCode, fault.ExceptionMessage))
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

func (f *Force) QueryTraceFlags() (results ForceQueryResult, err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/query/?q=Select+Id,+ApexCode,+ApexProfiling,+Callout,+CreatedDate,+Database,+ExpirationDate,+Scope.Name,+System,+TracedEntity.Name,+Validation,+Visualforce,+Workflow+From+TraceFlag+Order+By+ExpirationDate,TracedEntity.Name,Scope.Name", f.Credentials.InstanceUrl, apiVersion)
	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	json.Unmarshal(body, &results)
	return
}

func (f *Force) StartTrace(userId ...string) (result ForceCreateRecordResult, err error, emessages []ForceError) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/TraceFlag", f.Credentials.InstanceUrl, apiVersion)

	// The log levels are currently hard-coded to a useful level of logging
	// without hitting the maximum log size of 2MB in most cases, hopefully.
	attrs := make(map[string]string)
	attrs["ApexCode"] = "Debug"
	attrs["ApexProfiling"] = "Error"
	attrs["Callout"] = "Info"
	attrs["Database"] = "Info"
	attrs["System"] = "Info"
	attrs["Validation"] = "Warn"
	attrs["Visualforce"] = "Info"
	attrs["Workflow"] = "Info"
	if len(userId) == 1 {
		attrs["TracedEntityId"] = userId[0]
	} else {
		attrs["TracedEntityId"] = f.Credentials.UserId
	}

	body, err, emessages := f.httpPost(url, attrs)
	if err != nil {
		return
	}
	json.Unmarshal(body, &result)

	return
}

func (f *Force) RetrieveLog(logId string) (result string, err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/ApexLog/%s/Body", f.Credentials.InstanceUrl, apiVersion, logId)
	body, err := f.httpGet(url)
	result = string(body)
	return
}

func (f *Force) QueryLogs() (results ForceQueryResult, err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/query/?q=Select+Id,+Application,+DurationMilliseconds,+Location,+LogLength,+LogUser.Name,+Operation,+Request,StartTime,+Status+From+ApexLog+Order+By+StartTime", f.Credentials.InstanceUrl, apiVersion)
	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	json.Unmarshal(body, &results)
	return
}

func (f *Force) RetrieveEventLogFile(elfId string) (result string, err error) {
	url := fmt.Sprintf("%s/services/data/%s/sobjects/EventLogFile/%s/LogFile", f.Credentials.InstanceUrl, apiVersion, elfId)
	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	result = string(body)
	return
}

func (f *Force) QueryEventLogFiles() (results ForceQueryResult, err error) {
	url := fmt.Sprintf("%s/services/data/%s/query/?q=Select+Id,+LogDate,+EventType,+LogFileLength+FROM+EventLogFile+ORDER+BY+LogDate+DESC,+EventType", f.Credentials.InstanceUrl, apiVersion)
	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	json.Unmarshal(body, &results)
	return
}

func (f *Force) UpdateAuraComponent(source map[string]string, id string) (err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/AuraDefinition/%s", f.Credentials.InstanceUrl, apiVersion, id)
	_, err = f.httpPatch(url, source)
	return
}

func (f *Force) DeleteToolingRecord(objecttype string, id string) (err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/%s/%s", f.Credentials.InstanceUrl, apiVersion, objecttype, id)
	_, err = f.httpDelete(url)
	return
}

func (f *Force) DescribeSObject(objecttype string) (result string, err error) {
	url := fmt.Sprintf("%s/services/data/%s/sobjects/%s/describe", f.Credentials.InstanceUrl, apiVersion, objecttype)
	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	result = string(body)
	return
}

func (f *Force) UpdateRecord(sobject string, id string, attrs map[string]string) (err error) {
	fields := strings.Split(id, ":")
	var url string
	if len(fields) == 1 {
		url = fmt.Sprintf("%s/services/data/%s/sobjects/%s/%s", f.Credentials.InstanceUrl, apiVersion, sobject, id)
	} else {
		url = fmt.Sprintf("%s/services/data/%s/sobjects/%s/%s/%s", f.Credentials.InstanceUrl, apiVersion, sobject, fields[0], fields[1])
	}
	_, err = f.httpPatch(url, attrs)
	return
}

func (f *Force) DeleteRecord(sobject string, id string) (err error) {
	url := fmt.Sprintf("%s/services/data/%s/sobjects/%s/%s", f.Credentials.InstanceUrl, apiVersion, sobject, id)
	_, err = f.httpDelete(url)
	return
}

func (f *Force) Whoami() (me ForceRecord, err error) {
	parts := strings.Split(f.Credentials.Id, "/")
	me, err = f.GetRecord("User", parts[len(parts)-1])
	return
}

func (f *Force) httpGet(url string) (body []byte, err error) {
	body, err = f.httpGetRequest(url, "Authorization", fmt.Sprintf("Bearer %s", f.Credentials.AccessToken))
	return
}

func (f *Force) httpGetBulk(url string) (body []byte, err error) {
	body, err = f.httpGetRequest(url, "X-SFDC-Session", fmt.Sprintf("Bearer %s", f.Credentials.AccessToken))
	return
}

func (f *Force) httpGetRequest(url string, headerName string, headerValue string) (body []byte, err error) {
	req, err := httpRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Add(headerName, headerValue)
	res, err := doRequest(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	if res.StatusCode == 403 {
		err = errors.New("Forbidden; Your authorization may have expired, or you do not have access. Please run `force login` and try again")
		return
	}
	body, err = ioutil.ReadAll(res.Body)
	if res.StatusCode/100 != 2 {
		var messages []ForceError
		json.Unmarshal(body, &messages)
		if len(messages) > 0 {
			err = errors.New(messages[0].Message)
		} else {
			err = errors.New(string(body))
		}
		return
	}
	return
}

func (f *Force) httpPostCSV(url string, data string) (body []byte, err error) {
	body, err = f.httpPostWithContentType(url, data, "text/csv")
	return
}

func (f *Force) httpPostXML(url string, data string) (body []byte, err error) {
	body, err = f.httpPostWithContentType(url, data, "application/xml")
	return
}

func (f *Force) httpPostWithContentType(url string, data string, contenttype string) (body []byte, err error) {
	rbody := data
	req, err := httpRequest("POST", url, bytes.NewReader([]byte(rbody)))
	if err != nil {
		return
	}

	req.Header.Add("X-SFDC-Session", f.Credentials.AccessToken)
	req.Header.Add("Content-Type", contenttype)
	res, err := doRequest(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	body, err = ioutil.ReadAll(res.Body)

	if res.StatusCode/100 != 2 {
		var messages []ForceError
		json.Unmarshal(body, &messages)
		if messages != nil {
			err = errors.New(messages[0].Message)
		}
		return
	}
	return
}

func (f *Force) httpPost(url string, attrs map[string]string) (body []byte, err error, emessages []ForceError) {
	rbody, _ := json.Marshal(attrs)
	req, err := httpRequest("POST", url, bytes.NewReader(rbody))
	if err != nil {
		return
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.Credentials.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	res, err := doRequest(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	body, err = ioutil.ReadAll(res.Body)
	if res.StatusCode/100 != 2 {
		var messages []ForceError
		json.Unmarshal(body, &messages)
		err = errors.New(messages[0].Message)
		emessages = messages
		return
	}
	return
}

func (f *Force) httpPatch(url string, attrs map[string]string) (body []byte, err error) {
	rbody, _ := json.Marshal(attrs)
	req, err := httpRequest("PATCH", url, bytes.NewReader(rbody))
	if err != nil {
		return
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.Credentials.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	res, err := doRequest(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		err = errors.New("Authorization expired, please run `force login`")
		return
	}
	body, err = ioutil.ReadAll(res.Body)
	if res.StatusCode/100 != 2 {
		var messages []ForceError
		json.Unmarshal(body, &messages)
		err = errors.New(messages[0].Message)
		return
	}
	return
}

func (f *Force) httpDelete(url string) (body []byte, err error) {
	req, err := httpRequest("DELETE", url, nil)
	if err != nil {
		return
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.Credentials.AccessToken))
	res, err := doRequest(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	body, err = ioutil.ReadAll(res.Body)
	if res.StatusCode/100 != 2 {
		var messages []ForceError
		json.Unmarshal(body, &messages)
		err = errors.New(messages[0].Message)
		return
	}
	return
}

func doRequest(request *http.Request) (res *http.Response, err error) {
	client := &http.Client{}
	return client.Do(request)
}

func httpRequest(method, url string, body io.Reader) (request *http.Request, err error) {
	request, err = http.NewRequest(method, url, body)
	if err != nil {
		return
	}
	request.Header.Add("User-Agent", fmt.Sprintf("force/%s (%s-%s)", Version, runtime.GOOS, runtime.GOARCH))
	return
}

func startLocalHttpServer(ch chan ForceCredentials) (port int, err error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return
	}
	port = listener.Addr().(*net.TCPAddr).Port
	h := http.NewServeMux()
	url := "https://force-cli"
	if Version == "dev" {
		url = fmt.Sprintf("%s%s", url, "staging")
	}
	url = fmt.Sprintf("%s%s", url, ".herokuapp.com")
	h.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", url)
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Headers", "X-Requested-With")
		} else {
			query := r.URL.Query()
			var creds ForceCredentials
			creds.AccessToken = query.Get("access_token")
			creds.Id = query.Get("id")
			creds.InstanceUrl = query.Get("instance_url")
			creds.IssuedAt = query.Get("issued_at")
			creds.Scope = query.Get("scope")
			ch <- creds
			if _, ok := r.Header["X-Requested-With"]; ok == false {
				http.Redirect(w, r, fmt.Sprintf("%s/auth/complete", url), http.StatusSeeOther)
			}
			listener.Close()
		}
	})
	go http.Serve(listener, h)
	return
}
