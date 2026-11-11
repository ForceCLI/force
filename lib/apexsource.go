package lib

import (
	"fmt"
	"strings"
)

// ApexSource is the deployed source of an Apex class or trigger.
type ApexSource struct {
	ID              string
	Name            string
	NamespacePrefix string
	Trigger         bool
	Body            string
	// Hidden is true when the org does not return the source, as for
	// classes in managed packages.
	Hidden bool
}

// apexSourceQueryBatch is the number of IDs per query. It keeps the query
// string well under the URL length limit.
const apexSourceQueryBatch = 100

// ApexSourceByID fetches the source of Apex classes (IDs starting with
// 01p) and triggers (IDs starting with 01q). IDs may have 15 or 18
// characters. The result is keyed by 15-character ID. IDs that are not
// found are left out of the result.
func (f *Force) ApexSourceByID(ids []string) (map[string]ApexSource, error) {
	var classIDs, triggerIDs []string
	seen := map[string]bool{}
	for _, id := range ids {
		if len(id) < 15 {
			continue
		}
		id15 := id[:15]
		if seen[id15] {
			continue
		}
		seen[id15] = true
		switch id15[:3] {
		case "01p":
			classIDs = append(classIDs, id15)
		case "01q":
			triggerIDs = append(triggerIDs, id15)
		}
	}
	result := map[string]ApexSource{}
	if err := f.queryApexSource("ApexClass", classIDs, false, result); err != nil {
		return nil, err
	}
	if err := f.queryApexSource("ApexTrigger", triggerIDs, true, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (f *Force) queryApexSource(sobject string, ids []string, trigger bool, result map[string]ApexSource) error {
	for start := 0; start < len(ids); start += apexSourceQueryBatch {
		end := min(start+apexSourceQueryBatch, len(ids))
		soql := fmt.Sprintf("SELECT Id, Name, NamespacePrefix, Body FROM %s WHERE Id IN ('%s')",
			sobject, strings.Join(ids[start:end], "','"))
		records, err := f.Query(soql)
		if err != nil {
			return fmt.Errorf("querying %s source: %w", sobject, err)
		}
		for _, r := range records.Records {
			id, _ := r["Id"].(string)
			if len(id) < 15 {
				continue
			}
			source := ApexSource{ID: id, Trigger: trigger}
			source.Name, _ = r["Name"].(string)
			source.NamespacePrefix, _ = r["NamespacePrefix"].(string)
			source.Body, _ = r["Body"].(string)
			if source.Body == "(hidden)" {
				source.Body = ""
				source.Hidden = true
			}
			result[id[:15]] = source
		}
	}
	return nil
}
