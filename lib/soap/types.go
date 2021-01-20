package soap

import (
	"encoding/xml"
)

// Types converted from partner WSDL

type SObject struct {
	Type_        string   `xml:"type,omitempty"`
	FieldsToNull []string `xml:"fieldsToNull,omitempty"`
	Id           *ID      `xml:"Id,omitempty"`
	Items        []string `xml:",any"`
}

type Merge struct {
	XMLName xml.Name        `xml:"urn:partner.soap.sforce.com merge"`
	Request []*MergeRequest `xml:"request,omitempty"`
}

type AdditionalInformationMap struct {
	Name  string `xml:"name,omitempty"`
	Value string `xml:"value,omitempty"`
}

type MergeRequest struct {
	XMLName                  xml.Name                    `xml:"request"`
	AdditionalInformationMap []*AdditionalInformationMap `xml:"additionalInformationMap,omitempty"`
	MasterRecord             *SObject                    `xml:"masterRecord,omitempty"`
	RecordToMergeIds         []*ID                       `xml:"recordToMergeIds,omitempty"`
}

type MergeResponse struct {
	Result []MergeResult `xml:"Body>mergeResponse>result"`
}

type MergeResult struct {
	Errors            []Error `xml:"errors,omitempty"`
	Id                ID      `xml:"id,omitempty"`
	MergedRecordIds   []ID    `xml:"mergedRecordIds,omitempty"`
	Success           bool    `xml:"success,omitempty"`
	UpdatedRelatedIds []ID    `xml:"updatedRelatedIds,omitempty"`
}

type UndeleteResponse struct {
	Result []UndeleteResult `xml:"Body>undeleteResponse>result"`
}

type UndeleteResult struct {
	Errors  []Error `xml:"errors,omitempty"`
	Id      ID      `xml:"id,omitempty"`
	Success bool    `xml:"success,omitempty"`
}

type Error struct {
	ExtendedErrorDetails []ExtendedErrorDetails `xml:"extendedErrorDetails,omitempty"`
	Fields               []string               `xml:"fields,omitempty"`
	Message              string                 `xml:"message,omitempty"`
	StatusCode           StatusCode             `xml:"statusCode,omitempty"`
}

type ExtendedErrorDetails struct {
	ExtendedErrorCode ExtendedErrorCode `xml:"extendedErrorCode,omitempty"`
	Items             []string          `xml:",any"`
}

type ID string
type StatusCode string
type ExtendedErrorCode string
