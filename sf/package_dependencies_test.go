package sf

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPackageVersionDisplayDependencies(t *testing.T) {
	org := devHubSetup(t)
	transitive := true
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		query := strings.Join(strings.Fields(r.URL.Query().Get("q")), " ")
		var records []string
		version := func(id, name string, minor int) string {
			return fmt.Sprintf(`{"SubscriberPackageVersionId":"%s","Package2":{"Name":"%s"},"MajorVersion":1,"MinorVersion":%d,"PatchVersion":0,"BuildNumber":3}`, id, name, minor)
		}
		switch {
		case strings.HasPrefix(query, "SELECT Id FROM Package2Version WHERE SubscriberPackageVersionId = '04t000000000001AAA'"):
			records = []string{`{"Id":"05i000000000001AAA"}`}
		case strings.HasPrefix(query, "SELECT Id FROM Package2VersionCreateRequest WHERE Package2VersionId = '05i000000000001AAA'"):
			records = []string{`{"Id":"08c000000000001AAA"}`}
		case strings.HasPrefix(query, "SELECT CalcTransitiveDependencies") && strings.Contains(query, "08c000000000001AAA"):
			if transitive {
				records = []string{`{"CalcTransitiveDependencies":true,"DependencyGraphJson":"{\"creator\":\"x\",\"nodes\":[{\"id\":\"VERSION_BEING_BUILT\"},{\"id\":\"04t000000000002AAA\"},{\"id\":\"04t000000000003AAA\"}],\"edges\":[{\"source\":\"04t000000000002AAA\",\"target\":\"VERSION_BEING_BUILT\"},{\"source\":\"04t000000000003AAA\",\"target\":\"04t000000000002AAA\"}]}"}`}
			} else {
				records = []string{`{"CalcTransitiveDependencies":false,"DependencyGraphJson":null}`}
			}
		case strings.HasPrefix(query, "SELECT Package2Version.SubscriberPackageVersionId"):
			records = []string{`{"Package2":{"Name":"Billing"},"Package2Version":{"SubscriberPackageVersionId":"04t000000000001AAA","MajorVersion":2,"MinorVersion":0,"PatchVersion":0,"BuildNumber":1}}`}
		case strings.HasPrefix(query, "SELECT Dependencies FROM SubscriberPackageVersion WHERE Id = '04t000000000001AAA'"):
			records = []string{`{"Dependencies":{"ids":[{"subscriberPackageVersionId":"04t000000000002AAA"}]}}`}
		case strings.Contains(query, "FROM Package2Version WHERE SubscriberPackageVersionId = '04t000000000001AAA'"):
			records = []string{version("04t000000000001AAA", "Billing", 9)}
		case strings.Contains(query, "FROM Package2Version WHERE SubscriberPackageVersionId = '04t000000000002AAA'"):
			records = []string{version("04t000000000002AAA", "Base", 1)}
		case strings.Contains(query, "FROM Package2Version WHERE SubscriberPackageVersionId = '04t000000000003AAA'"):
			records = []string{version("04t000000000003AAA", "Core", 2)}
		}
		io.WriteString(w, fmt.Sprintf(`{"done":true,"totalSize":%d,"records":[%s]}`, len(records), strings.Join(records, ",")))
	}

	expectRun(t, 0, "strict digraph G {\n"+
		"\t node_04t000000000001AAA [label=\"Billing@2.0.0.1\" color=\"green\"]\n"+
		"\t node_04t000000000002AAA [label=\"Base@1.1.0.3\" color=\"green\"]\n"+
		"\t node_04t000000000003AAA [label=\"Core@1.2.0.3\"]\n"+
		"\t node_04t000000000001AAA -> node_04t000000000002AAA\n"+
		"\t node_04t000000000002AAA -> node_04t000000000003AAA\n"+
		"}\n", "package", "version", "displaydependencies", "-p", "08c000000000001AAA")
	out, _, status := run("package", "version", "displaydependencies", "-p", "04t000000000001AAA", "--edge-direction", "root-last", "--verbose")
	if status != 0 || !strings.Contains(out, "\t node_04t000000000002AAA [label=\"Base@1.1.0.3 (04t000000000002AAA)\" color=\"green\"]\n") ||
		!strings.Contains(out, "\t node_04t000000000002AAA -> node_04t000000000001AAA\n") {
		t.Errorf("root-last: status %d\n%s", status, out)
	}
	out, _, _ = run("package", "version", "displaydependencies", "-p", "08c000000000001AAA", "--json")
	if !strings.Contains(out, `"result": "strict digraph G {\n\t node_04t000000000001AAA [label=\"Billing@2.0.0.1\" color=\"green\"]\n`) {
		t.Errorf("json:\n%s", out)
	}

	// Without calculated transitive dependencies the graph has the direct
	// dependencies of a version, and a create request cannot be shown.
	transitive = false
	expectRun(t, 0, "strict digraph G {\n"+
		"\t node_04t000000000001AAA [label=\"Billing@1.9.0.3\"]\n"+
		"\t node_04t000000000002AAA [label=\"Base@1.1.0.3\"]\n"+
		"\t node_04t000000000001AAA -> node_04t000000000002AAA\n"+
		"}\n", "package", "version", "displaydependencies", "-p", "04t000000000001AAA")
	_, errOut, status := run("package", "version", "displaydependencies", "-p", "08c000000000001AAA")
	if status != 1 || !strings.HasPrefix(errOut, "Error (NoDependencyGraphJsonMustProvideVersionError): Can't display package dependencies. This Package2VersionCreateRequest does not have CalcTransitiveDependencies set to true.") {
		t.Errorf("flat request: status %d, stderr %q", status, errOut)
	}
	for _, id := range []string{"04t000000000009AAA", "08c000000000009AAA", "0Ho000000000001AAA"} {
		_, errOut, status = run("package", "version", "displaydependencies", "-p", id)
		if status != 1 || !strings.Contains(errOut, "Can't display package dependencies. The package version ID "+id+" you specified is invalid.") {
			t.Errorf("%s: status %d, stderr %q", id, status, errOut)
		}
	}
	if _, errOut, status = run("package", "version", "displaydependencies", "-p", "04t000000000001AAA", "--edge-direction", "up"); status == 0 || !strings.Contains(errOut, "Expected --edge-direction=up to be one of: root-first, root-last") {
		t.Errorf("direction: status %d, stderr %q", status, errOut)
	}
}
