package salesforce

/*import (
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
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)*/
/*
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

type RunTestResult struct {
	NumberOfFailures int `xml:"numFailures"`
	NumberOfTestsRun int `xml:"numTestsRun"`
	TotalTime        int `xml:"totalTime"`
}

type ComponentDetails struct {
	ComponentSuccesses []ComponentSuccess `xml:"componentSuccesses"`
	ComponentFailures  []ComponentFailure `xml:"componentFailures"`
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
	RunAllTests       bool     `xml:"runAllTests"`
	runTests          []string `xml:"runTests"`
	SinglePackage     bool     `xml:"singlePackage"`
}
*/
/* These structs define which options are available and which are
   required for the various field types you can create. Reflection
   is used to leverage these structs in validating options when creating
   a custom field.
*/
/*
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

type MetaData struct {
	FullName 		string
}

type MetaDataWithContent struct {
	MetaData
	Content 			[]byte
}

type CustomField struct {
	MetaData
	CaseSensitive 				bool
	DefaultValue   				bool
	DeleteConstraint 			DeleteConstraint
	Deprecated					bool
	Description 				string
	DisplayFormat 				string
	DisplayLocationInDecimal 	bool
	Encrypted 					bool
	ExternalDeveloperName 		string
	ExternalId 					bool
	Formula 					string
	FormulaTreatBlankAs 		TreatBlankAs
	Indexed 					bool
	InlineHelperText 			string
	IsFilteringDisabled 		bool
	IsNameField 				bool
	IsSortingDisabled 			bool
	ReparentableMasterDetail 	bool
	Label 						string
	Length 						int
	LookupFilter 				LookupFilter
	MaskChar 					EncryptedFieldMaskChar
	MaskType 					EncryptedFieldMaskType
	Picklist 					Picklist
	PopulateExistingRows		bool
	Precision 					int
	ReferenceTargetField		string
	ReferenceTo 				string
	RelationshipLabel 			string
	RelationshipName 			string
	RelationshipOrder 			int
	Required 					bool
	Scale 						int
	StartingNumber 				int
	StripMarkup 				bool
	SummarizedField 			string
	SummaryFilterItems 			[]FilterItem
	SummaryForeignKey 			string
	SummaryOperations 			SummaryOperations
	TrackFeedHistory 			bool
	TrackHistory 				bool
	TrackTrending 				bool
	TrueValueIndexed 			bool
	Type 						FieldType
	Unique 						bool
	VisibleLines 				int
	WriteRequiresMasterRead 	bool
}

type EncryptedFieldMaskChar int
const (
	Asterisk EncryptedFieldMaskChar = 1 + iota
	X
)
var encryptedFieldMaskChars = [...]string {
	"asterisk",
	"X",
}
func (efm EncryptedFieldMaskChar) String() string { return encryptedFieldMaskChars[efm - 1] }

type EncryptedFieldMaskType int
const (
	All EncryptedFieldMaskType = 1 + iota
	CreditCard
	Ssn
	LastFour
	Sin
	Nino
)
var encryptedFieldMaskTypes = [...]string {
	"all",
	"creditCard",
	"ssn",
	"lastFour",
	"sin",
	"nino",
}
func (efmt EncryptedFieldMaskType) String() string { return encryptedFieldMaskTypes[efmt - 1] }

type DeleteConstraint int
const (
	SetNull DeleteConstraint = 1 + iota
	Restrict
	Cascade
)
var deleteConstraints = [...]string {
	"SetNull",
	"Restrict",
	"Cascade",
}
func (dc DeleteConstraint) String() string { return deleteConstraints[dc - 1] }

type TreatBlankAs int
const (
	BlankAsBlank TreatBlankAs = 1 + iota
	BlankAsZero
)
var treatBlankAs = [...]string {
	"BlankAsBlank",
	"BlankAsZero",
}
func (tba TreatBlankAs) String() string { return treatBlankAs[tba - 1] }

type FilterOperation int
const (
	Equals FilterOperation = 1 + iota
	NotEqual
	LessThan
	GreaterThan
	LessOrEqual
	GreaterOrEqual
	Contains
	NotContain
	StartsWith
	Includes
	Excludes
	Within
)
var filterOperations = [...]string {
	"equals",
	"notEqual",
	"lessThan",
	"greaterThan",
	"lessOrEqual",
	"greaterOrEqual",
	"contains",
	"notContain",
	"startsWith",
	"includes",
	"excludes",
	"within",
}
func (fo FilterOperation) String() string { return filterOperations[fo - 1] }

type PickList struct {
	ControllingField 			string
	PicklistValues 				[]PicklistValue
	Sorted 						bool
}

type FilterItem struct {
	Field 			string
	Operation 		FilterOperation
	Value 			string
	ValueField 		string
}

type LookupFilter struct {
	Active 				bool
	BooleanFilter 		string
	Description 		string
	ErrorMessage 		string
	FilterItems 		[]FilterItem
	InfoMessage 		string
	IsOptional 			bool
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
			util.ErrorAndExit(fmt.Sprintf("validation error: %s:%s is not a valid option for field type %s", name, value, typ))
		} else {
			newOptions[field.Tag.Get("xml")] = options[name]
		}
	}

	// validate required attributes
	s := requiredDefaults
	tod := s.Type()
	for i := 0; i < s.NumField(); i++ {
		_, ok := options[strings.ToLower(tod.Field(i).Name)]
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
			newOptions[tod.Field(i).Tag.Get("xml")] = options[strings.ToLower(tod.Field(i).Name)]
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
	case "phone":
		attrs = getAttributes(&PhoneField{})
		s = reflect.ValueOf(&PhoneFieldRequired{}).Elem()
		break
	case "email", "url":
		attrs = getAttributes(&StringField{})
		s = reflect.ValueOf(&EmailFieldRequired{}).Elem()
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
		//util.ErrorAndExit(fmt.Sprintf("Field type %s is not implemented.", typ))
		break
	}

	newOptions, err = ValidateOptionsAndDefaults(typ, attrs, s, options)

	return newOptions, nil
}

func NewForceMetadata(force *Force) (fm *ForceMetadata) {
	fm = &ForceMetadata{ApiVersion: apiVersionNumber, Force: force}
	return
}
*/
