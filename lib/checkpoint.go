package lib

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

// MaxCheckpoints is the number of ApexExecutionOverlayAction records
// Salesforce allows a user to have.
const MaxCheckpoints = 5

// Checkpoint is a line of an Apex class or trigger at which Salesforce saves
// a heap dump when execution reaches it.
type Checkpoint struct {
	// Name is the class or trigger name.
	Name string
	Line int
}

func (c Checkpoint) String() string {
	return fmt.Sprintf("%s:%d", c.Name, c.Line)
}

// CreateCheckpoints creates an ApexExecutionOverlayAction for each
// checkpoint, which saves a heap dump the first time the current user's code
// reaches the line. The returned function deletes them. If one cannot be
// created, the ones already created are deleted.
func (f *Force) CreateCheckpoints(checkpoints []Checkpoint) (remove func() error, err error) {
	var ids []string
	remove = func() error {
		var failed []string
		for _, id := range ids {
			if err := f.DeleteToolingRecord("ApexExecutionOverlayAction", id); err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", id, err))
			}
		}
		if len(failed) > 0 {
			return fmt.Errorf("deleting checkpoints: %s", strings.Join(failed, "; "))
		}
		return nil
	}
	for _, c := range checkpoints {
		body, _ := json.Marshal(map[string]interface{}{
			"ActionScript":         "",
			"ActionScriptType":     "None",
			"ExecutableEntityName": c.Name,
			"IsDumpingHeap":        true,
			"Iteration":            1,
			"Line":                 c.Line,
		})
		response, err := f.PostREST("tooling/sobjects/ApexExecutionOverlayAction", string(body))
		var created ForceCreateRecordResult
		if err == nil {
			err = json.Unmarshal([]byte(response), &created)
		}
		if err == nil && (!created.Success || created.Id == "") {
			err = fmt.Errorf("unexpected response: %s", response)
		}
		if err != nil {
			if removeErr := remove(); removeErr != nil {
				err = fmt.Errorf("%v; %v", err, removeErr)
			}
			return nil, errors.Wrapf(err, "creating checkpoint %s", c)
		}
		ids = append(ids, created.Id)
	}
	return remove, nil
}

// HeapDump returns the HeapDump field of an ApexExecutionOverlayResult, the
// heap dump saved at a checkpoint. A log's HEAP_DUMP events give the IDs.
func (f *Force) HeapDump(id string) (json.RawMessage, error) {
	u := fmt.Sprintf("%s/services/data/%s/tooling/sobjects/ApexExecutionOverlayResult/%s?fields=%s",
		f.Credentials.InstanceUrl, apiVersion, url.PathEscape(id), url.QueryEscape("Id,HeapDump"))
	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(u))
	if err != nil {
		return nil, errors.Wrapf(err, "fetching heap dump %s", id)
	}
	var record struct {
		HeapDump json.RawMessage
	}
	if err := json.Unmarshal(body, &record); err != nil {
		return nil, errors.Wrapf(err, "parsing heap dump %s", id)
	}
	if len(record.HeapDump) == 0 || string(record.HeapDump) == "null" {
		return nil, fmt.Errorf("heap dump %s is empty", id)
	}
	return record.HeapDump, nil
}
