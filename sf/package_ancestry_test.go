package sf

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// The fake Dev Hub has one managed package with the released versions
//
//	1.0.0.1 ─┬─ 1.1.0.1 ─── 1.2.0.1
//	         └─ 1.0.1.1
//
// and an unlocked package.
func ancestryOrg(t *testing.T) *fakeOrg {
	org := devHubSetup(t)
	version := func(id, ancestor string, major, minor, patch int) string {
		parent := "null"
		if ancestor != "" {
			parent = `"` + ancestor + `"`
		}
		return fmt.Sprintf(`{"AncestorId":%s,"SubscriberPackageVersionId":"%s","MajorVersion":%d,"MinorVersion":%d,"PatchVersion":%d,"BuildNumber":1}`, parent, id, major, minor, patch)
	}
	versions := map[string]string{
		"04t000000000001AAA": version("04t000000000001AAA", "", 1, 0, 0),
		"04t000000000002AAA": version("04t000000000002AAA", "04t000000000001AAA", 1, 1, 0),
		"04t000000000003AAA": version("04t000000000003AAA", "04t000000000001AAA", 1, 0, 1),
		"04t000000000004AAA": version("04t000000000004AAA", "04t000000000002AAA", 1, 2, 0),
	}
	children := map[string][]string{
		"04t000000000001AAA": {"04t000000000002AAA", "04t000000000003AAA"},
		"04t000000000002AAA": {"04t000000000004AAA"},
	}
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		var records []string
		switch {
		case strings.Contains(query, "FROM Package2 WHERE Id = '0Ho000000000001AAA'"):
			records = []string{`{"ContainerOptions":"Managed"}`}
		case strings.Contains(query, "FROM Package2 WHERE Id = '0Ho000000000002AAA'"):
			records = []string{`{"ContainerOptions":"Unlocked"}`}
		case strings.Contains(query, "SELECT Package2.ContainerOptions"):
			records = []string{`{"Package2":{"ContainerOptions":"Managed"}}`}
		case strings.Contains(query, "AncestorId = NULL AND Package2Id = '0Ho000000000001AAA'"):
			records = []string{versions["04t000000000001AAA"]}
		case strings.Contains(query, "WHERE AncestorId = '"):
			id := strings.SplitN(strings.SplitN(query, "AncestorId = '", 2)[1], "'", 2)[0]
			for _, child := range children[id] {
				records = append(records, versions[child])
			}
		case strings.Contains(query, "WHERE SubscriberPackageVersionId = '"):
			id := strings.SplitN(strings.SplitN(query, "SubscriberPackageVersionId = '", 2)[1], "'", 2)[0]
			if record, ok := versions[id]; ok {
				records = []string{record}
			}
		}
		io.WriteString(w, fmt.Sprintf(`{"done":true,"totalSize":%d,"records":[%s]}`, len(records), strings.Join(records, ",")))
	}
	return org
}

func TestPackageVersionDisplayAncestryOfAPackage(t *testing.T) {
	ancestryOrg(t)
	expectRun(t, 0, `└─ 1.0.0.1
   ├─ 1.0.1.1
   └─ 1.1.0.1
      └─ 1.2.0.1
`, "package", "version", "displayancestry", "-p", "0Ho000000000001AAA")
	expectRun(t, 0, `└─ 1.0.0.1 (04t000000000001AAA)
   ├─ 1.0.1.1 (04t000000000003AAA)
   └─ 1.1.0.1 (04t000000000002AAA)
      └─ 1.2.0.1 (04t000000000004AAA)
`, "package", "version", "displayancestry", "-p", "billing", "--verbose")
	expectRun(t, 0, "strict graph G {\n"+
		"\t nodeunknown [label=\"root\"]\n"+
		"\t node04t000000000001AAA [label=\"1.0.0.1\"]\n"+
		"\t node04t000000000003AAA [label=\"1.0.1.1\"]\n"+
		"\t node04t000000000002AAA [label=\"1.1.0.1\"]\n"+
		"\t node04t000000000004AAA [label=\"1.2.0.1\"]\n"+
		"\t nodeunknown -- node04t000000000001AAA\n"+
		"\t node04t000000000001AAA -- node04t000000000003AAA\n"+
		"\t node04t000000000001AAA -- node04t000000000002AAA\n"+
		"\t node04t000000000002AAA -- node04t000000000004AAA\n"+
		"}\n", "package", "version", "displayancestry", "-p", "0Ho000000000001AAA", "--dot-code")
	// The JSON result lists the descendants in the order of sf's
	// traversal: the last one the org returned comes first.
	out, _, status := run("package", "version", "displayancestry", "-p", "0Ho000000000001AAA", "--json")
	first, second := strings.Index(out, "04t000000000003AAA"), strings.Index(out, "04t000000000002AAA")
	if status != 0 || first < 0 || second < first || !strings.Contains(out, `"depthCounter": 2`) || !strings.HasSuffix(strings.TrimSpace(out), "\"depthCounter\": 0\n    }\n  },\n  \"warnings\": []\n}") {
		t.Errorf("json: status %d\n%s", status, out)
	}
}

func TestPackageVersionDisplayAncestryOfAVersion(t *testing.T) {
	ancestryOrg(t)
	expectRun(t, 0, `1.2.0.1 -> 1.1.0.1 -> 1.0.0.1 (root)

└─ 1.2.0.1
`, "package", "version", "displayancestry", "-p", "04t000000000004AAA")
	expectRun(t, 0, `1.1.0.1 -> 1.0.0.1 (root)

└─ 1.1.0.1
   └─ 1.2.0.1
`, "package", "version", "displayancestry", "-p", "04t000000000002AAA")
}

func TestPackageVersionDisplayAncestryErrors(t *testing.T) {
	ancestryOrg(t)
	// The first two were compared with sf.
	for _, c := range []struct {
		id, stderr string
	}{
		{"04t000000000000AAA", "Error (VersionNotFoundError): Can’t display the ancestry tree for 04t000000000000AAA. Verify the package version number (starts with 04t) or the package version alias listed in the sfdx-project.json file, and try creating the ancestry tree again.\n\n\n"},
		{"NoSuchPackage", "Error (IdOrAliasNotFoundError): Can't find package with id or alias: NoSuchPackage\n\n\n"},
		{"0Ho000000000002AAA", "Error (UnlockedPackageError): Can’t display package ancestry. Package ancestry is available only for second-generation managed packages. Retry this command and specify a second-generation managed package or package version.\n\n\n"},
	} {
		_, errOut, status := run("package", "version", "displayancestry", "-p", c.id)
		if status != 1 || errOut != c.stderr {
			t.Errorf("%s: status %d, stderr %q", c.id, status, errOut)
		}
	}
}
