package lib

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/ForceCLI/force/lib/soap"
)

func (partner *ForcePartner) Undelete(id string) (err error) {
	deleted := []string{id}
	result, err := partner.UndeleteMany(deleted)
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

func (partner *ForcePartner) UndeleteMany(deleted []string) (results []soap.UndeleteResult, err error) {
	if len(deleted) > 200 {
		return nil, fmt.Errorf("Only 200 records can be undeleted at a time")
	}
	req := buildUndeleteRequest(deleted)
	body, err := partner.SoapExecuteCore("undelete", string(req))
	if err != nil {
		return
	}
	var response soap.UndeleteResponse
	if err = xml.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	return response.Result, nil
}

func buildUndeleteRequest(deleted []string) string {
	var ids []string
	for _, id := range deleted {
		ids = append(ids, fmt.Sprintf("<ids>%s</ids>", id))
	}
	return strings.Join(ids, "")
}
