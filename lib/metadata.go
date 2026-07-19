package lib

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ForceCLI/inflect"

	. "github.com/ForceCLI/force/error"
)

type BigObject struct {
	DeploymentStatus string
	Label            string
	PluralLabel      string
	Fields           []BigObjectField
}

var AlreadyCompletedError = errors.New("Deployment already completed")

var preserveZip bool

// SetPreserveZip controls whether retrieved zip payloads are also saved to
// inbound.zip in the working directory.
func SetPreserveZip(value bool) {
	preserveZip = value
}

func (bo *BigObject) ToXml() string {
	soap := `<?xml version="1.0" encoding="UTF-8"?>
		<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
			<deploymentStatus>%s</deploymentStatus>
			<label>%s</label>
			<pluralLabel>%s</pluralLabel>
			%s
		</CustomObject>
	`
	textfieldsoap := `
			<fields>
				<fullName>%s__c</fullName>
				<label>%s</label>
				<length>%d</length>
				<type>Text</type>
			</fields>
	`
	datetimefieldsoap := `
			<fields>
				<fullName>%s__c</fullName>
				<label>%s</label>
				<type>DateTime</type>
			</fields>
	`
	lookupfieldsoap := `
			<fields>
				<fullName>%s__c</fullName>
				<label>%s</label>
				<referenceTo>%s</referenceTo>
				<relationshipName>%s</relationshipName>
				<type>Lookup</type>
			</fields>
	`
	fieldsoap := ``
	for _, field := range bo.Fields {
		switch strings.ToLower(field.Type) {
		case "datetime":
			fieldsoap += fmt.Sprintf(datetimefieldsoap, field.FullName, field.Label)
		case "text":
			fieldsoap += fmt.Sprintf(textfieldsoap, field.FullName, field.Label, field.Length)
		case "lookup":
			fieldsoap += fmt.Sprintf(lookupfieldsoap, field.FullName, field.Label, field.ReferenceTo, field.RelationshipName)
		}
	}
	return fmt.Sprintf(soap, bo.DeploymentStatus, bo.Label, bo.PluralLabel, fieldsoap)
}

type BigObjectField struct {
	FullName         string
	Label            string
	Length           int
	ReferenceTo      string
	RelationshipName string
	Type             string
}

type ForceConnectedApps []ForceConnectedApp

type ForceConnectedApp struct {
	Name string `xml:"fullName"`
	Id   string `xml:"id"`
	Type string `xml:"type"`
}

type ComponentFailure struct {
	Changed       bool   `xml:"changed"`
	ColumnNumber  int    `xml:"columnNumber"`
	ComponentType string `xml:"componentType"`
	Created       bool   `xml:"created"`
	Deleted       bool   `xml:"deleted"`
	FileName      string `xml:"fileName"`
	FullName      string `xml:"fullName"`
	LineNumber    int    `xml:"lineNumber"`
	Problem       string `xml:"problem"`
	ProblemType   string `xml:"problemType"`
	Success       bool   `xml:"success"`
}

type ComponentSuccess struct {
	Changed       bool   `xml:"changed"`
	ComponentType string `xml:"componentType"`
	Created       bool   `xml:"created"`
	Deleted       bool   `xml:"deleted"`
	FileName      string `xml:"fileName"`
	FullName      string `xml:"fullName"`
	Id            string `xml:"id"`
	Success       bool   `xml:"success"`
}

type TestFailure struct {
	Message    string  `xml:"message"`
	Name       string  `xml:"name"`
	MethodName string  `xml:"methodName"`
	StackTrace string  `xml:"stackTrace"`
	Time       float32 `xml:"time"`
}

type TestSuccess struct {
	Name       string  `xml:"name"`
	MethodName string  `xml:"methodName"`
	Time       float32 `xml:"time"`
}

type CodeCoverageWarning struct {
	Name    string `xml:"name"`
	Message string `xml:"message"`
}

type CodeCoverage struct {
	Id                     string               `xml:"id"`
	LocationsNotCovered    []LocationNotCovered `xml:"locationsNotCovered"`
	Name                   string               `xml:"name"`
	Namespace              string               `xml:"namespace"`
	Type                   string               `xml:"type"`
	NumLocations           int                  `xml:"numLocations"`
	NumLocationsNotCovered int                  `xml:"numLocationsNotCovered"`
}

type LocationNotCovered struct {
	Column        int     `xml:"column"`
	Line          int     `xml:"line"`
	NumExecutions int     `xml:"numExecutions"`
	Time          float32 `xml:"time"`
}

type RunTestResult struct {
	NumberOfFailures     int                   `xml:"numFailures"`
	NumberOfTestsRun     int                   `xml:"numTestsRun"`
	TotalTime            float32               `xml:"totalTime"`
	TestFailures         []TestFailure         `xml:"failures"`
	TestSuccesses        []TestSuccess         `xml:"successes"`
	CodeCoverageWarnings []CodeCoverageWarning `xml:"codeCoverageWarnings"`
	CodeCoverage         []CodeCoverage        `xml:"codeCoverage"`
}

type ComponentDetails struct {
	ComponentSuccesses []ComponentSuccess `xml:"componentSuccesses"`
	ComponentFailures  []ComponentFailure `xml:"componentFailures"`
	RunTestResult      RunTestResult      `xml:"runTestResult"`
}

type ForceCheckDeploymentStatusResult struct {
	CheckOnly                bool             `xml:"checkOnly"`
	CompletedDate            time.Time        `xml:"completedDate"`
	CreatedDate              time.Time        `xml:"createdDate"`
	CreatedByName            string           `xml:"createdByName"`
	Details                  ComponentDetails `xml:"details"`
	Done                     bool             `xml:"done"`
	Id                       string           `xml:"id"`
	ErrorMessage             string           `xml:"errorMessage"`
	ErrorStatusCode          string           `xml:"errorStatusCode"`
	LastModifiedDate         time.Time        `xml:"lastModifiedDate"`
	NumberComponentErrors    int              `xml:"numberComponentErrors"`
	NumberComponentsDeployed int              `xml:"numberComponentsDeployed"`
	NumberComponentsTotal    int              `xml:"numberComponentsTotal"`
	NumberTestErrors         int              `xml:"numberTestErrors"`
	NumberTestsCompleted     int              `xml:"numberTestsCompleted"`
	NumberTestsTotal         int              `xml:"numberTestsTotal"`
	RollbackOnError          bool             `xml:"rollbackOnError"`
	Status                   string           `xml:"status"`
	StateDetail              string           `xml:"stateDetail"`
	Success                  bool             `xml:"success"`
}

type ForceCancelDeployResult struct {
	Done bool   `xml:"done"`
	Id   string `xml:"id"`
}

type ForceMetadataDeployProblem struct {
	Changed     bool   `xml:"changed"`
	Created     bool   `xml:"created"`
	Deleted     bool   `xml:"deleted"`
	Filename    string `xml:"fileName"`
	Name        string `xml:"fullName"`
	Problem     string `xml:"problem"`
	ProblemType string `xml:"problemType"`
	Success     bool   `xml:"success"`
}

type ForceMetadataQueryElement struct {
	Name    []string
	Members []string
}

type ForceMetadataQuery []ForceMetadataQueryElement

type ForceMetadataFiles map[string][]byte

type ForceMetadata struct {
	ApiVersion string
	Force      *Force
}

type ForceDeployOptions struct {
	XMLName           xml.Name `xml:"deployOptions"`
	AllowMissingFiles bool     `xml:"allowMissingFiles"`
	AutoUpdatePackage bool     `xml:"autoUpdatePackage"`
	CheckOnly         bool     `xml:"checkOnly"`
	IgnoreWarnings    bool     `xml:"ignoreWarnings"`
	PerformRetrieve   bool     `xml:"performRetrieve"`
	PurgeOnDelete     bool     `xml:"purgeOnDelete"`
	RollbackOnError   bool     `xml:"rollbackOnError"`
	TestLevel         string   `xml:"testLevel,omitempty"`
	RunTests          []string `xml:"runTests"`
	SinglePackage     bool     `xml:"singlePackage"`
}

/*
These structs define which options are available and which are

	required for the various field types you can create. Reflection
	is used to leverage these structs in validating options when creating
	a custom field.
*/
type GeolocationFieldRequired struct {
	DisplayLocationInDecimal bool `xml:"displayLocationInDecimal"`
	Scale                    int  `xml:"scale"`
}

type GeolocationField struct {
	DsiplayLocationInDecimal bool   `xml:"displayLocationInDecimal"`
	Required                 bool   `xml:"required"`
	Scale                    int    `xml:"scale"`
	Description              string `xml:"description"`
	HelpText                 string `xml:"inlineHelpText"`
}

type AutoNumberFieldRequired struct {
	StartingNumber int    `xml:"startingNumber"`
	DisplayFormat  string `xml:"displayFormat"`
}

type AutoNumberField struct {
	Label          string `xml:"label"`
	StartingNumber int    `xml:"startingNumber"`
	DisplayFormat  string `xml:"displayFormat"`
	Description    string `xml:"description"`
	HelpText       string `xml:"inlineHelpText"`
	ExternalId     bool   `xml:"externalId"`
}

type FloatFieldRequired struct {
	Precision int `xml:"precision"`
	Scale     int `xml:"scale"`
}

type FloatField struct {
	Label                string `xml:"label"`
	Description          string `xml:"description"`
	HelpText             string `xml:"inlineHelpText"`
	Unique               bool   `xml:"unique"`
	ExternalId           bool   `xml:"externalId"`
	DefaultValue         uint   `xml:"defaultValue"`
	Precision            int    `xml:"precision"`
	Scale                int    `xml:"scale"`
	Formula              string `xml:"formula"`
	FormulaTreatBlanksAs string `xml:"formulaTreatBlanksAs"`
}

type NumberFieldRequired struct {
	Precision int `xml:"precision"`
	Scale     int `xml:"scale"`
}

type NumberField struct {
	Label                string `xml:"label"`
	Description          string `xml:"description"`
	HelpText             string `xml:"inlineHelpText"`
	Unique               bool   `xml:"unique"`
	ExternalId           bool   `xml:"externalId"`
	DefaultValue         uint   `xml:"defaultValue"`
	Formula              string `xml:"formula"`
	FormulaTreatBlanksAs string `xml:"formulaTreatBlanksAs"`
	Precision            int    `xml:"precision"`
	Scale                int    `xml:"scale"`
}

type DatetimeFieldRequired struct {
}

type DatetimeField struct {
	Label                string    `xml:"label"`
	Description          string    `xml:"description"`
	HelpText             string    `xml:"inlineHelpText"`
	DefaultValue         time.Time `xml:"defaultValue"`
	Required             bool      `xml:"required"`
	Formula              string    `xml:"formula"`
	FormulaTreatBlanksAs string    `xml:"formulaTreatBlanksAs"`
}

type PicklistValue struct {
	FullName string `xml:"fullName"`
	Default  bool   `xml:"default"`
}

type PicklistFieldRequired struct {
	Picklist []PicklistValue `xml:"picklist>picklistValues"`
}

type PicklistField struct {
	Label    string          `xml:"label"`
	Picklist []PicklistValue `xml:"picklist>picklistValues"`
}

type BoolFieldRequired struct {
	DefaultValue bool `xml:"defaultValue"`
}

type BoolField struct {
	Label                string `xml:"label"`
	Description          string `xml:"description"`
	HelpText             string `xml:"inlineHelpText"`
	DefaultValue         bool   `xml:"defaultValue"`
	Formula              string `xml:"formula"`
	FormulaTreatBlanksAs string `xml:"formulaTreatBlanksAs"`
}

type DescribeMetadataObject struct {
	ChildXmlNames []string `xml:"childXmlNames"`
	DirectoryName string   `xml:"directoryName"`
	InFolder      bool     `xml:"inFolder"`
	MetaFile      bool     `xml:"metaFile"`
	Suffix        string   `xml:"suffix"`
	XmlName       string   `xml:"xmlName"`
}

type MetadataDescribeResult struct {
	NamespacePrefix    string                   `xml:"organizationNamespace"`
	PartialSaveAllowed bool                     `xml:"partialSaveAllowed"`
	TestRequired       bool                     `xml:"testRequired"`
	MetadataObjects    []DescribeMetadataObject `xml:"metadataObjects"`
}

type MetadataDescribeValueTypeResult struct {
	ValueTypeFields []MetadataValueTypeField `xml:"result"`
}

type MetadataValueTypeField struct {
	//Fields 						MetadataValueTypeField
	ForeignKeyDomain string
	IsForeignKey     bool
	IsNameField      bool
	MinOccurs        int
	Name             string
	SoapType         string
}

type MDFileProperties struct {
	CreatedById        string    `xml:"createdById"`
	CreateByName       string    `xml:"createdByName"`
	CreateDate         time.Time `xml:"createdDate"`
	FileName           string    `xml:"fileName"`
	FullName           string    `xml:"fullName"`
	Id                 string    `xml:"id"`
	LastModifiedById   string    `xml:"lastModifiedById"`
	LastModifiedByName string    `xml:"lastModifiedByName"`
	LastModifedDate    time.Time `xml:"lastModifiedDate"`
	ManageableState    string    `xml:"manageableState"`
	NamespacePrefix    string    `xml:"namespacePrefix"`
	Type               string    `xml:"type"`
}

type ListMetadataResponse struct {
	Result []MDFileProperties `xml:"result"`
}

type EncryptedFieldRequired struct {
	Length   int    `xml:"length"`
	MaskType string `xml:"maskType"`
	MaskChar string `xml:"maskChar"`
}

type EncryptedField struct {
	Label       string `xml:"label"`
	Name        string `xml:"fullName"`
	Required    bool   `xml:"required"`
	Length      int    `xml:"length"`
	Description string `xml:"description"`
	HelpText    string `xml:"inlineHelpText"`
	MaskType    string `xml:"maskType"`
	MaskChar    string `xml:"maskChar"`
}

type StringFieldRequired struct {
	Length int `xml:"length"`
}

type StringField struct {
	Label                string `xml:"label"`
	Name                 string `xml:"fullName"`
	Required             bool   `xml:"required"`
	Length               int    `xml:"length"`
	Description          string `xml:"description"`
	HelpText             string `xml:"inlineHelpText"`
	Unique               bool   `xml:"unique"`
	CaseSensitive        bool   `xml:"caseSensitive"`
	ExternalId           bool   `xml:"externalId"`
	DefaultValue         string `xml:"defaultValue"`
	Formula              string `xml:"formula"`
	FormulaTreatBlanksAs string `xml:"formulaTreatBlanksAs"`
}

type PhoneFieldRequired struct {
}

type PhoneField struct {
	Label        string `xml:"label"`
	Name         string `xml:"fullName"`
	Required     bool   `xml:"required"`
	Description  string `xml:"description"`
	HelpText     string `xml:"inlineHelpText"`
	DefaultValue string `xml:"defaultValue"`
}

type UrlFieldRequired struct {
}

type UrlField struct {
	Label                string `xml:"label"`
	Name                 string `xml:"fullName"`
	Required             bool   `xml:"required"`
	Description          string `xml:"description"`
	HelpText             string `xml:"inlineHelpText"`
	DefaultValue         string `xml:"defaultValue"`
	Formula              string `xml:"formula"`
	FormulaTreatBlanksAs string `xml:"formulaTreatBlanksAs"`
}

type EmailFieldRequired struct {
}

type EmailField struct {
	Label string `xml:"label"`
}

type TextAreaFieldRequired struct {
}

type TextAreaField struct {
	Label        string `xml:"label"`
	Name         string `xml:"fullName"`
	Required     bool   `xml:"required"`
	Description  string `xml:"description"`
	HelpText     string `xml:"inlineHelpText"`
	DefaultValue string `xml:"defaultValue"`
}

type LongTextAreaFieldRequired struct {
	Length       int `xml:"length"`
	VisibleLines int `xml:"visibleLines"`
}

type LongTextAreaField struct {
	Label        string `xml:"label"`
	Name         string `xml:"fullName"`
	Required     bool   `xml:"required"`
	Description  string `xml:"description"`
	HelpText     string `xml:"inlineHelpText"`
	DefaultValue string `xml:"defaultValue"`
	Length       int    `xml:"length"`
	VisibleLines int    `xml:"visibleLines"`
}

type RichTextAreaFieldRequired struct {
	Length       int `xml:"length"`
	VisibleLines int `xml:"visibleLines"`
}

type RichTextAreaField struct {
	Label        string `xml:"label"`
	Name         string `xml:"fullName"`
	Required     bool   `xml:"required"`
	Description  string `xml:"description"`
	HelpText     string `xml:"inlineHelpText"`
	Length       int    `xml:"length"`
	VisibleLines int    `xml:"visibleLines"`
}

type LookupFieldRequired struct{}

type LookupField struct {
	Label             string `xml:"label"`
	ReferenceTo       string `xml:"referenceTo"`
	RelationshipLabel string `xml:"relationshipLabel"`
	RelationshipName  string `xml:"relationshipName"`
}

type MasterDetailRequired struct{}

type MasterDetail struct {
	Label             string `xml:"label"`
	ReferenceTo       string `xml:"referenceTo"`
	RelationshipLabel string `xml:"relationshipLabel"`
	RelationshipName  string `xml:"relationshipName"`
}

var (
	metadataType string
)

// Example of how to use Go's reflection
// Print the attributes of a Data Model
func getAttributes(m interface{}) map[string]reflect.StructField {
	typ := reflect.TypeOf(m)
	// if a pointer to a struct is passed, get the type of the dereferenced object
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// create an attribute data structure as a map of types keyed by a string.
	attrs := make(map[string]reflect.StructField)
	// Only structs are supported so return an empty result if the passed object
	// isn't a struct
	if typ.Kind() != reflect.Struct {
		fmt.Printf("%v type can't have attributes inspected\n", typ.Kind())
		return attrs
	}

	// loop through the struct's fields and set the map
	for i := 0; i < typ.NumField(); i++ {
		p := typ.Field(i)
		if !p.Anonymous {
			attrs[strings.ToLower(p.Name)] = p
		}
	}

	return attrs
}

func ValidateOptionsAndDefaults(typ string, fields map[string]reflect.StructField, requiredDefaults reflect.Value, options map[string]string) (newOptions map[string]string, err error) {
	newOptions = make(map[string]string)
	// validate optional attributes
	for name, value := range options {
		field, ok := fields[strings.ToLower(name)]
		if !ok {
			ErrorAndExit(fmt.Sprintf("validation error: %s:%s is not a valid option for field type %s", name, value, typ))
		} else {
			newOptions[field.Tag.Get("xml")] = options[name]
		}
	}

	// validate required attributes
	s := requiredDefaults
	tod := s.Type()
	for i := 0; i < s.NumField(); i++ {
		_, ok := options[inflect.CamelizeDownFirst(tod.Field(i).Name)]
		if !ok {
			switch s.Field(i).Type().Name() {
			case "int":
				newOptions[tod.Field(i).Tag.Get("xml")] = strconv.Itoa(s.Field(i).Interface().(int))
				break
			case "bool":
				if typ == "bool" {
					if _, ok = options["formula"]; ok {
						if tod.Field(i).Tag.Get("xml") == "defaultValue" {
							break
						}
					}
				} //else {
				newOptions[tod.Field(i).Tag.Get("xml")] = strconv.FormatBool(s.Field(i).Interface().(bool))
				//}
				break
			case "string":
				newOptions[tod.Field(i).Tag.Get("xml")] = s.Field(i).Interface().(string)
				break
			}
		} else {
			newOptions[tod.Field(i).Tag.Get("xml")] = options[inflect.CamelizeDownFirst(tod.Field(i).Name)]
		}
	}
	return newOptions, err
}

func (fm *ForceMetadata) ValidateFieldOptions(typ string, options map[string]string) (newOptions map[string]string, err error) {

	newOptions = make(map[string]string)
	var attrs map[string]reflect.StructField
	var s reflect.Value

	switch strings.ToLower(typ) {
	case "picklist":
		attrs = getAttributes(&PicklistField{})
		s = reflect.ValueOf(&PicklistFieldRequired{}).Elem()
		break
	case "phone":
		attrs = getAttributes(&PhoneField{})
		s = reflect.ValueOf(&PhoneFieldRequired{}).Elem()
		break
	case "email":
		attrs = getAttributes(&StringField{})
		s = reflect.ValueOf(&EmailFieldRequired{}).Elem()
		break
	case "url":
		attrs = getAttributes(&UrlField{})
		s = reflect.ValueOf(&UrlFieldRequired{}).Elem()
		break
	case "encryptedtext":
		attrs = getAttributes(&EncryptedField{})
		s = reflect.ValueOf(&EncryptedFieldRequired{175, "all", "asterisk"}).Elem()
		break
	case "string", "text":
		attrs = getAttributes(&StringField{})
		if _, ok := options["formula"]; ok {
			s = reflect.ValueOf(&StringFieldRequired{}).Elem()
		} else {
			s = reflect.ValueOf(&StringFieldRequired{255}).Elem()
		}
		break
	case "textarea":
		attrs = getAttributes(&TextAreaField{})
		s = reflect.ValueOf(&TextAreaFieldRequired{}).Elem()
		break
	case "longtextarea":
		attrs = getAttributes(&LongTextAreaField{})
		s = reflect.ValueOf(&LongTextAreaFieldRequired{32768, 5}).Elem()
		break
	case "richtextarea":
		attrs = getAttributes(&RichTextAreaField{})
		s = reflect.ValueOf(&RichTextAreaFieldRequired{32768, 5}).Elem()
		break
	case "bool", "boolean", "checkbox":
		attrs = getAttributes(&BoolField{})
		if _, ok := options["formula"]; ok {
			s = reflect.ValueOf(&BoolFieldRequired{}).Elem()
		} else {
			s = reflect.ValueOf(&BoolFieldRequired{false}).Elem()
		}
		break
	case "datetime", "date":
		attrs = getAttributes(&DatetimeField{})
		s = reflect.ValueOf(&DatetimeFieldRequired{}).Elem()
		break
	case "float", "double", "percent", "currency":
		attrs = getAttributes(&FloatField{})
		s = reflect.ValueOf(&FloatFieldRequired{16, 2}).Elem()
		break
	case "number", "int":
		attrs = getAttributes(&NumberField{})
		s = reflect.ValueOf(&NumberFieldRequired{18, 0}).Elem()
		break
	case "autonumber":
		attrs = getAttributes(&AutoNumberField{})
		s = reflect.ValueOf(&AutoNumberFieldRequired{0, "AN-{00000}"}).Elem()
		break
	case "geolocation":
		attrs = getAttributes(&GeolocationField{})
		s = reflect.ValueOf(&GeolocationFieldRequired{true, 5}).Elem()
		break
	case "lookup":
		attrs = getAttributes(&LookupField{})
		s = reflect.ValueOf(&LookupFieldRequired{}).Elem()
		break
	case "masterdetail":
		attrs = getAttributes(&MasterDetail{})
		s = reflect.ValueOf(&MasterDetailRequired{}).Elem()
		break
	default:
		//ErrorAndExit(fmt.Sprintf("Field type %s is not implemented.", typ))
		break
	}

	newOptions, err = ValidateOptionsAndDefaults(typ, attrs, s, options)

	return newOptions, nil
}

func NewForceMetadata(force *Force) (fm *ForceMetadata) {
	fm = &ForceMetadata{ApiVersion: apiVersionNumber, Force: force}
	return
}

type MetadataDeployStatus struct {
	Done    bool   `xml:"Body>checkStatusResponse>result>done"`
	State   string `xml:"Body>checkStatusResponse>result>state"`
	Message string `xml:"Body>checkStatusResponse>result>message"`
}

func (fm *ForceMetadata) GetStatus(id string) (status MetadataDeployStatus, err error) {
	body, err := fm.soapExecute("checkStatus", fmt.Sprintf("<id>%s</id>", id))
	if err != nil {
		return status, err
	}
	err = xml.Unmarshal(body, &status)
	return status, err
}

func (fm *ForceMetadata) CheckStatus(id string) error {
	for {
		status, err := fm.GetStatus(id)
		switch {
		case err != nil:
			return err
		case !status.Done:
			Log.Info(fmt.Sprintf("Not done yet: %s  Will check again in five seconds.", status.State))
			time.Sleep(5000 * time.Millisecond)
		case status.State == "Error":
			return errors.New(status.Message)
		default:
			return nil
		}
	}
}

func (results ForceCheckDeploymentStatusResult) String() string {
	complete := ""
	if results.Status == "InProgress" {
		complete = fmt.Sprintf(" (%d/%d)", results.NumberComponentsDeployed, results.NumberComponentsTotal)
	}
	if results.NumberTestsCompleted > 0 {
		complete = fmt.Sprintf(" (%d/%d)", results.NumberTestsCompleted, results.NumberTestsTotal)
	}

	return fmt.Sprintf("Status: %s%s %s", results.Status, complete, results.StateDetail)
}

func (fm *ForceMetadata) CancelDeploy(id string) (ForceCancelDeployResult, error) {
	var cancelResult ForceCancelDeployResult
	body, err := fm.soapExecute("cancelDeploy", fmt.Sprintf("<id>%s</id>", id))
	if err != nil {
		if err.Error() == "INVALID_ID_FIELD: Deployment already completed" {
			err = AlreadyCompletedError
		}
		return cancelResult, err
	}

	if err = xml.Unmarshal(body, &cancelResult); err != nil {
		return cancelResult, err
	}

	return cancelResult, nil
}

func (fm *ForceMetadata) CheckDeployStatus(id string) (results ForceCheckDeploymentStatusResult, err error) {
	body, err := fm.soapExecute("checkDeployStatus", fmt.Sprintf("<id>%s</id><includeDetails>true</includeDetails>", id))
	if err != nil {
		return
	}

	var deployResult struct {
		Results ForceCheckDeploymentStatusResult `xml:"Body>checkDeployStatusResponse>result"`
	}

	if err = xml.Unmarshal(body, &deployResult); err != nil {
		err = errors.New("Error decoding SOAP body: " + err.Error())
		return results, err
	}

	results = deployResult.Results
	return
}

func (fm *ForceMetadata) CheckRetrieveStatus(id string) (files ForceMetadataFiles, problems []string, err error) {
	tmp, err := os.CreateTemp("", "force-retrieve-*.zip")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	problems, err = fm.checkRetrieveStatusTo(id, tmp)
	if err != nil {
		return
	}
	if preserveZip {
		if err = copyFileContents(tmp.Name(), "inbound.zip"); err != nil {
			return
		}
	}
	zipfiles, err := zip.OpenReader(tmp.Name())
	if err != nil {
		return
	}
	defer zipfiles.Close()
	files = make(map[string][]byte)
	for _, file := range zipfiles.File {
		fd, ferr := file.Open()
		if ferr != nil {
			err = ferr
			return
		}
		data, ferr := ioutil.ReadAll(fd)
		fd.Close()
		if ferr != nil {
			err = ferr
			return
		}
		files[file.Name] = data
	}
	return
}

// checkRetrieveStatusTo streams the base64-decoded zip payload of the
// checkRetrieveStatus response to zipOut instead of holding it in memory. A
// session-expired response contains no zip payload, so nothing has been
// written to zipOut when the request is retried after refreshing the session.
func (fm *ForceMetadata) checkRetrieveStatusTo(id string, zipOut io.Writer) (problems []string, err error) {
	url := fmt.Sprintf("%s/services/Soap/m/%s", fm.Force.Credentials.InstanceUrl, fm.ApiVersion)
	soap := NewSoap(url, "http://soap.sforce.com/2006/04/metadata", fm.Force.Credentials.AccessToken)
	b64 := newBase64Writer(zipOut)
	body, err := soap.ExecuteExtract("checkRetrieveStatus", fmt.Sprintf("<id>%s</id>", id), "zipFile", b64)
	if err == SessionExpiredError {
		if err = fm.Force.RefreshSession(); err != nil {
			return
		}
		return fm.checkRetrieveStatusTo(id, zipOut)
	}
	if err != nil {
		return
	}
	if err = b64.Close(); err != nil {
		return
	}
	var status struct {
		Problems []string `xml:"Body>checkRetrieveStatusResponse>result>messages>problem"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	problems = status.Problems
	return
}

func copyFileContents(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func (fm *ForceMetadata) DescribeMetadata() (describe MetadataDescribeResult, err error) {
	body, err := fm.soapExecute("describeMetadata", fmt.Sprintf("<apiVersion>%s</apiVersion>", apiVersionNumber))
	if err != nil {
		return
	}
	var result struct {
		Data MetadataDescribeResult `xml:"Body>describeMetadataResponse>result"`
	}

	err = xml.Unmarshal([]byte(body), &result)

	if err == nil {
		describe = result.Data
	}
	return
}

func (fm *ForceMetadata) CreateConnectedApp(name, callback string) (err error) {
	soap := `
		<metadata xsi:type="ConnectedApp">
			<fullName>%s</fullName>
			<version>%s</version>
			<label>%s</label>
			<contactEmail>%s</contactEmail>
			<oauthConfig>
				<callbackUrl>%s</callbackUrl>
				<scopes>Full</scopes>
				<scopes>RefreshToken</scopes>
			</oauthConfig>
		</metadata>
	`
	me, err := fm.Force.Whoami()
	if err != nil {
		return err
	}
	email := me["Email"]
	body, err := fm.soapExecute("create", fmt.Sprintf(soap, name, apiVersionNumber, name, email, callback))
	if err != nil {
		return err
	}
	var status struct {
		Id string `xml:"Body>createResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	if err = fm.CheckStatus(status.Id); err != nil {
		return
	}
	return
}

func (fm *ForceMetadata) CreateCustomField(object, field, typ string, options map[string]string) (err error) {
	label := field
	field = strings.Replace(field, " ", "_", -1)
	soap := `
		<metadata xsi:type="CustomField" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<fullName>%s.%s__c</fullName>
			<label>%s</label>
			%s
		</metadata>
	`
	soapField := ""
	switch strings.ToLower(typ) {
	case "bool", "boolean", "checkbox":
		soapField = `<type>Checkbox</type>`
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "encryptedtext":
		soapField = "<type>EncryptedText</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "text", "string":
		soapField = "<type>Text</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "picklist":
		soapField = "<type>Picklist</type>\n"
		for key, value := range options {
			fmt.Println("Options: ", options)
			fmt.Println(fmt.Sprintf("Key %s", key))
			if key == "picklist>picklistValues" {
				soapField += "<picklist>\n"
				for _, k := range strings.Split(value, ",") {
					soapField += fmt.Sprintf("<picklistValues>\n<fullName>%s</fullName>\n<default>false</default>\n</picklistValues>\n", strings.Trim(k, " "))
				}
				soapField += "</picklist>\n"
			} else {
				soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
			}
		}
	case "email":
		soapField = "<type>Email</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "url":
		soapField = "<type>Url</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "phone":
		soapField = "<type>Phone</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "date":
		soapField = "<type>Date</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "datetime":
		soapField = "<type>DateTime</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "number", "int":
		soapField = "<type>Number</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "percent":
		soapField = "<type>Percent</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "autonumber":
		soapField = "<type>AutoNumber</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "float", "double":
		soapField = "<type>Number</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "currency":
		soapField = "<type>Currency</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "geolocation":
		soapField = "<type>Location</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "lookup":
		soapField = `<type>Lookup</type>
					<referenceTo>%s</referenceTo>
					<relationshipLabel>%ss</relationshipLabel>
					<relationshipName>%s_del</relationshipName>
					`
		scanner := bufio.NewScanner(os.Stdin)

		var inp, inp2 string
		fmt.Print("Enter object to lookup: ")

		scanner.Scan()
		inp = scanner.Text()

		fmt.Print("What is the label for the loookup? ")
		scanner.Scan()
		inp2 = scanner.Text()

		soapField = fmt.Sprintf(soapField, inp, inp2, strings.Replace(inp2, " ", "_", -1))
	case "masterdetail":
		soapField = `<type>MasterDetail</type>
					 <externalId>false</externalId>
					 <referenceTo>%s</referenceTo>
					 <relationshipLabel>%ss</relationshipLabel>
					 <relationshipName>%s_del</relationshipName>
					 <relationshipOrder>0</relationshipOrder>
					 <reparentableMasterDetail>false</reparentableMasterDetail>
					 <trackTrending>false</trackTrending>
					 <writeRequiresMasterRead>false</writeRequiresMasterRead>
					`

		scanner := bufio.NewScanner(os.Stdin)
		var inp, inp2 string
		fmt.Print("Enter object to lookup: ")

		scanner.Scan()
		inp = scanner.Text()

		fmt.Print("What is the label for the loookup? ")
		scanner.Scan()
		inp2 = scanner.Text()

		soapField = fmt.Sprintf(soapField, inp, inp2, strings.Replace(inp2, " ", "_", -1))
	case "textarea":
		soapField = "<type>TextArea</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "longtextarea":
		soapField = "<type>LongTextArea</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	case "richtextarea":
		soapField = "<type>Html</type>"
		for key, value := range options {
			soapField += fmt.Sprintf("<%s>%s</%s>", key, value, key)
		}
	default:
		ErrorAndExit("unable to create field type: %s", typ)
	}

	body, err := fm.soapExecute("create", fmt.Sprintf(soap, object, field, label, soapField))
	if err != nil {
		return err
	}
	var status struct {
		Id string `xml:"Body>createResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	if err = fm.CheckStatus(status.Id); err != nil {
		return
	}
	return
}

func (fm *ForceMetadata) DeleteCustomField(object, field string) (err error) {
	soap := `
		<metadata xsi:type="CustomField" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<fullName>%s.%s</fullName>
		</metadata>
	`
	body, err := fm.soapExecute("delete", fmt.Sprintf(soap, object, field))
	if err != nil {
		return err
	}
	var status struct {
		Id string `xml:"Body>deleteResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	if err = fm.CheckStatus(status.Id); err != nil {
		return
	}
	return
}

func (fm *ForceMetadata) CreateBigObject(object BigObject) (err error) {
	soap := object.ToXml()

	ioutil.WriteFile(filepath.Join("metadata/objects", fmt.Sprintf("%s__b.object", object.Label)), []byte(soap), 0644)
	path, _ := filepath.Abs(filepath.Join("metadata/objects", fmt.Sprintf("%s__b.object", object.Label)))
	cmd := exec.Command("force", "push",
		fmt.Sprintf("-f=%s", path))
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	return
}

func (fm *ForceMetadata) InstallPackage(namespace, version, password string) (err error) {
	activateRemoteSiteSettings := false
	return fm.InstallPackageWithRSS(namespace, version, password, activateRemoteSiteSettings)
}

func (fm *ForceMetadata) InstallPackageByNamespaceAndVersion(namespace, version, password string, activateRemoteSiteSettings bool) (id string, err error) {
	soap := `
		<metadata xsi:type="InstalledPackage" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<fullName>%s</fullName>
			<versionNumber>%s</versionNumber>
			<password>%s</password>
			<activateRSS>%t</activateRSS>
		</metadata>
	`
	body, err := fm.soapExecute("create", fmt.Sprintf(soap, namespace, version, password, activateRemoteSiteSettings))
	if err != nil {
		return "", err
	}
	var status struct {
		Id string `xml:"Body>createResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return "", err
	}
	return status.Id, nil
}

func (fm *ForceMetadata) InstallPackageWithRSS(namespace, version, password string, activateRemoteSiteSettings bool) (err error) {
	id, err := fm.InstallPackageByNamespaceAndVersion(namespace, version, password, activateRemoteSiteSettings)
	if err = fm.CheckStatus(id); err != nil {
		return
	}
	return
}

func (fm *ForceMetadata) CreateCustomObject(object string) (err error) {
	fld := ""
	fld = strings.ToUpper(object)
	fld = fld[0:1]
	soap := `
		<metadata xsi:type="CustomObject" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<fullName>%s__c</fullName>
			<label>%s</label>
			<pluralLabel>%s</pluralLabel>
			<deploymentStatus>Deployed</deploymentStatus>
			<sharingModel>ReadWrite</sharingModel>
			<nameField>
				<label>%s Name</label>
				<type>AutoNumber</type>
				<displayFormat>%s-{00000}</displayFormat>
				<startingNumber>1</startingNumber>
			</nameField>
		</metadata>
	`
	body, err := fm.soapExecute("create", fmt.Sprintf(soap, object, object, inflect.Pluralize(object), object, fld))
	if err != nil {
		return err
	}
	var status struct {
		Id string `xml:"Body>createResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	if err = fm.CheckStatus(status.Id); err != nil {
		return
	}
	return
}

func (fm *ForceMetadata) DeleteCustomObject(object string) (err error) {
	soap := `
		<metadata xsi:type="CustomObject" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<fullName>%s</fullName>
		</metadata>
	`
	body, err := fm.soapExecute("delete", fmt.Sprintf(soap, object))
	if err != nil {
		return err
	}
	var status struct {
		Id string `xml:"Body>deleteResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	if err = fm.CheckStatus(status.Id); err != nil {
		return
	}
	return
}

func (fm *ForceMetadata) MakeDeploySoap(options ForceDeployOptions) (soap string) {
	if len(options.RunTests) > 0 {
		options.TestLevel = "RunSpecifiedTests"
	}
	deployOptions, _ := xml.Marshal(options)
	soap = fmt.Sprintf("<zipFile>%s</zipFile>%s", "%s", string(deployOptions))
	return
}

func (fm *ForceMetadata) MakeZip(files ForceMetadataFiles) (zipdata []byte, err error) {
	var options ForceDeployOptions
	options.SinglePackage = true
	return fm.MakeZipWithOptions(files, options)
}

func (fm *ForceMetadata) MakeZipWithOptions(files ForceMetadataFiles, options ForceDeployOptions) (zipdata []byte, err error) {
	zipfile := new(bytes.Buffer)
	zipper := zip.NewWriter(zipfile)
	for name, data := range files {
		name = filepath.ToSlash(name)
		if !options.SinglePackage {
			name = fmt.Sprintf("unpackaged/%s", name)
		}
		wr, err := zipper.Create(name)
		if err != nil {
			return nil, err
		}
		wr.Write(data)
	}
	zipper.Close()
	zipdata = zipfile.Bytes()
	return
}

// Deploy metadata and wait unti deploy is complete, then return results
func (fm *ForceMetadata) Deploy(files ForceMetadataFiles, options ForceDeployOptions) (results ForceCheckDeploymentStatusResult, err error) {
	zipfile, err := fm.MakeZip(files)

	results, err = fm.DeployZipFile(zipfile, options)
	return
}

// Start a deployment of metadata and return the deploy id
func (fm *ForceMetadata) StartDeploy(files ForceMetadataFiles, options ForceDeployOptions) (string, error) {
	zipfile, err := fm.MakeZip(files)
	if err != nil {
		return "", err
	}
	return fm.startDeployZipFile(zipfile, options)
}

func (fm *ForceMetadata) DeployZipFile(zipfile []byte, options ForceDeployOptions) (results ForceCheckDeploymentStatusResult, err error) {
	deployId, err := fm.startDeployZipFile(zipfile, options)
	if err != nil {
		return results, err
	}
	for {
		results, err = fm.CheckDeployStatus(deployId)
		if err != nil || results.Done {
			return
		}
		Log.Info(results)
		time.Sleep(5000 * time.Millisecond)
	}
}

func deploySoapBody(zipfile []byte, options ForceDeployOptions) string {
	if len(options.RunTests) > 0 {
		options.TestLevel = "RunSpecifiedTests"
	}
	deployOptions, _ := xml.Marshal(options)
	var soapBody strings.Builder
	soapBody.WriteString("<zipFile>")
	soapBody.WriteString(base64.StdEncoding.EncodeToString(zipfile))
	soapBody.WriteString("</zipFile>")
	soapBody.WriteString(string(deployOptions))
	return soapBody.String()
}

func (fm *ForceMetadata) startDeployZipFile(zipfile []byte, options ForceDeployOptions) (string, error) {
	body, err := fm.soapExecute("deploy", deploySoapBody(zipfile, options))
	if err != nil {
		return "", err
	}

	var status struct {
		Id string `xml:"Body>deployResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return "", err
	}
	return status.Id, nil
}

func (fm *ForceMetadata) DeployRecentValidation(validationId string) (results ForceCheckDeploymentStatusResult, err error) {
	body, err := fm.soapExecute("deployRecentValidation", fmt.Sprintf("<validationID>%s</validationID>", validationId))
	if err != nil {
		return
	}

	var status struct {
		Id string `xml:"Body>deployRecentValidationResponse>result"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	for {
		results, err = fm.CheckDeployStatus(status.Id)
		if err != nil || results.Done {
			return
		}
		Log.Info(results)
		time.Sleep(5000 * time.Millisecond)
	}
}

func (fm *ForceMetadata) RetrieveByPackageXml(package_xml string) (ForceMetadataFiles, []string, error) {
	// Need to crack open the xml file and pull out the <types> array
	data, err := ioutil.ReadFile(package_xml)

	if err != nil {
		ErrorAndExit(err.Error())
	}
	return fm.RetrieveByPackageXmlContents(data)
}

func (fm *ForceMetadata) RetrieveByPackageXmlContents(data []byte) (files ForceMetadataFiles, problems []string, err error) {
	query, err := PackageXmlToQuery(data)
	if err != nil {
		return
	}
	return fm.Retrieve(query)
}

// PackageXmlToQuery converts the contents of a package.xml manifest into a
// ForceMetadataQuery.
func PackageXmlToQuery(data []byte) (query ForceMetadataQuery, err error) {
	var pxml struct {
		Types []struct {
			Name    string   `xml:"name"`
			Members []string `xml:"members"`
		} `xml:"types"`
	}
	if err = xml.Unmarshal(data, &pxml); err != nil {
		return
	}
	for _, t := range pxml.Types {
		query = append(query, ForceMetadataQueryElement{Name: []string{t.Name}, Members: t.Members})
	}
	return
}

func (fm *ForceMetadata) startRetrieve(query ForceMetadataQuery) (id string, err error) {
	soap := `
		<retrieveRequest>
			<apiVersion>%s</apiVersion>
			<unpackaged>
				%s
			</unpackaged>
		</retrieveRequest>
	`
	soapType := `
		<types>
			<name>%s</name>
			%s
		</types>
	`
	soapTypeMembers := `<members>%s</members>`
	types := ""
	for _, element := range query {
		members := ""
		for _, member := range element.Members {
			members += fmt.Sprintf(soapTypeMembers, member)
		}
		for _, atype := range element.Name {
			types += fmt.Sprintf(soapType, atype, members)
		}
	}
	body, err := fm.soapExecute("retrieve", fmt.Sprintf(soap, apiVersionNumber, types))
	if err != nil {
		return
	}
	var status struct {
		Id string `xml:"Body>retrieveResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	id = status.Id
	return
}

func (fm *ForceMetadata) Retrieve(query ForceMetadataQuery) (files ForceMetadataFiles, problems []string, err error) {
	files = make(ForceMetadataFiles)
	problems, err = fm.RetrieveStream(query, collectFiles(files))
	return
}

// collectFiles returns a RetrieveEntryHandler that reads each entry into
// files.
func collectFiles(files ForceMetadataFiles) RetrieveEntryHandler {
	return func(name string, r io.Reader) error {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		files[name] = data
		return nil
	}
}

// RetrieveEntryHandler receives each retrieved file as it is decompressed.
// name has the retrieve prefix (e.g. "unpackaged/") stripped and uses forward
// slashes. The handler must consume r before returning.
type RetrieveEntryHandler func(name string, r io.Reader) error

// RetrieveStream retrieves metadata, streaming each retrieved file to handle
// as it is decompressed, so the retrieved metadata is never held in memory.
// Queries exceeding the retrieve file limit are split into multiple retrieve
// calls; the package.xml manifests of all batches are merged and passed to
// handle once, after all other entries.
func (fm *ForceMetadata) RetrieveStream(query ForceMetadataQuery, handle RetrieveEntryHandler) (problems []string, err error) {
	var packageXmls [][]byte
	zipCount := 0
	for _, batch := range splitQuery(query, maxFilesPerRetrieve) {
		var batchProblems []string
		batchProblems, err = fm.retrieveBatchStream(batch, &zipCount, &packageXmls, handle)
		if err != nil {
			return
		}
		problems = append(problems, batchProblems...)
	}
	err = emitPackageXml(packageXmls, handle)
	return
}

// retrieveBatchStream retrieves one batch. If the server still rejects the
// batch for having too many files, which can happen when wildcard members
// expand past the limit, the batch is bisected and each half retried.
func (fm *ForceMetadata) retrieveBatchStream(batch ForceMetadataQuery, zipCount *int, packageXmls *[][]byte, handle RetrieveEntryHandler) (problems []string, err error) {
	id, err := fm.startRetrieve(batch)
	if err == nil {
		problems, err = fm.streamRetrieveResult(id, zipCount, packageXmls, handle)
	}
	if isRetrieveLimitError(err) {
		if total := countQueryMembers(batch); total > 1 {
			Log.Info("Retrieve exceeded the file limit; splitting into smaller retrieve calls")
			problems = nil
			for _, half := range splitQuery(batch, (total+1)/2) {
				var halfProblems []string
				halfProblems, err = fm.retrieveBatchStream(half, zipCount, packageXmls, handle)
				if err != nil {
					return
				}
				problems = append(problems, halfProblems...)
			}
		}
	}
	return
}

// RetrievePackageStream retrieves an unmanaged package, streaming each
// retrieved file to handle as it is decompressed.
func (fm *ForceMetadata) RetrievePackageStream(packageName string, handle RetrieveEntryHandler) (problems []string, err error) {
	id, err := fm.startRetrievePackage(packageName)
	if err != nil {
		return
	}
	var packageXmls [][]byte
	zipCount := 0
	problems, err = fm.streamRetrieveResult(id, &zipCount, &packageXmls, handle)
	if err != nil {
		return
	}
	err = emitPackageXml(packageXmls, handle)
	return
}

// RetrieveToDir retrieves metadata and extracts it directly under root,
// streaming the zip payload to a temporary file and writing each entry to
// disk as it is decompressed.
func (fm *ForceMetadata) RetrieveToDir(root string, query ForceMetadataQuery) (problems []string, err error) {
	return fm.RetrieveStream(query, EntryDiskWriter(root))
}

// streamRetrieveResult downloads the result zip for a completed retrieve to a
// temporary file and streams its entries to handle. package.xml entries are
// collected into packageXmls instead so they can be merged across batches.
func (fm *ForceMetadata) streamRetrieveResult(id string, zipCount *int, packageXmls *[][]byte, handle RetrieveEntryHandler) (problems []string, err error) {
	if err = fm.CheckStatus(id); err != nil {
		return
	}
	tmp, err := os.CreateTemp("", "force-retrieve-*.zip")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	problems, err = fm.checkRetrieveStatusTo(id, tmp)
	if err != nil {
		return
	}
	*zipCount++
	if preserveZip {
		name := "inbound.zip"
		if *zipCount > 1 {
			name = fmt.Sprintf("inbound-%d.zip", *zipCount)
		}
		if err = copyFileContents(tmp.Name(), name); err != nil {
			return
		}
	}
	err = streamZipEntries(tmp.Name(), "unpackaged/", packageXmls, handle)
	return
}

func streamZipEntries(zipPath, stripPrefix string, packageXmls *[][]byte, handle RetrieveEntryHandler) error {
	zipfiles, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zipfiles.Close()
	for _, file := range zipfiles.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name := strings.Replace(file.Name, stripPrefix, "", -1)
		fd, err := file.Open()
		if err != nil {
			return err
		}
		if name == "package.xml" {
			data, rerr := io.ReadAll(fd)
			fd.Close()
			if rerr != nil {
				return rerr
			}
			*packageXmls = append(*packageXmls, data)
			continue
		}
		herr := handle(name, fd)
		fd.Close()
		if herr != nil {
			return herr
		}
	}
	return nil
}

func emitPackageXml(packageXmls [][]byte, handle RetrieveEntryHandler) error {
	if len(packageXmls) == 0 {
		return nil
	}
	pkg := packageXmls[0]
	if len(packageXmls) > 1 {
		var err error
		if pkg, err = mergePackageXml(packageXmls...); err != nil {
			return err
		}
	}
	return handle("package.xml", bytes.NewReader(pkg))
}

// EntryDiskWriter returns a RetrieveEntryHandler that writes each entry to
// its path under root.
func EntryDiskWriter(root string) RetrieveEntryHandler {
	cleanRoot := filepath.Clean(root)
	return func(name string, r io.Reader) error {
		target := filepath.Join(cleanRoot, filepath.FromSlash(name))
		if target != cleanRoot && !strings.HasPrefix(target, cleanRoot+string(os.PathSeparator)) {
			return fmt.Errorf("entry %s would be written outside %s", name, root)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, r); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
}

func (fm *ForceMetadata) startRetrievePackage(packageName string) (id string, err error) {
	soap := `
		<retrieveRequest>
			<apiVersion>%s</apiVersion>
			<packageNames>%s</packageNames>
		</retrieveRequest>
	`
	body, err := fm.soapExecute("retrieve", fmt.Sprintf(soap, apiVersionNumber, packageName))
	if err != nil {
		return
	}
	var status struct {
		Id string `xml:"Body>retrieveResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	id = status.Id
	return
}

func (fm *ForceMetadata) RetrievePackage(packageName string) (files ForceMetadataFiles, problems []string, err error) {
	files = make(ForceMetadataFiles)
	problems, err = fm.RetrievePackageStream(packageName, collectFiles(files))
	return
}

func (fm *ForceMetadata) ListMetadata(query string) (res []byte, err error) {
	if strings.Contains(query, ":") {
		newquery := strings.Split(query, ":")
		return fm.soapExecute("listMetadata", fmt.Sprintf("<queries><type>%s</type><folder>%s</folder></queries>", newquery[0], newquery[1]))
	} else {
		return fm.soapExecute("listMetadata", fmt.Sprintf("<queries><type>%s</type></queries>", query))
	}
}

func (fm *ForceMetadata) ListAllMetadata() (describe MetadataDescribeResult, err error) {
	describe, err = fm.DescribeMetadata()
	return
}

func (fm *ForceMetadata) ListConnectedApps() (apps ForceConnectedApps, err error) {
	originalVersion := fm.ApiVersion
	fm.ApiVersion = apiVersionNumber
	body, err := fm.ListMetadata("ConnectedApp")
	fm.ApiVersion = originalVersion
	if err != nil {
		return
	}
	var res struct {
		ConnectedApps []ForceConnectedApp `xml:"Body>listMetadataResponse>result"`
	}
	if err = xml.Unmarshal(body, &res); err != nil {
		return
	}
	apps = res.ConnectedApps
	return
}

func (fm *ForceMetadata) soapExecute(action, query string) (response []byte, err error) {
	url := fmt.Sprintf("%s/services/Soap/m/%s", fm.Force.Credentials.InstanceUrl, fm.ApiVersion)
	soap := NewSoap(url, "http://soap.sforce.com/2006/04/metadata", fm.Force.Credentials.AccessToken)
	response, err = soap.Execute(action, query)
	if err == SessionExpiredError {
		err = fm.Force.RefreshSession()
		if err != nil {
			return
		}
		return fm.soapExecute(action, query)
	}
	return
}
