package lib

import (
	"encoding/xml"
	"fmt"

	"github.com/ForceCLI/force/lib/soap"
)

func (partner *ForcePartner) Merge(sobjectType string, masterId string, duplicateId string) (err error) {
	dups := make(map[string]string)
	dups[duplicateId] = masterId
	result, err := partner.MergeMany(sobjectType, dups)
	if err != nil {
		return err
	}
	for _, r := range result {
		if !r.Success {
			for _, e := range r.Errors {
				return fmt.Errorf("Merge failed: %s", e.Message)
			}
		}
	}
	return nil
}

// Merge multiple records using map of duplicate ids to master ids
func (partner *ForcePartner) MergeMany(sobjectType string, dupMap map[string]string) (results []soap.MergeResult, err error) {
	if len(dupMap) > 200 {
		return nil, fmt.Errorf("Only 200 records can be merged at a time")
	}
	mergeRequests := buildMergeRequests(sobjectType, dupMap)
	req, err := xml.Marshal(mergeRequests)
	if err != nil {
		return
	}
	body, err := partner.SoapExecuteCore("merge", string(req))
	if err != nil {
		return
	}
	var response soap.MergeResponse
	if err = xml.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	return response.Result, nil
}

func buildMergeRequests(sobjectType string, dupMap map[string]string) []soap.MergeRequest {
	var mergeRequests []soap.MergeRequest
	for duplicateId, masterId := range dupMap {
		id := soap.ID(masterId)
		var toMerge []*soap.ID
		dup := soap.ID(duplicateId)
		toMerge = append(toMerge, &dup)
		mergeRequest := soap.MergeRequest{
			MasterRecord: &soap.SObject{
				Type_: sobjectType,
				Id:    &id,
			},
			RecordToMergeIds: toMerge,
		}
		mergeRequests = append(mergeRequests, mergeRequest)
	}
	return mergeRequests
}
