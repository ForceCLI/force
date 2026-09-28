package query

import (
	"fmt"
	"strings"
	"testing"
)

// pages returns an HttpGetter that serves each body in turn, keyed by the
// URL suffix the queryer requests.
func pages(t *testing.T, bodies map[string]string) HttpGetter {
	return func(url string) ([]byte, error) {
		for suffix, body := range bodies {
			if strings.HasSuffix(url, suffix) {
				return []byte(body), nil
			}
		}
		t.Fatalf("unexpected request for %s", url)
		return nil, fmt.Errorf("unexpected request for %s", url)
	}
}

func TestTotalSizeReportsTheCountOfAnAggregateQueryWithoutRecords(t *testing.T) {
	get := pages(t, map[string]string{
		"?q=SELECT+COUNT%28%29+FROM+Account": `{"done":true,"totalSize":162,"records":[]}`,
	})
	var totalSize int

	records, err := Eager(HttpGet(get), ApiVersion("v66.0"), QS("SELECT COUNT() FROM Account"), TotalSize(&totalSize))

	if err != nil {
		t.Fatalf("Eager returned error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Expected no records, got %d", len(records))
	}
	if totalSize != 162 {
		t.Errorf("Expected totalSize 162, got %d", totalSize)
	}
}

func TestTotalSizeIsTakenFromTheTopLevelQueryNotASubqueryPage(t *testing.T) {
	get := pages(t, map[string]string{
		"?q=SELECT+Id%2C+%28SELECT+Id+FROM+Contacts%29+FROM+Account": `{"done":false,"totalSize":3,"nextRecordsUrl":"/next","records":[
			{"attributes":{"type":"Account"},"Id":"001A","Contacts":{"done":false,"totalSize":50,"nextRecordsUrl":"/contacts","records":[{"attributes":{"type":"Contact"},"Id":"003A"}]}}
		]}`,
		"/contacts": `{"done":true,"totalSize":50,"records":[{"attributes":{"type":"Contact"},"Id":"003B"}]}`,
		"/next":     `{"done":true,"totalSize":3,"records":[{"attributes":{"type":"Account"},"Id":"001B"},{"attributes":{"type":"Account"},"Id":"001C"}]}`,
	})
	var totalSize int

	records, err := Eager(HttpGet(get), ApiVersion("v66.0"), QS("SELECT Id, (SELECT Id FROM Contacts) FROM Account"), TotalSize(&totalSize))

	if err != nil {
		t.Fatalf("Eager returned error: %v", err)
	}
	if len(records) != 3 {
		t.Errorf("Expected 3 records, got %d", len(records))
	}
	if contacts := records[0].Fields["Contacts"].([]Record); len(contacts) != 2 {
		t.Errorf("Expected 2 contacts across subquery pages, got %d", len(contacts))
	}
	if totalSize != 3 {
		t.Errorf("Expected totalSize 3, got %d", totalSize)
	}
}
