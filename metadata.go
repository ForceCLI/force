package main

import (
	"archive/zip"
	"bitbucket.org/pkg/inflect"
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type BigObject struct {
	DeploymentStatus string
	Label            string
	PluralLabel      string
	Fields           []BigObjectField
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
	Changed     bool   `xml:"changed"`
	Created     bool   `xml:"created"`
	Deleted     bool   `xml:"deleted"`
	FileName    string `xml:"fileName"`
	FullName    string `xml:"fullName"`
	LineNumber  int    `xml:"lineNumber"`
	Problem     string `xml:"problem"`
	ProblemType string `xml:"problemType"`
	Success     bool   `xml:"success"`
}

type ComponentSuccess struct {
	Changed  bool   `xml:"changed"`
	Created  bool   `xml:"created"`
	Deleted  bool   `xml:"deleted"`
	FileName string `xml:"fileName"`
	FullName string `xml:"fullName"`
	Id       string `xml:"id"`
	Success  bool   `xml:"success"`
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

type RunTestResult struct {
	NumberOfFailures int           `xml:"numFailures"`
	NumberOfTestsRun int           `xml:"numTestsRun"`
	TotalTime        float32       `xml:"totalTime"`
	TestFailures     []TestFailure `xml:"failures"`
	TestSuccesses    []TestSuccess `xml:"successes"`
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
	Details                  ComponentDetails `xml:"details"`
	Done                     bool             `xml:"done"`
	Id                       string           `xml:"id"`
	NumberComponentErrors    int              `xml:"numberComponentErrors"`
	NumberComponentsDeployed int              `xml:"numberComponentsDeployed"`
	NumberComponentsTotal    int              `xml:"numberComponentsTotal"`
	NumberTestErrors         int              `xml:"numberTestErrors"`
	NumberTestsCompleted     int              `xml:"numberTestsCompleted"`
	NumberTestsTotal         int              `xml:"numberTestsTotal"`
	RollbackOnError          bool             `xml:"rollbackOnError"`
	Status                   string           `xml:"status"`
	Success                  bool             `xml:"success"`
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
	Name    string
	Members []string
}

type ForceMetadataQuery []ForceMetadataQueryElement

type ForceMetadataFiles map[string][]byte

type ForceMetadata struct {
	ApiVersion string
	Force      *Force
}

type ForceDeployOptions struct {
	AllowMissingFiles bool     `xml:"allowMissingFiles"`
	AutoUpdatePackage bool     `xml:"autoUpdatePackage"`
	CheckOnly         bool     `xml:"checkOnly"`
	IgnoreWarnings    bool     `xml:"ignoreWarnings"`
	PerformRetrieve   bool     `xml:"performRetrieve"`
	PurgeOnDelete     bool     `xml:"purgeOnDelete"`
	RollbackOnError   bool     `xml:"rollbackOnError"`
	TestLevel         string   `xml:"testLevel"`
	RunTests          []string `xml:"runTests"`
	SinglePackage     bool     `xml:"singlePackage"`
}

/* These structs define which options are available and which are
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
	HelpText                 string `xml:"helpText"`
}

type AutoNumberFieldRequired struct {
	StartingNumber int    `xml:"startingNumber"`
	DisplayFormat  string `xml:"displayFormat"`
}

type AutoNumberField struct {
	StartingNumber int    `xml:"startingNumber"`
	DisplayFormat  string `xml:"displayFormat"`
	Description    string `xml:"description"`
	HelpText       string `xml:"helpText"`
	ExternalId     bool   `xml:"externalId"`
}

type FloatFieldRequired struct {
	Precision int `xml:"precision"`
	Scale     int `xml:"scale"`
}

type FloatField struct {
	Description          string `xml:"description"`
	HelpText             string `xml:"helpText"`
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
	Description          string `xml:"description"`
	HelpText             string `xml:"helpText"`
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
	Description          string    `xml:"description"`
	HelpText             string    `xml:"helpText"`
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
	Picklist []PicklistValue `xml:"picklist>picklistValues"`
}

type BoolFieldRequired struct {
	DefaultValue bool `xml:"defaultValue"`
}

type BoolField struct {
	Description          string `xml:"description"`
	HelpText             string `xml:"helpText"`
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
	LastModifedByDate  time.Time `xml:"lastModifiedByDate"`
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
	HelpText    string `xml:"helpText"`
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
	HelpText             string `xml:"helpText"`
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
	HelpText     string `xml:"helpText"`
	DefaultValue string `xml:"defaultValue"`
}

type UrlFieldRequired struct {
}

type UrlField struct {
	Label                string `xml:"label"`
	Name                 string `xml:"fullName"`
	Required             bool   `xml:"required"`
	Description          string `xml:"description"`
	HelpText             string `xml:"helpText"`
	DefaultValue         string `xml:"defaultValue"`
	Formula              string `xml:"formula"`
	FormulaTreatBlanksAs string `xml:"formulaTreatBlanksAs"`
}

type EmailFieldRequired struct {
}

type TextAreaFieldRequired struct {
}

type TextAreaField struct {
	Label        string `xml:"label"`
	Name         string `xml:"fullName"`
	Required     bool   `xml:"required"`
	Description  string `xml:"description"`
	HelpText     string `xml:"helpText"`
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
	HelpText     string `xml:"helpText"`
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
	HelpText     string `xml:"helpText"`
	Length       int    `xml:"length"`
	VisibleLines int    `xml:"visibleLines"`
}

type LookupFieldRequired struct{}

type LookupField struct {
	ReferenceTo       string `xml:"referenceTo"`
	RelationshipLabel string `xml:"relationshipLabel"`
	RelationshipName  string `xml:"relationshipName"`
}

type MasterDetailRequired struct{}

type MasterDetail struct {
	ReferenceTo       string `xml:"referenceTo"`
	RelationshipLabel string `xml:"relationshipLabel"`
	RelationshipName  string `xml:"relationshipName"`
}

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

	switch typ {
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

func (fm *ForceMetadata) CheckStatus(id string) (err error) {
	body, err := fm.soapExecute("checkStatus", fmt.Sprintf("<id>%s</id>", id))
	if err != nil {
		return
	}
	var status struct {
		Done    bool   `xml:"Body>checkStatusResponse>result>done"`
		State   string `xml:"Body>checkStatusResponse>result>state"`
		Message string `xml:"Body>checkStatusResponse>result>message"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	switch {
	case !status.Done:
		fmt.Printf("Not done yet: %s  Will check again in five seconds.\n", status.State)
		time.Sleep(5000 * time.Millisecond)
		return fm.CheckStatus(id)
	case status.State == "Error":
		return errors.New(status.Message)
	}
	return
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
		ErrorAndExit(err.Error())
	}

	results = deployResult.Results
	return
}

func (fm *ForceMetadata) CheckRetrieveStatus(id string) (files ForceMetadataFiles, err error) {
	body, err := fm.soapExecute("checkRetrieveStatus", fmt.Sprintf("<id>%s</id>", id))
	if err != nil {
		fmt.Printf("Hrm... will probably try again\n")
		return
	}
	var status struct {
		ZipFile string `xml:"Body>checkRetrieveStatusResponse>result>zipFile"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	data, err := base64.StdEncoding.DecodeString(status.ZipFile)
	if err != nil {
		return
	}

	zipfiles, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if preserveZip == true {
		ioutil.WriteFile("inbound.zip", data, 0644)
	}
	if err != nil {
		return
	}
	files = make(map[string][]byte)
	for _, file := range zipfiles.File {
		fd, _ := file.Open()
		defer fd.Close()
		data, _ := ioutil.ReadAll(fd)
		files[file.Name] = data
	}
	return
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

	if err != nil {
		fmt.Println(err.Error())
	} else {
		describe = result.Data
	}
	//fm.DescribeMetadataValue("{http://soap.sforce.com/2006/04/metadata}EmailTemplate")
	return
}

//func (fm *ForceMetadata) DescribeMetadataValue(entitytype string) (describe MetadataDescribeValueTypeResult, err error) {
//	body, err := fm.soapExecute("describeValueType", fmt.Sprintf("<type>%s</type>", entitytype))
//	if err != nil {
//		return
//	}
/*var result struct {
	Data MetadataDescribeResult `xml:"Body>describeMetadataResponse>result"`
}

err = xml.Unmarshal([]byte(body), &result)

if err != nil {
	fmt.Println(err.Error())
	} else {
		describe = result.Data
	}
*/
//		return
//	}

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

	//fmt.Println(fmt.Sprintf(soap, object, field, label, soapField))
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
	tests := ""
	for _, test := range options.RunTests {
		tests += "<runTests>" + test + "</runTests>\n"
	}
	var testLevel string
	if tests == "" {
		testLevel = options.TestLevel
	} else {
		testLevel = "RunSpecifiedTests"
	}
	soap = fmt.Sprintf(`
		<zipFile>%s</zipFile>
		<deployOptions>
			<allowMissingFiles>%t</allowMissingFiles>
			<autoUpdatePackage>%t</autoUpdatePackage>
			<checkOnly>%t</checkOnly>
			<ignoreWarnings>%t</ignoreWarnings>
			<purgeOnDelete>%t</purgeOnDelete>
			<rollbackOnError>%t</rollbackOnError>
			<testLevel>%s</testLevel>
			%s
		</deployOptions>
		`, "%s", options.AllowMissingFiles, options.AutoUpdatePackage, options.CheckOnly, options.IgnoreWarnings, options.PurgeOnDelete, options.RollbackOnError, testLevel, tests)
	return
}

func (fm *ForceMetadata) MakeZip(files ForceMetadataFiles) (zipdata []byte, err error) {
	zipfile := new(bytes.Buffer)
	zipper := zip.NewWriter(zipfile)
	for name, data := range files {
		name = filepath.ToSlash(name)
		wr, err := zipper.Create(fmt.Sprintf("unpackaged/%s", name))
		if err != nil {
			return nil, err
		}
		wr.Write(data)
	}
	zipper.Close()
	zipdata = zipfile.Bytes()
	return
}

func (fm *ForceMetadata) Deploy(files ForceMetadataFiles, options ForceDeployOptions) (results ForceCheckDeploymentStatusResult, err error) {
	soap := fm.MakeDeploySoap(options)

	zipfile, err := fm.MakeZip(files)

	results, err = fm.DeployZipFile(soap, zipfile)
	return
}

func (fm *ForceMetadata) DeployZipFile(soap string, zipfile []byte) (results ForceCheckDeploymentStatusResult, err error) {
	//ioutil.WriteFile("package.zip", zipfile, 0644)
	encoded := base64.StdEncoding.EncodeToString(zipfile)
	body, err := fm.soapExecute("deploy", fmt.Sprintf(soap, encoded))
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	var status struct {
		Id string `xml:"Body>deployResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	if err = fm.CheckStatus(status.Id); err != nil {
		return
	}
	results, err = fm.CheckDeployStatus(status.Id)

	return
}

func (fm *ForceMetadata) Retrieve(query ForceMetadataQuery) (files ForceMetadataFiles, err error) {

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
		types += fmt.Sprintf(soapType, element.Name, members)
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

	if err = fm.CheckStatus(status.Id); err != nil {
		return
	}
	raw_files, err := fm.CheckRetrieveStatus(status.Id)
	if err != nil {
		return
	}
	files = make(ForceMetadataFiles)
	for raw_name, data := range raw_files {
		name := strings.Replace(raw_name, "unpackaged/", "", -1)
		files[name] = data
	}
	return
}

func (fm *ForceMetadata) RetrievePackage(packageName string) (files ForceMetadataFiles, err error) {
	soap := `
		<retrieveRequest>
			<apiVersion>%s</apiVersion>
			<packageNames>%s</packageNames>
		</retrieveRequest>
	`
	soap = fmt.Sprintf(soap, apiVersionNumber, packageName)
	body, err := fm.soapExecute("retrieve", soap)
	if err != nil {
		return
	}
	var status struct {
		Id string `xml:"Body>retrieveResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	if err = fm.CheckStatus(status.Id); err != nil {
		return
	}
	raw_files, err := fm.CheckRetrieveStatus(status.Id)
	if err != nil {
		return
	}
	files = make(ForceMetadataFiles)
	for raw_name, data := range raw_files {
		name := strings.Replace(raw_name, fmt.Sprintf("unpackaged%s", string(os.PathSeparator)), "", -1)
		files[name] = data
	}
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
	login, err := fm.Force.Get(fm.Force.Credentials.Id)
	if err != nil {
		return
	}
	url := strings.Replace(login["urls"].(map[string]interface{})["metadata"].(string), "{version}", fm.ApiVersion, 1)
	soap := NewSoap(url, "http://soap.sforce.com/2006/04/metadata", fm.Force.Credentials.AccessToken)
	response, err = soap.Execute(action, query)
	return
}
