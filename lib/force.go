package lib

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ForceCLI/force/desktop"
	. "github.com/ForceCLI/force/error"
	"github.com/ForceCLI/force/lib/internal"
	"github.com/ForceCLI/force/lib/query"
)

// forceQuery is a package-level reference to query.Query, overridden in tests to simulate record pages.
var forceQuery = query.Query

var (
	ClientId    = "3MVG9ytVT1SanXDnX_hOa9Ys5NxVp5C26JlyQjwr.xTJtUqoKonXY.M8CcjoEknMrV4YUvPvXLiMyzI.Aw23C"
	RedirectUri = "http://localhost:3835/oauth/callback"
)

var Timeout int64 = 0
var CustomEndpoint = ``
var SessionExpiredError = errors.New("Session expired")
var APILimitExceededError = errors.New("API limit exceeded")
var APIDisabledForUser = errors.New("API disabled for user")
var PasswordExpiredError = errors.New("Password is expired")
var ClassNotFoundError = errors.New("class not found")
var MetricsNotFoundError = errors.New("metrics not found")
var DevHubOrgRequiredError = errors.New("Org must be a Dev Hub")

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
	retrier     *HttpRetrier
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
	// RefreshFunc can be set to support refreshing of additional session/refresh types than
	// available in RefreshMethod. It should return an error on failure.
	// On success, it is responsible for updating the given Force's credentials.
	// Usually this is done by calling Force.UpdateCredentials (which persist the changes)
	// or Force.CopyCredentialAuthFields (which modifies memory only).
	//
	// Note that RefreshMethod implementations use Force.UpdateCredentials.
	RefreshFunc func(*Force) error `json:"-"`
}

type OAuthError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type ForceSession struct {
	AccessToken    string `json:"access_token"`
	InstanceUrl    string `json:"instance_url"`
	IssuedAt       string `json:"issued_at"`
	Scope          string `json:"scope"`
	Id             string `json:"id"`
	ClientId       string
	RefreshToken   string
	ForceEndpoint  ForceEndpoint
	EndpointUrl    string `json:"endpoint_url"`
	UserInfo       *UserInfo
	SessionOptions *SessionOptions
}

type LoginFault struct {
	ExceptionCode    string `xml:"exceptionCode" json:"exceptionCode"`
	ExceptionMessage string `xml:"exceptionMessage" json:"exceptionMessage"`
}

func (lf LoginFault) Error() string {
	return fmt.Sprintf("%s: %s", lf.ExceptionCode, lf.ExceptionMessage)
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

type ForceErrors []ForceError

func (fs ForceErrors) Error() string {
	sb := strings.Builder{}
	for i, e := range fs {
		sb.WriteString(e.ErrorCode)
		sb.WriteString(": ")
		sb.WriteString(e.Message)
		if i < len(fs)-1 {
			sb.WriteString(", ")
		}
	}
	return sb.String()
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

type ForceSearchResult struct {
	SearchRecords []ForceRecord `json:"searchRecords"`
}

type ForceSobjectsResult struct {
	Encoding     string
	MaxBatchSize int
	Sobjects     []ForceSobject
}

type Result struct {
	Id      string        `json:"id"`
	Success bool          `json:"success"`
	Created bool          `json:"created"`
	Errors  []ResultError `json:"errors"`
}

type ResultError struct {
	StatusCode string   `json:"statusCode"`
	Message    string   `json:"message"`
	Fields     []string `json:"fields"`
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

type QueryPlan struct {
	Cardinality          int64           `json:"cardinality"`
	Fields               []string        `json:"fields"`
	LeadingOperationType string          `json:"leadingOperationType"`
	Notes                []QueryPlanNote `json:"notes"`
	RelativeCost         float64         `json:"relativeCost"`
	SObjectCardinality   int64           `json:"sobjectCardinality"`
	SObjectType          string          `json:"sobjectType"`
}

type QueryPlanNote struct {
	Description   string   `json:"description"`
	Fields        []string `json:"fields"`
	TableEnumOrId string   `json:"tableEnumOrId"`
}

type QueryPlanResult struct {
	Plans       []QueryPlan `json:"plans"`
	SourceQuery string      `json:"sourceQuery"`
}

func NewForce(creds *ForceSession) (force *Force) {
	force = new(Force)
	force.Credentials = creds
	force.Metadata = NewForceMetadata(force)
	force.Partner = NewForcePartner(force)
	force.retrier = DefaultRetrier()
	return
}

func NewForceWithRetrier(creds *ForceSession, retrier *HttpRetrier) (force *Force) {
	force = new(Force)
	force.Credentials = creds
	force.Metadata = NewForceMetadata(force)
	force.Partner = NewForcePartner(force)
	force.retrier = retrier
	return
}

func endpointUrl(endpoint ForceEndpoint) string {
	switch endpoint {
	case EndpointProduction:
		return "https://login.salesforce.com"
	case EndpointTest:
		return "https://test.salesforce.com"
	case EndpointPrerelease:
		return "https://prerelna1.pre.salesforce.com"
	case EndpointMobile1:
		return "https://mobile1.t.pre.salesforce.com"
	case EndpointCustom:
		fallthrough
	default:
		Log.Info("Deprecated use of CustomEndpoint")
		return CustomEndpoint
	}
}

func ForceSoapLogin(endpoint ForceEndpoint, username string, password string) (creds ForceSession, err error) {
	Log.Info("Deprecated call to ForceSoapLogin.  Use ForceSoapLoginAtEndpoint.")
	url := endpointUrl(endpoint)
	return ForceSoapLoginAtEndpoint(url, username, password)
}

func ForceSoapLoginAtEndpoint(endpoint string, username string, password string) (creds ForceSession, err error) {
	var surl string
	version := strings.Split(apiVersion, "v")[1]
	if endpoint == "" {
		return creds, fmt.Errorf("Unable to login with SOAP. Unknown endpoint type")
	}
	surl = fmt.Sprintf("%s/services/Soap/u/%s", endpoint, version)

	soap := NewSoap(surl, "", "")
	response, err := soap.ExecuteLogin(username, password)
	if err != nil {
		return creds, fmt.Errorf("Login failed: %w", err)
	}
	var result struct {
		SessionId    string `xml:"Body>loginResponse>result>sessionId"`
		Id           string `xml:"Body>loginResponse>result>userId"`
		Instance_url string `xml:"Body>loginResponse>result>serverUrl"`
	}
	var fault SoapFault
	if err = xml.Unmarshal(response, &fault); fault.Detail.ExceptionMessage != "" {
		return creds, fmt.Errorf("Login error: %s", fault.Detail.ExceptionCode+": "+fault.Detail.ExceptionMessage)
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
		AccessToken: result.SessionId,
		InstanceUrl: instanceUrl,
		EndpointUrl: endpoint,
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

func tokenURL(endpoint string) (tokenURL string) {
	return fmt.Sprintf("%s/services/oauth2/token", endpoint)
}

func (f *Force) refreshTokenURL() string {
	return tokenURL(f.Credentials.EndpointUrl)
}

func ForceLogin(endpoint ForceEndpoint) (creds ForceSession, err error) {
	Log.Info("Deprecated call to ForceLogin.  Use ForceLogin.")
	url := endpointUrl(endpoint)
	return ForceLoginAtEndpoint(url)
}

func ForceLoginAtEndpointWithPrompt(endpoint string, prompt string) (creds ForceSession, err error) {
	ch := make(chan ForceSession)
	port, err := startLocalHttpServer(ch)
	var url string

	Redir := RedirectUri

	url = fmt.Sprintf("%s/services/oauth2/authorize?response_type=token&client_id=%s&redirect_uri=%s&state=%d&prompt=%s", endpoint, ClientId, Redir, port, prompt)

	err = desktop.Open(url)
	creds = <-ch
	if creds.RefreshToken != "" {
		creds.SessionOptions = &SessionOptions{
			RefreshMethod: RefreshOauth,
		}
	}
	creds.EndpointUrl = endpoint
	creds.ClientId = ClientId
	return creds, err
}

func ForceLoginAtEndpoint(endpoint string) (creds ForceSession, err error) {
	return ForceLoginAtEndpointWithPrompt(endpoint, "login")
}

func (f *Force) GetCodeCoverage(classId string, className string) (err error) {
	url := fmt.Sprintf("%s/services/data/%s/query/?q=Select+Id+From+ApexClass+Where+Name+=+'%s'", f.Credentials.InstanceUrl, apiVersion, className)

	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
	if err != nil {
		return
	}
	var result ForceQueryResult
	json.Unmarshal(body, &result)

	if len(result.Records) == 0 {
		return ClassNotFoundError
	}

	classId = result.Records[0]["Id"].(string)
	url = fmt.Sprintf("%s/services/data/%s/tooling/query/?q=Select+Coverage,+NumLinesCovered,+NumLinesUncovered,+ApexTestClassId,+ApexClassorTriggerId+From+ApexCodeCoverage+Where+ApexClassorTriggerId='%s'", f.Credentials.InstanceUrl, apiVersion, classId)

	body, err = f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
	if err != nil {
		return
	}

	//var result ForceSobjectsResult
	json.Unmarshal(body, &result)

	if len(result.Records) == 0 {
		return MetricsNotFoundError
	}

	Log.Info(fmt.Sprintf("\n%d lines covered\n%d lines not covered\n", int(result.Records[0]["NumLinesCovered"].(float64)), int(result.Records[0]["NumLinesUncovered"].(float64))))
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
	return f.QueryDataPipeline(query)
}

func (f *Force) QueryDataPipeline(soql string) (results ForceQueryResult, err error) {
	aurl := fmt.Sprintf("%s/services/data/%s/tooling/query?q=%s", f.Credentials.InstanceUrl, apiVersion,
		url.QueryEscape(soql))

	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(aurl))
	if err != nil {
		return
	}
	json.Unmarshal(body, &results)

	return
}

func (f *Force) QueryDataPipelineJob(soql string) (results ForceQueryResult, err error) {
	aurl := fmt.Sprintf("%s/services/data/%s/tooling/query?q=%s", f.Credentials.InstanceUrl, apiVersion,
		url.QueryEscape(soql))

	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(aurl))
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

	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(aurl))
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

		body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(aurl))
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
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(aurl))
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

	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(aurl))
	if err != nil {
		return
	}
	json.Unmarshal(body, &bundles)

	return
}

func (f *Force) GetAuraBundleDefinition(id string) (definitions AuraDefinitionBundleResult, err error) {
	aurl := fmt.Sprintf("%s/services/data/%s/tooling/query?q=%s", f.Credentials.InstanceUrl, apiVersion,
		url.QueryEscape(fmt.Sprintf("SELECT Id, Source, AuraDefinitionBundleId, DefType, Format FROM AuraDefinition WHERE AuraDefinitionBundleId = '%s'", id)))

	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(aurl))
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
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
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
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
	if err != nil {
		return
	}
	json.Unmarshal(body, &sobject)
	return
}

func (f *Force) CancelableQueryAndSend(ctx context.Context, qs string, processor chan<- ForceRecord, options ...func(*QueryOptions)) error {
	defer func() {
		close(processor)
	}()

	qopts := f.legacyQueryOptions(qs, options...)

	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		err = forceQuery(func(records []query.Record) bool {
			for _, row := range records {
				select {
				case <-ctx.Done():
					return false
				case processor <- row.Fields:
				}
			}
			return true
		}, qopts...)
	}()
	select {
	case <-ctx.Done():
		select {
		case <-done:
		case <-time.After(1 * time.Second):
			// Wait briefly for goroutine to finish before returning and closing the processor channel
		}
		return fmt.Errorf("Query cancelled: %w", ctx.Err())
	case <-done:
	}
	if err != nil {
		return fmt.Errorf("Error querying records: %w", err)
	}
	return nil
}

func (f *Force) AbortableQueryAndSend(qs string, processor chan<- ForceRecord, abort <-chan bool, options ...func(*QueryOptions)) error {
	defer func() {
		close(processor)
	}()

	qopts := f.legacyQueryOptions(qs, options...)
	err := forceQuery(func(records []query.Record) bool {
		for _, row := range records {
			select {
			case <-abort:
				return false
			case processor <- row.Fields:
			}
		}
		return true
	}, qopts...)
	return err
}

func (f *Force) QueryAndSend(qs string, processor chan<- ForceRecord, options ...func(*QueryOptions)) error {
	defer func() {
		close(processor)
	}()

	qopts := f.legacyQueryOptions(qs, options...)
	err := query.Query(func(records []query.Record) bool {
		for _, row := range records {
			processor <- row.Raw
		}
		return true
	}, qopts...)
	return err
}

func (f *Force) Query(qs string, options ...func(*QueryOptions)) (ForceQueryResult, error) {
	qopts := f.legacyQueryOptions(qs, options...)
	result := ForceQueryResult{}
	records, err := query.Eager(qopts...)
	if err != nil {
		return result, err
	}
	result.Done = true
	result.TotalSize = len(records)
	result.Records = make([]ForceRecord, len(records))
	for i, r := range records {
		// NOTE: This will keep intact the subquery locator bug
		result.Records[i] = r.Raw
	}
	return result, err
}

func (f *Force) QueryOptions() []query.Option {
	instUrl := ""
	if f.Credentials != nil {
		instUrl = f.Credentials.InstanceUrl
	}
	return []query.Option{
		query.HttpGet(func(url string) ([]byte, error) {
			body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
			return body, err
		}),
		query.InstanceUrl(instUrl),
		query.ApiVersion(apiVersion),
	}
}

func (f *Force) legacyQueryOptions(qs string, options ...func(*QueryOptions)) []query.Option {
	qopts := f.QueryOptions()
	qopts = append(qopts, query.QS(qs))

	queryOptions := QueryOptions{}
	for _, option := range options {
		option(&queryOptions)
	}
	if queryOptions.QueryAll {
		qopts = append(qopts, query.All)
	}
	if queryOptions.IsTooling {
		qopts = append(qopts, query.Tooling)
	}
	return qopts
}

func (f *Force) Search(qs string) ([]ForceRecord, error) {
	url := "/search/?q=" + url.QueryEscape(qs)
	body, err := f.makeHttpRequestSync(NewRequest("GET").RestUrl(url))
	if err != nil {
		return nil, err
	}
	r := ForceSearchResult{}
	err = json.Unmarshal(body, &r)
	if err != nil {
		return nil, err
	}
	return r.SearchRecords, nil
}

func (f *Force) Get(url string) (object ForceRecord, err error) {
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &object)
	return
}

func (f *Force) GetLimits() (result map[string]ForceLimit, err error) {

	url := fmt.Sprintf("%s/services/data/%s/limits", f.Credentials.InstanceUrl, apiVersion)
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(body), &result)
	return

}

func (f *Force) GetPasswordStatus(id string) (result ForcePasswordStatusResult, err error) {
	url := fmt.Sprintf("%s/services/data/%s/sobjects/User/%s/password", f.Credentials.InstanceUrl, apiVersion, id)
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
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

	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
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

func (f *Force) QueryProfile(fields ...string) (results ForceQueryResult, err error) {

	url := fmt.Sprintf("%s/services/data/%s/tooling/query?q=Select+%s+From+Profile+Where+Id='%s'",
		f.Credentials.InstanceUrl,
		apiVersion,
		strings.Join(fields, ","),
		f.Credentials.UserInfo.ProfileId)

	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
	if err != nil {
		return
	}
	json.Unmarshal(body, &results)
	return
}

func (f *Force) QueryTraceFlags() (results ForceQueryResult, err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/query/?q=Select+Id,+DebugLevel.DeveloperName,++ApexCode,+ApexProfiling,+Callout,+CreatedDate,+Database,+ExpirationDate,+System,+TracedEntity.Name,+Validation,+Visualforce,+Workflow+From+TraceFlag+Order+By+ExpirationDate,TracedEntity.Name", f.Credentials.InstanceUrl, apiVersion)
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
	if err != nil {
		return
	}
	json.Unmarshal(body, &results)
	return
}

func (f *Force) QueryDefaultDebugLevel() (id string, err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/query/?q=Select+Id+From+DebugLevel+Where+DeveloperName+=+'Force_CLI'", f.Credentials.InstanceUrl, apiVersion)
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
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

func (f *Force) RetrieveLog(logId string) (result string, err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/ApexLog/%s/Body", f.Credentials.InstanceUrl, apiVersion, logId)
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
	result = string(body)
	return
}

func (f *Force) QueryLogs() (results ForceQueryResult, err error) {
	url := fmt.Sprintf("%s/services/data/%s/tooling/query/?q=Select+Id,+Application,+DurationMilliseconds,+Location,+LogLength,+LogUser.Name,+Operation,+Request,StartTime,+Status+From+ApexLog+Order+By+StartTime", f.Credentials.InstanceUrl, apiVersion)
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
	if err != nil {
		return
	}
	json.Unmarshal(body, &results)
	return
}

func (f *Force) RetrieveEventLogFile(elfId string) (result string, err error) {
	url := fmt.Sprintf("%s/services/data/%s/sobjects/EventLogFile/%s/LogFile", f.Credentials.InstanceUrl, apiVersion, elfId)
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
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
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
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
	body, err, _ := f.httpPost(aurl, attrs)

	if err != nil {
		return
	}
	json.Unmarshal(body, &result)

	return
}

func (f *Force) DescribeSObject(objecttype string) (result string, err error) {
	url := fmt.Sprintf("%s/services/data/%s/sobjects/%s/describe", f.Credentials.InstanceUrl, apiVersion, objecttype)
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(url))
	if err != nil {
		return
	}
	result = string(body)
	return
}

// DescribeSObjectResponse represents the response from a describe SObject request with headers
type DescribeSObjectResponse struct {
	Body        string
	ETag        string
	NotModified bool
}

// DescribeSObjectWithETag fetches SObject describe information with ETag support for conditional requests
func (f *Force) DescribeSObjectWithETag(objecttype string, ifNoneMatch string) (result *DescribeSObjectResponse, err error) {
	url := fmt.Sprintf("%s/services/data/%s/sobjects/%s/describe", f.Credentials.InstanceUrl, apiVersion, objecttype)

	req := NewRequest("GET").AbsoluteUrl(url).ReadResponseBody()
	if ifNoneMatch != "" {
		req = req.WithHeader("If-None-Match", ifNoneMatch)
	}

	response, err := f.ExecuteRequest(req)
	if err != nil {
		return nil, err
	}

	result = &DescribeSObjectResponse{
		Body:        string(response.ReadResponseBody),
		ETag:        response.HttpResponse.Header.Get("ETag"),
		NotModified: response.HttpResponse.StatusCode == 304,
	}

	return result, nil
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
	if f.Credentials.UserInfo == nil {
		userInfo, err := f.UserInfo()
		if err != nil {
			return nil, err
		}
		f.Credentials.UserInfo = &userInfo
	}
	me, err = f.GetRecord("User", f.Credentials.UserInfo.UserId)
	return
}

// Prepend https schema and instance to URL
func (f *Force) qualifyUrl(url string) string {
	return fmt.Sprintf("%s/%s", f.Credentials.InstanceUrl, strings.TrimLeft(url, "/"))
}

func (f *Force) GetAbsoluteBytes(url string) (result []byte, err error) {
	return f.makeHttpRequestSync(NewRequest("GET").RootUrl(url))
}

func (f *Force) GetAbsolute(url string) (string, error) {
	data, err := f.GetAbsoluteBytes(url)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *Force) Explain(query string) (QueryPlanResult, error) {
	var result QueryPlanResult
	url := "/query/?explain=" + url.QueryEscape(query)
	body, err := f.makeHttpRequestSync(NewRequest("GET").RestUrl(url))
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(body, &result)
	return result, err
}

func (f *Force) GetREST(url string) (result string, err error) {
	body, err := f.makeHttpRequestSync(NewRequest("GET").RestUrl(url))
	return string(body), err
}

func (f *Force) PostPatchAbsolute(url string, content string, method string) (result string, err error) {
	if method == "POST" {
		return f.PostAbsolute(url, content)
	} else if method == "PUT" {
		return f.PutAbsolute(url, content)
	} else {
		return f.PatchAbsolute(url, content)
	}
}

func (f *Force) PostPatchREST(url string, content string, method string) (result string, err error) {
	if method == "POST" {
		return f.PostREST(url, content)
	} else if method == "PUT" {
		return f.PutREST(url, content)
	} else {
		return f.PatchREST(url, content)
	}
}

func (f *Force) PostAbsolute(url string, content string) (string, error) {
	qualifiedUrl := f.qualifyUrl(url)
	body, err := f.httpPostPatchWithRetry(qualifiedUrl, content, ContentTypeJson, HttpMethodPost)
	return string(body), err
}

func (f *Force) PostREST(url string, content string) (result string, err error) {
	fullUrl := fullRestUrl(url)
	return f.PostAbsolute(fullUrl, content)
}

func (f *Force) PatchAbsolute(url string, content string) (result string, err error) {
	qualifiedUrl := f.qualifyUrl(url)
	body, err := f.httpPostPatchWithRetry(qualifiedUrl, content, ContentTypeJson, HttpMethodPatch)
	return string(body), err
}

func (f *Force) PatchREST(url string, content string) (result string, err error) {
	fullUrl := fullRestUrl(url)
	return f.PatchAbsolute(fullUrl, content)
}

func (f *Force) PutAbsolute(url string, content string) (result string, err error) {
	qualifiedUrl := f.qualifyUrl(url)
	body, err := f.httpPostPatchWithRetry(qualifiedUrl, content, ContentTypeJson, HttpMethodPut)
	return string(body), err
}

func (f *Force) PutREST(url string, content string) (result string, err error) {
	fullUrl := fullRestUrl(url)
	return f.PutAbsolute(fullUrl, content)
}

func (f *Force) getForceResult(url string) (results ForceQueryResult, err error) {
	furl := fmt.Sprintf("%s%s", f.Credentials.InstanceUrl, url)
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(furl))
	if err != nil {
		return
	}
	json.Unmarshal(body, &results)
	return
}

func (f *Force) setHttpInputAuth(input *httpRequestInput) *httpRequestInput {
	input.Headers["X-SFDC-Session"] = f.Credentials.AccessToken
	input.Headers["Authorization"] = fmt.Sprintf("Bearer %s", f.Credentials.AccessToken)
	return input
}

func (f *Force) makeHttpRequestSync(req *Request) (body []byte, err error) {
	req = req.ReadResponseBody()
	resp, err := f.ExecuteRequest(req)
	if err != nil {
		return nil, err
	}
	return resp.ReadResponseBody, nil
}

func (f *Force) makeHttpRequest(input *httpRequestInput) error {
	for {
		res, err := f._makeHttpRequestWithoutRetry(input)

		if !input.retrier.shouldRetry(res, err) {
			if err != nil {
				return err
			}
			cberr := input.Callback(res)
			return cberr
		}

		if err == SessionExpiredError {
			// If refreshing causes an error, we should return the original error.
			// Otherwise we end up retrying even if we haven't updated auth.
			if refreshedErr := f.RefreshSession(); refreshedErr != nil {
				return err
			}
			f.setHttpInputAuth(input)
		} else if input.retrier.backoffDelay > 0 && input.retrier.attempt < input.retrier.maxAttempts {
			time.Sleep(time.Duration(rand.Int63n(int64(input.retrier.backoffDelay / time.Nanosecond))))
		}

	}
}

func (f *Force) _makeHttpRequestWithoutRetry(input *httpRequestInput) (*http.Response, error) {
	req, err := httpRequestWithHeaders(input.Method, input.Url, input.Headers, input.Body)
	if err != nil {
		return nil, err
	}
	res, err := doRequest(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode/100 == 2 || res.StatusCode == 304 {
		return res, nil
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return res, f._coerceHttpError(res, body)
}

// Custom HTTP error handling, which is not obvious or trivial because Salesforce returns
// many shapes and forms of errors.
//
// The returned error is never nil.
//
// In general, a 401 or 403 will be treated as SessionExpiredError,
// but there are some special cases. We handle InvalidSessionId in XML errors explicitly;
// we also handle rate limit issues, which are a 403, explicitly, so we don't keep refreshing.
// So we pull these out and handle them in this if/else;
// if we don't find them, we create a fallback error based on the actual response shape.
// Then if it's a 401/403, treat the session expired (discard the fallback error);
// Finally, return the fallback error.
func (f *Force) _coerceHttpError(res *http.Response, body []byte) error {
	var fallbackErr error
	sessionExpired := res.StatusCode == 401 || res.StatusCode == 403
	if strings.HasPrefix(res.Header.Get("Content-Type"), string(ContentTypeXml)) {
		var fault LoginFault
		if err := internal.XmlUnmarshal(body, &fault); err != nil {
			return err
		}
		if fault.ExceptionCode == "InvalidSessionId" {
			return SessionExpiredError
		} else if fault.ExceptionCode != "" {
			return fault
		}
	} else {
		var errors ForceErrors
		if err := internal.JsonUnmarshal(body, &errors); err != nil && !sessionExpired {
			return fmt.Errorf("unhandled error: %d %s", res.StatusCode, string(body))
		}
		if len(errors) > 0 && errors[0].ErrorCode == "REQUEST_LIMIT_EXCEEDED" {
			return APILimitExceededError
		} else if len(errors) > 0 && errors[0].ErrorCode == "API_CURRENTLY_DISABLED" {
			return APIDisabledForUser
		} else if len(errors) > 0 && errors[0].ErrorCode == "INVALID_OPERATION_WITH_EXPIRED_PASSWORD" {
			return PasswordExpiredError
		} else if len(errors) > 0 {
			fallbackErr = errors
		}
	}

	if sessionExpired {
		return SessionExpiredError
	}
	if fallbackErr == nil {
		fallbackErr = fmt.Errorf("unhandled error: %d %s", res.StatusCode, string(body))
	}
	return fallbackErr
}

func (f *Force) httpPostPatchWithRetry(url string, rbody string, contenttype ContentType, method HttpMethod, requestOptions ...func(*http.Request)) ([]byte, error) {
	retrier := f.GetRetrier()

	for {
		res, err := f.httpPostPatch(url, rbody, contenttype, method, requestOptions...)
		var body []byte

		if err == nil {
			body, err = f.readResponseBody(res)
		}

		if err == nil {
			return body, nil
		}
		if !retrier.shouldRetry(res, err) {
			return nil, err
		}

		if err == SessionExpiredError {
			if refreshedErr := f.RefreshSession(); refreshedErr != nil {
				return body, err
			}
		} else if retrier.backoffDelay > 0 && retrier.attempt < retrier.maxAttempts {
			time.Sleep(time.Duration(rand.Int63n(int64(retrier.backoffDelay / time.Nanosecond))))
		}

	}
}

func (f *Force) httpPostPatch(url string, rbody string, contenttype ContentType, method HttpMethod, requestOptions ...func(*http.Request)) (*http.Response, error) {
	req, err := httpRequest(string(method), url, strings.NewReader(rbody))
	if err != nil {
		return nil, err
	}

	for _, option := range requestOptions {
		option(req)
	}

	req.Header.Add("X-SFDC-Session", f.Credentials.AccessToken)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.Credentials.AccessToken))
	req.Header.Add("Content-Type", string(contenttype))

	res, err := doRequest(req)

	if res.StatusCode == 401 {
		return nil, SessionExpiredError
	}

	return res, err
}

func (f *Force) readResponseBody(res *http.Response) ([]byte, error) {
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode/100 == 2 {
		return body, nil
	}
	if res.StatusCode == 204 {
		return []byte("Patch command succeeded...."), nil
	}

	return body, f._coerceHttpError(res, body)
}

func (f *Force) httpPost(url string, attrs map[string]string) (body []byte, err error, emessages []ForceError) {
	body, err, emessages = f.httpPostAttributes(url, attrs)
	if err == SessionExpiredError {
		err = f.RefreshSession()
		if err != nil {
			return
		}
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
		err = f.RefreshSession()
		if err != nil {
			return
		}
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
		err = f.RefreshSession()
		if err != nil {
			return
		}
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

func startLocalHttpServer(ch chan ForceSession) (port int, err error) {
	listener, err := net.Listen("tcp", ":3835")
	if err != nil {
		return
	}
	port = listener.Addr().(*net.TCPAddr).Port
	h := http.NewServeMux()
	h.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("error") != "" {
			oauthCallbackError(w, query.Get("error"), query.Get("error_description"))
			return
		}
		if r.Method != "POST" {
			io.WriteString(w, oauthCallbackHtml())
			return
		}
		var creds ForceSession
		creds.AccessToken = query.Get("access_token")
		creds.RefreshToken = query.Get("refresh_token")
		creds.InstanceUrl = query.Get("instance_url")
		creds.IssuedAt = query.Get("issued_at")
		creds.Scope = query.Get("scope")
		ch <- creds
		listener.Close()
	})
	go http.Serve(listener, h)
	return
}

func oauthCallbackError(w io.Writer, code, description string) {
	tmpl := `
<!doctype html>
<html>
  <head>
	  <title>Force CLI OAuth Callback</title>
  </head>
  <body>
	  <h1>OAuth Error</h1>
	  <p id="status">Status: {{.Code}}</p>
	  <p>{{.Description}}</p>
  </body>
</html>`
	t, err := template.New("error-page").Parse(tmpl)
	if err != nil {
		ErrorAndExit("Unable to parse template: " + err.Error())
	}

	err = t.Execute(w, struct {
		Code        string
		Description string
	}{
		Code:        code,
		Description: description,
	})
	if err != nil {
		ErrorAndExit("Unable to render template: " + err.Error())
	}
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

func (force *Force) GetMetadata() *ForceMetadata {
	return force.Metadata
}

func (force *Force) GetCredentials() *ForceSession {
	return force.Credentials
}

func (force *Force) GetPartner() *ForcePartner {
	return force.Partner
}

func (force *Force) GetRetrier() *HttpRetrier {
	if force.retrier == nil {
		return &HttpRetrier{}
	}

	return NewHttpRetrier(force.retrier.maxAttempts, force.retrier.backoffDelay, force.retrier.retryOnErrors...)
}

func (force *Force) WithRetrier(retrier *HttpRetrier) *Force {
	return &Force{
		Credentials: force.GetCredentials(),
		Metadata:    force.GetMetadata(),
		Partner:     force.GetPartner(),
		retrier:     retrier,
	}
}
