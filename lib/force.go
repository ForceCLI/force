package lib

import (
	"bytes"
	"container/list"
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
	"strconv"
	"strings"
	"time"

	"github.com/ForceCLI/force/desktop"
	. "github.com/ForceCLI/force/error"
)

var (
	ClientId    = "3MVG9ytVT1SanXDnX_hOa9Ys5NxVp5C26JlyQjwr.xTJtUqoKonXY.M8CcjoEknMrV4YUvPvXLiMyzI.Aw23C"
	RedirectUri = "http://localhost:3835/oauth/callback"
)

var Timeout int64 = 0
var CustomEndpoint = ``
var SessionExpiredError = errors.New("Session expired")

const (
	EndpointProduction = iota
	EndpointTest       = iota
	EndpointPrerelease = iota
	EndpointMobile1    = iota
	EndpointCustom     = iota
)

const (
	RefreshUnavailable = iota
	RefreshOauth       = iota
	RefreshSFDX        = iota
)

type RefreshMethod int

type Force struct {
	Credentials *ForceSession
	Metadata    *ForceMetadata
	Partner     *ForcePartner
}

type UserInfo struct {
	UserName     string `json:"preferred_username"`
	OrgId        string `json:"organization_id"`
	UserId       string `json:"user_id"`
	ProfileId    string
	OrgNamespace string
}

type SessionOptions struct {
	ApiVersion    string
	Alias         string
	RefreshMethod RefreshMethod
}

type ForceSession struct {
	AccessToken    string `json:"access_token"`
	InstanceUrl    string `json:"instance_url"`
	IssuedAt       string `json:"issued_at"`
	Scope          string `json:"scope"`
	ClientId       string
	RefreshToken   string
	ForceEndpoint  ForceEndpoint
	UserInfo       *UserInfo
	SessionOptions *SessionOptions
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

type FieldName struct {
	FieldName string
	IsObject  bool
}

type SelectStruct struct {
	ObjectName string
	FieldNames []FieldName
}

type ForceEndpoint int

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

type ForceRecord map[string]interface{}

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

type QueryOptions struct {
	IsTooling bool
	QueryAll  bool
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

func NewForce(creds *ForceSession) (force *Force) {
	force = new(Force)
	force.Credentials = creds
	force.Metadata = NewForceMetadata(force)
	force.Partner = NewForcePartner(force)
	return
}

func ForceSoapLogin(endpoint ForceEndpoint, username string, password string) (creds ForceSession, err error) {
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
	if err != nil {
		ErrorAndExit(err.Error())
	}
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
	creds = ForceSession{
		AccessToken:   result.SessionId,
		InstanceUrl:   instanceUrl,
		ForceEndpoint: endpoint,
		UserInfo: &UserInfo{
			OrgId:  orgid,
			UserId: result.Id,
		},
		SessionOptions: &SessionOptions{
			ApiVersion: apiVersionNumber,
		},
	}
	return
}

func (f *Force) refreshTokenURL() string {
	var refreshURL string
	endpoint := f.Credentials.ForceEndpoint
	switch endpoint {
	case EndpointProduction:
		refreshURL = fmt.Sprintf("https://login.salesforce.com/services/oauth2/token")
	case EndpointTest:
		refreshURL = fmt.Sprintf("https://test.salesforce.com/services/oauth2/token")
	case EndpointPrerelease:
		refreshURL = fmt.Sprintf("https://prerellogin.pre.salesforce.com/services/oauth2/token")
	case EndpointMobile1:
		refreshURL = fmt.Sprintf("https://EndpointMobile1.t.salesforce.com/services/oauth2/token")
	case EndpointCustom:
		refreshURL = fmt.Sprintf("%s/services/oauth2/token", CustomEndpoint)
	default:
		ErrorAndExit("no such endpoint type")
	}
	return refreshURL
}

func ForceLogin(endpoint ForceEndpoint) (creds ForceSession, err error) {
	ch := make(chan ForceSession)
	port, err := startLocalHttpServer(ch)
	var url string

	Redir := RedirectUri

	switch endpoint {
	case EndpointProduction:
		url = fmt.Sprintf("https://login.salesforce.com/services/oauth2/authorize?response_type=token&client_id=%s&redirect_uri=%s&state=%d&prompt=login", ClientId, Redir, port)
	case EndpointTest:
		url = fmt.Sprintf("https://test.salesforce.com/services/oauth2/authorize?response_type=token&client_id=%s&redirect_uri=%s&state=%d&prompt=login", ClientId, Redir, port)
	case EndpointPrerelease:
		url = fmt.Sprintf("https://prerellogin.pre.salesforce.com/services/oauth2/authorize?response_type=token&client_id=%s&redirect_uri=%s&state=%d&prompt=login", ClientId, Redir, port)
	case EndpointMobile1:
		url = fmt.Sprintf("https://EndpointMobile1.t.salesforce.com/services/oauth2/authorize?response_type=token&client_id=%s&redirect_uri=%s&state=%d&prompt=login", ClientId, Redir, port)
	case EndpointCustom:
		url = fmt.Sprintf("%s/services/oauth2/authorize?response_type=token&client_id=%s&redirect_uri=%s&state=%d&prompt=login", CustomEndpoint, ClientId, Redir, port)
	default:
		ErrorAndExit("Unable to login with OAuth. Unknown endpoint type")
	}

	err = desktop.Open(url)
	creds = <-ch
	if creds.RefreshToken != "" {
		creds.SessionOptions = &SessionOptions{
			RefreshMethod: RefreshOauth,
		}
	}
	creds.ForceEndpoint = endpoint
	creds.ClientId = ClientId
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

func (f *Force) QueryAndSend(query string, processor chan<- ForceRecord, options ...func(*QueryOptions)) (err error) {
	queryOptions := QueryOptions{}
	for _, option := range options {
		option(&queryOptions)
	}
	cmd := "query"
	if queryOptions.QueryAll {
		cmd = "queryAll"
	}
	if queryOptions.IsTooling {
		cmd = "tooling/" + cmd
	}
	processResults := func(body []byte) (result ForceQueryResult, err error) {
		err = json.Unmarshal(body, &result)
		if err != nil {
			return
		}
		for _, row := range result.Records {
			processor <- row
		}
		return
	}

	var body []byte
	url := fmt.Sprintf("%s/services/data/%s/%s?q=%s", f.Credentials.InstanceUrl, apiVersion, cmd, url.QueryEscape(query))
	for {
		body, err = f.httpGet(url)
		if err != nil {
			return
		}
		var result ForceQueryResult
		result, err = processResults(body)
		if err != nil {
			return
		}
		if result.Done {
			break
		}
		url = fmt.Sprintf("%s%s", f.Credentials.InstanceUrl, result.NextRecordsUrl)
	}
	close(processor)
	return
}

func (f *Force) Query(query string, options ...func(*QueryOptions)) (result ForceQueryResult, err error) {
	queryOptions := QueryOptions{}
	for _, option := range options {
		option(&queryOptions)
	}
	cmd := "query"
	if queryOptions.QueryAll {
		cmd = "queryAll"
	}
	if queryOptions.IsTooling {
		cmd = "tooling/" + cmd
	}

	result = ForceQueryResult{
		Done:           false,
		NextRecordsUrl: fmt.Sprintf("%s/services/data/%s/%s?q=%s", f.Credentials.InstanceUrl, apiVersion, cmd, url.QueryEscape(query)),
		TotalSize:      0,
		Records:        []ForceRecord{},
	}

	/* The Force API will split queries returning large result sets into
	 * multiple pieces (generally every 200 records). We need to repeatedly
	 * query until we've retrieved all of them. */
	for !result.Done {
		var body []byte
		body, err = f.httpGet(result.NextRecordsUrl)

		if err != nil {
			return
		}

		var currResult ForceQueryResult
		json.Unmarshal(body, &currResult)
		result.Update(currResult, f)
	}

	return
}

func (f *Force) DecodeMe2(jsonStream string) (result ForceQueryResult) {
	dec := json.NewDecoder(strings.NewReader(jsonStream))
	dec.UseNumber()
	recordsFound := false

	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			ErrorAndExit(err.Error())
		}

		var tokenType = fmt.Sprintf("%T", t)
		var token = fmt.Sprintf("%v", t)
		//fmt.Printf("%s: %s\n", tokenType, token)
		if tokenType == "json.Delim" {

		} else {
			if !recordsFound {
				switch token {
				case "totalSize":
					t, _ = dec.Token()
					v, _ := strconv.Atoi(t.(json.Number).String())
					result.TotalSize = v
				case "done":
					t, _ = dec.Token()
					v, _ := t.(bool)
					result.Done = v
				case "nextRecordsUrl":
					t, _ = dec.Token()
					result.NextRecordsUrl = fmt.Sprintf("%v", t)
				case "records":
					recordsFound = true
				}

			}
		}
		switch tokenType {
		case "{":
			if !recordsFound {
				// This should be the start of the entire json
			} else {
				// Need to set a flag that in the next loop we are adding fields
				// to a child object
			}
		default:

		}
	}
	return
}

func (f *Force) DecodeMe(jsonStream string) (result *list.List) {
	type val interface{}
	type keyval struct {
		Key string
		Val val
	}
	dec := json.NewDecoder(strings.NewReader(jsonStream))
	dec.UseNumber()
	//var records = list.New()
	//var currentContainer = new(val)
	var recordsFound = false
	//var stack = make([]string, 0, 0)
	result = list.New()
	SObjecttype := ""
	var isAttributes = false
	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			ErrorAndExit(err.Error())
		}

		var tokenType = fmt.Sprintf("%T", t)
		var token = fmt.Sprintf("%v", t)
		//fmt.Printf("tokenType: %s, token: %s\n", tokenType, token)
		if token == "type" {
			t, err = dec.Token()
			token = fmt.Sprintf("%v", t)
			SObjecttype = token
			//fmt.Printf("case token=type %s\n", SObjecttype)
			spec := f.GetObjectSpec(SObjecttype, result)
			spec.ObjectName = token
		} else if token == "totalSize" {
			t, err = dec.Token()
		} else if token == "done" {
			t, err = dec.Token()
		} else if token == "records" {
			//fmt.Println("Records have been found...")
			recordsFound = true
		} else if token == "attributes" {
			isAttributes = true
		} else if tokenType != "json.Delim" && recordsFound && token != "attributes" && token != "url" {
			//fmt.Printf("\ncase fieldName: %v.%v\n", SObjecttype, token)
			spec := f.GetObjectSpec(SObjecttype, result)
			//fmt.Printf("%v:", token)
			t, err = dec.Token()
			tt := fmt.Sprintf("%T", t)
			//if /*t != nil &&*/ tt != "json.Delim" {
			//fmt.Printf("%T\n", t)
			f.PushFieldName(token, spec, (tt == "json.Delim" || t == nil))
			/*if tt == "json.Number" {
				fmt.Printf("Value: %#v\n",  JSONNumberToString(t, ','))
			} else {
				fmt.Printf("Value: %s\n", t)
			}*/
			//}
			//fmt.Printf("\n")
		} else {
			if err != nil {
				ErrorAndExit(err.Error())
			}
			if token == "url" {
				t, err = dec.Token()
			} else if token == "[" {
				//fmt.Println("Starting Array...")
			} else if token == "]" {
				//fmt.Println("Ending Array...")
			} else if token == "}" {
				if isAttributes {
					isAttributes = false
				} else {
					prev := f.GetPrevObjectSpec(SObjecttype, result)
					if prev != nil {
						SObjecttype = prev.ObjectName
						//fmt.Printf("Prev Obj: %s\n\n", prev.ObjectName)
					} else {
						//fmt.Printf("NO PREV OBJ\n\n")
					}
					//fmt.Println("Ending Object...")
				}
			} else if token == "{" {
				//spec := f.GetObjectSpec(SObjecttype, result)
				//result.PushFront(spec)
				//fmt.Println("Starting Object...")
			}
		}
		f.DumpListStack(result)
	}
	f.DumpListStack(result)
	return
}

func (f *Force) DumpListStack(l *list.List) {
	fmt.Printf("\nDecode Results:\n")
	for e := l.Front(); e != nil; e = e.Next() {
		spec := e.Value.(*SelectStruct)
		fmt.Println(spec.ObjectName)
		for _, v := range spec.FieldNames {
			fmt.Printf("\t%v", v.FieldName)
			if v.IsObject {
				fmt.Printf(" (Object)\n")
			} else {
				fmt.Printf("\n")
			}
		}
	}
	fmt.Printf("\n\n")
}

func (f *Force) PushFieldName(fieldName string, spec *SelectStruct, IsObject bool) {
	//fmt.Println("Pushing fieldname: ", fieldName)
	for _, v := range spec.FieldNames {
		if v.FieldName == fieldName {
			return
		}
	}
	spec.FieldNames = append(spec.FieldNames, FieldName{fieldName, IsObject})
	return
}

func (f *Force) GetPrevObjectSpec(objectName string, l *list.List) (foundItem *SelectStruct) {
	for e := l.Front(); e != nil; e = e.Next() {
		if e.Value.(*SelectStruct).ObjectName == objectName {
			p := e.Prev()
			if p != nil {
				foundItem = e.Prev().Value.(*SelectStruct)
				return
			}
		}
	}
	return
}

func (f *Force) GetObjectSpec(objectName string, l *list.List) (result *SelectStruct) {
	//fmt.Println("Looking for Spec", objectName)
	found, result := f.HasObject(objectName, l)
	if !found {
		result = new(SelectStruct)
		result.ObjectName = objectName
		l.PushBack(result)
	}
	return
}

func (f *Force) HasObject(objectName string, l *list.List) (result bool, foundItem *SelectStruct) {
	for e := l.Front(); e != nil; e = e.Next() {
		if e.Value.(*SelectStruct).ObjectName == objectName {
			result = true
			foundItem = e.Value.(*SelectStruct)
			return
		}
	}
	result = false
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

func (f *Force) SetFLS(profileId string, objectName string, fieldName string) {
	// First, write out a file to a temporary location with a package.xml

	f.Metadata.UpdateFLSOnProfile(objectName, fieldName)
}

func (f *Force) QueryProfile(fields ...string) (results ForceQueryResult, err error) {

	url := fmt.Sprintf("%s/services/data/%s/tooling/query?q=Select+%s+From+Profile+Where+Id='%s'",
		f.Credentials.InstanceUrl,
		apiVersion,
		strings.Join(fields, ","),
		f.Credentials.UserInfo.ProfileId)

	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	json.Unmarshal(body, &results)
	return
}

func (f *Force) QueryTraceFlags() (results ForceQueryResult, err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/query/?q=Select+Id,+DebugLevel.DeveloperName,++ApexCode,+ApexProfiling,+Callout,+CreatedDate,+Database,+ExpirationDate,+System,+TracedEntity.Name,+Validation,+Visualforce,+Workflow+From+TraceFlag+Order+By+ExpirationDate,TracedEntity.Name", f.Credentials.InstanceUrl, apiVersion)
	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	json.Unmarshal(body, &results)
	return
}

func (f *Force) QueryDefaultDebugLevel() (id string, err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/query/?q=Select+Id+From+DebugLevel+Where+DeveloperName+=+'Force_CLI'", f.Credentials.InstanceUrl, apiVersion)
	body, err := f.httpGet(url)
	if err != nil {
		return
	}
	var results ForceQueryResult
	json.Unmarshal(body, &results)
	if len(results.Records) == 1 {
		id = results.Records[0]["Id"].(string)
	}
	return
}

func (f *Force) DefaultDebugLevel() (id string, err error, emessages []ForceError) {
	id, err = f.QueryDefaultDebugLevel()
	if err != nil || id != "" {
		return
	}
	url := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/DebugLevel", f.Credentials.InstanceUrl, apiVersion)

	// The log levels are currently hard-coded to a useful level of logging
	// without hitting the maximum log size of 2MB in most cases, hopefully.
	attrs := make(map[string]string)
	attrs["ApexCode"] = "Fine"
	attrs["ApexProfiling"] = "Error"
	attrs["Callout"] = "Info"
	attrs["Database"] = "Info"
	attrs["System"] = "Info"
	attrs["Validation"] = "Warn"
	attrs["Visualforce"] = "Info"
	attrs["Workflow"] = "Info"
	attrs["DeveloperName"] = "Force_CLI"
	attrs["MasterLabel"] = "Force_CLI"

	body, err, emessages := f.httpPost(url, attrs)
	if err != nil {
		return
	}
	var result ForceCreateRecordResult
	json.Unmarshal(body, &result)
	if result.Success {
		id = result.Id
	}

	return
}

func (f *Force) StartTrace(userId ...string) (result ForceCreateRecordResult, err error, emessages []ForceError) {
	debugLevel, err, emessages := f.DefaultDebugLevel()
	if err != nil {
		return
	}
	url := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/TraceFlag", f.Credentials.InstanceUrl, apiVersion)

	attrs := make(map[string]string)
	attrs["DebugLevelId"] = debugLevel
	if len(userId) == 1 {
		attrs["TracedEntityId"] = userId[0]
		attrs["LogType"] = "USER_DEBUG"
	} else {
		attrs["TracedEntityId"] = f.Credentials.UserInfo.UserId
		attrs["LogType"] = "DEVELOPER_LOG"
	}

	body, err, emessages := f.httpPost(url, attrs)
	if err != nil {
		return
	}
	json.Unmarshal(body, &result)

	return
}

func (f *Force) GetConsoleLogLevelId() (result string, err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/query?q=Select+Id+From+DebugLevel+Where+DeveloperName+=+'SFDC_DevConsole'", f.Credentials.InstanceUrl, apiVersion)
	body, err := f.httpGet(url)
	var res ForceQueryResult
	if err != nil {
		return
	}
	json.Unmarshal(body, &res)
	result = res.Records[0]["Id"].(string)
	fmt.Println(result)
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

func (force *Force) useHourlyLogs() bool {
	const EventLogFile string = "EventLogFile"
	const HourlyEnabledField string = "Sequence"
	sobject, err := force.GetSobject(EventLogFile)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fields := ForceSobjectFields(sobject["fields"].([]interface{}))
	for _, f := range fields {
		field := f.(map[string]interface{})
		if field["name"] == HourlyEnabledField {
			return true
		}
	}
	return false
}

func (f *Force) QueryEventLogFiles() (results ForceQueryResult, err error) {
	url := ""
	currApi, e := strconv.ParseFloat(f.Credentials.SessionOptions.ApiVersion, 64)
	if e != nil {
		ErrorAndExit(e.Error())
	}
	if f.useHourlyLogs() && currApi >= 37.0 {
		url = fmt.Sprintf("%s/services/data/%s/query/?q=Select+Id,+LogDate,+EventType,+LogFileLength,+Sequence,+Interval+FROM+EventLogFile+ORDER+BY+LogDate+DESC,+EventType,+Sequence,+Interval", f.Credentials.InstanceUrl, apiVersion)
	} else {
		url = fmt.Sprintf("%s/services/data/%s/query/?q=Select+Id,+LogDate,+EventType,+LogFileLength+FROM+EventLogFile+ORDER+BY+LogDate+DESC,+EventType", f.Credentials.InstanceUrl, apiVersion)
	}
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

func (f *Force) CreateToolingRecord(objecttype string, attrs map[string]string) (result ForceCreateRecordResult, err error) {
	aurl := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/%s", f.Credentials.InstanceUrl, apiVersion, objecttype)
	fmt.Println(attrs)
	body, err, _ := f.httpPost(aurl, attrs)

	if err != nil {
		return
	}
	fmt.Println(string(body))
	json.Unmarshal(body, &result)

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
	me, err = f.GetRecord("User", f.Credentials.UserInfo.UserId)
	return
}

// Prepend /services/data/vXX.0 to URL
func (f *Force) fullRestUrl(url string) string {
	return fmt.Sprintf("/services/data/%s/%s", apiVersion, strings.TrimLeft(url, "/"))
}

// Prepend https schema and instance to URL
func (f *Force) qualifyUrl(url string) string {
	return fmt.Sprintf("%s/%s", f.Credentials.InstanceUrl, strings.TrimLeft(url, "/"))
}

func (f *Force) GetAbsolute(url string) (result string, err error) {
	qualifiedUrl := f.qualifyUrl(url)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", f.Credentials.AccessToken),
	}
	body, err := f.httpGetRequest(qualifiedUrl, headers)
	if err == SessionExpiredError {
		f.RefreshSessionOrExit()
		return f.GetREST(url)
	}
	result = string(body)
	return
}

func (f *Force) GetREST(url string) (result string, err error) {
	fullUrl := f.fullRestUrl(url)
	return f.GetAbsolute(fullUrl)
}

func (f *Force) PostPatchAbsolute(url string, content string, method string) (result string, err error) {
	if method == "POST" {
		return f.PostAbsolute(url, content)
	} else {
		return f.PatchAbsolute(url, content)
	}
}

func (f *Force) PostPatchREST(url string, content string, method string) (result string, err error) {
	if method == "POST" {
		return f.PostREST(url, content)
	} else {
		return f.PatchREST(url, content)
	}
}

func (f *Force) PostAbsolute(url string, content string) (result string, err error) {
	qualifiedUrl := f.qualifyUrl(url)
	body, err := f.httpPostJSON(qualifiedUrl, content)
	if err == SessionExpiredError {
		f.RefreshSessionOrExit()
		return f.PostREST(url, content)
	}
	result = string(body)
	return
}

func (f *Force) PostREST(url string, content string) (result string, err error) {
	fullUrl := f.fullRestUrl(url)
	return f.PostAbsolute(fullUrl, content)
}

func (f *Force) PatchAbsolute(url string, content string) (result string, err error) {
	qualifiedUrl := f.qualifyUrl(url)
	body, err := f.httpPatchJSON(qualifiedUrl, content)
	if err == SessionExpiredError {
		f.RefreshSessionOrExit()
		return f.PatchREST(url, content)
	}
	result = string(body)
	return
}

func (f *Force) PatchREST(url string, content string) (result string, err error) {
	fullUrl := f.fullRestUrl(url)
	return f.PatchAbsolute(fullUrl, content)
}

func (f *Force) getForceResult(url string) (results ForceQueryResult, err error) {
	body, err := f.httpGet(fmt.Sprintf("%s%s", f.Credentials.InstanceUrl, url))
	if err != nil {
		return
	}
	json.Unmarshal(body, &results)
	return
}

func (f *Force) httpGet(url string) (body []byte, err error) {
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", f.Credentials.AccessToken),
	}
	body, err = f.httpGetRequest(url, headers)
	if err == SessionExpiredError {
		f.RefreshSessionOrExit()
		return f.httpGet(url)
	}
	return
}

func (f *Force) httpGetBulk(url string) (body []byte, err error) {
	headers := map[string]string{
		"X-SFDC-Session": fmt.Sprintf("Bearer %s", f.Credentials.AccessToken),
		"Content-Type":   "application/xml",
	}
	body, err = f.httpGetRequest(url, headers)
	if err == SessionExpiredError {
		f.RefreshSessionOrExit()
		return f.httpGetBulk(url)
	}
	return
}

func (f *Force) httpGetRequest(url string, headers map[string]string) (body []byte, err error) {
	req, err := httpRequest("GET", url, nil)
	if err != nil {
		return
	}
	for headerName, headerValue := range headers {
		req.Header.Add(headerName, headerValue)
	}
	res, err := doRequest(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		err = SessionExpiredError
		return
	}
	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	if res.StatusCode/100 != 2 {
		contentType := res.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "application/xml") {
			var fault LoginFault
			xml.Unmarshal(body, &fault)
			if fault.ExceptionCode == "InvalidSessionId" {
				err = SessionExpiredError
			}
		} else {
			var messages []ForceError
			json.Unmarshal(body, &messages)
			if len(messages) > 0 {
				err = errors.New(messages[0].Message)
			} else {
				err = errors.New(string(body))
			}
		}
		return
	}
	return
}

func (f *Force) httpPostCSV(url string, data string) (body []byte, err error) {
	body, err = f.httpPostWithContentType(url, data, "text/csv")
	if err == SessionExpiredError {
		f.RefreshSessionOrExit()
		return f.httpPostCSV(url, data)
	}
	return
}

func (f *Force) httpPostXML(url string, data string) (body []byte, err error) {
	body, err = f.httpPostWithContentType(url, data, "application/xml")
	if err == SessionExpiredError {
		f.RefreshSessionOrExit()
		return f.httpPostXML(url, data)
	}
	return
}

func (f *Force) httpPostJSON(url string, data string) (body []byte, err error) {
	body, err = f.httpPostWithContentType(url, data, "application/json")
	if err == SessionExpiredError {
		f.RefreshSessionOrExit()
		return f.httpPostJSON(url, data)
	}
	return
}

func (f *Force) httpPatchJSON(url string, data string) (body []byte, err error) {
	body, err = f.httpPatchWithContentType(url, data, "application/json")
	if err == SessionExpiredError {
		f.RefreshSessionOrExit()
		return f.httpPatchJSON(url, data)
	}
	return
}

func (f *Force) httpPatchWithContentType(url string, data string, contenttype string) (body []byte, err error) {
	body, err = f.httpPostPatchWithContentType(url, data, contenttype, "PATCH")
	return
}

func (f *Force) httpPostWithContentType(url string, data string, contenttype string) (body []byte, err error) {
	body, err = f.httpPostPatchWithContentType(url, data, contenttype, "POST")
	return
}

func (f *Force) httpPostPatchWithContentType(url string, data string, contenttype string, method string) (body []byte, err error) {
	rbody := data
	req, err := httpRequest(strings.ToUpper(method), url, bytes.NewReader([]byte(rbody)))
	if err != nil {
		return
	}

	req.Header.Add("X-SFDC-Session", f.Credentials.AccessToken)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.Credentials.AccessToken))
	req.Header.Add("Content-Type", contenttype)
	res, err := doRequest(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		err = SessionExpiredError
		return
	}
	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}
	if res.StatusCode/100 != 2 {
		if contenttype == "application/xml" {
			var fault LoginFault
			xml.Unmarshal(body, &fault)
			if fault.ExceptionCode == "InvalidSessionId" {
				err = SessionExpiredError
			}
		} else {
			var messages []ForceError
			json.Unmarshal(body, &messages)
			if messages != nil {
				err = errors.New(messages[0].Message)
			}
		}
		return
	} else if res.StatusCode == 204 {
		body = []byte("Patch command succeeded....")
	}
	return
}

func (f *Force) httpPost(url string, attrs map[string]string) (body []byte, err error, emessages []ForceError) {
	body, err, emessages = f.httpPostAttributes(url, attrs)
	if err == SessionExpiredError {
		f.RefreshSessionOrExit()
		return f.httpPost(url, attrs)
	}
	return
}

func (f *Force) httpPostAttributes(url string, attrs map[string]string) (body []byte, err error, emessages []ForceError) {
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
		err = SessionExpiredError
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
	body, err = f.httpPatchAttributes(url, attrs)
	if err == SessionExpiredError {
		f.RefreshSessionOrExit()
		return f.httpPatchAttributes(url, attrs)
	}
	return
}

func (f *Force) httpPatchAttributes(url string, attrs map[string]string) (body []byte, err error) {
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
		err = SessionExpiredError
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
	body, err = f.httpDeleteUrl(url)
	if err == SessionExpiredError {
		f.RefreshSessionOrExit()
		return f.httpDeleteUrl(url)
	}
	return
}

func (f *Force) httpDeleteUrl(url string) (body []byte, err error) {
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
		err = SessionExpiredError
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

func (result *ForceQueryResult) Update(other ForceQueryResult, force *Force) {
	result.Done = other.Done
	result.Records = append(result.Records, other.Records...)
	result.TotalSize = len(result.Records)
	result.NextRecordsUrl = fmt.Sprintf("%s%s", force.Credentials.InstanceUrl, other.NextRecordsUrl)
}

func doRequest(request *http.Request) (res *http.Response, err error) {
	client := &http.Client{}
	client.Timeout = time.Duration(Timeout) * time.Millisecond
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

func startLocalHttpServer(ch chan ForceSession) (port int, err error) {
	listener, err := net.Listen("tcp", ":3835")
	if err != nil {
		return
	}
	port = listener.Addr().(*net.TCPAddr).Port
	h := http.NewServeMux()
	h.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if r.Method == "POST" {
			var creds ForceSession
			creds.AccessToken = query.Get("access_token")
			creds.RefreshToken = query.Get("refresh_token")
			creds.InstanceUrl = query.Get("instance_url")
			creds.IssuedAt = query.Get("issued_at")
			creds.Scope = query.Get("scope")
			ch <- creds
			listener.Close()
		} else {
			io.WriteString(w, oauthCallbackHtml())
		}
	})
	go http.Serve(listener, h)
	return
}

func oauthCallbackHtml() string {
	return `
<!doctype html>
<html>
  <head>
	  <title>Force CLI OAuth Callback</title>
  </head>
  <body>
	  <h1>OAuth Callback</h1>
	  <p id="status">Status: Idle</p>
	  <script type="text/javascript">
	  window.onload = function() {
		  var url = window.location.href.replace('#', '?');
		  var req = new XMLHttpRequest();
		  var completeText = 'Complete! You may now close this window';
		  var errorText = 'Something went wrong!';
		  var statusEl = document.getElementById('status');

		  req.onreadystatechange=function() {

			  if(req.readyState==4 && req.status==200) {
				  statusEl.innerHTML = completeText;
			  } else {
				  statusEl.innerHTML = errorText;
			  }
		  }

		  req.open('POST', url, true);
		  req.setRequestHeader('Content-Type', 'text/plain');
		  statusEl.innerHTML = 'Status: Sending Auth...';
		  req.send();
	  }
	  </script>
  </body>
</html>`
}
