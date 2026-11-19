package sf

import (
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
)

var listQueryPattern = regexp.MustCompile(`<queries>(?:<folder>([^<]*)</folder>)?<type>([^<]*)</type></queries>`)

// serveOrgMetadata makes the fake org describe four metadata types and list
// components for them.  It returns the queries the org received and the API
// versions they were sent with.
func serveOrgMetadata(org *fakeOrg) (*[]string, *[]string) {
	var queries, versions []string
	listed := map[string]string{
		"ApexClass": `<result><fullName>Greeter</fullName><fileName>classes/Greeter.cls</fileName><manageableState>unmanaged</manageableState><type>ApexClass</type></result>` +
			`<result><fullName>acme__Billing</fullName><fileName>classes/acme__Billing.cls</fileName><manageableState>installed</manageableState><namespacePrefix>acme</namespacePrefix><type>ApexClass</type></result>` +
			`<result><fullName>Tools</fullName><fileName>classes/Tools.cls</fileName><manageableState>installedEditable</manageableState><type>ApexClass</type></result>`,
		"CustomObject": `<result><fullName>Invoice__c</fullName><fileName>objects/Invoice__c.object</fileName><manageableState>unmanaged</manageableState><type>CustomObject</type></result>`,
		"CustomField":  `<result><fullName>Invoice__c.Amount__c</fullName><fileName>objects/Invoice__c.object</fileName><manageableState>unmanaged</manageableState><type>CustomField</type></result>`,
		"CustomLabels": `<result><fullName>CustomLabels</fullName><fileName>labels/CustomLabels.labels</fileName><type>CustomLabels</type></result>`,
		"CustomLabel":  `<result><fullName>CustomLabels.Welcome</fullName><fileName>labels/CustomLabels.labels</fileName><manageableState>unmanaged</manageableState><type>CustomLabel</type></result>`,
		"ReportFolder": `<result><fullName>Sales</fullName><fileName>reports/Sales</fileName><manageableState>unmanaged</manageableState><type>ReportFolder</type></result>`,
		"Report:Sales": `<result><fullName>Sales/Pipeline</fullName><fileName>reports/Sales/Pipeline.report</fileName><manageableState>unmanaged</manageableState><type>Report</type></result>`,
		// The org lists the folders of email templates under two names.
		"EmailFolder":         `<result><fullName>unfiled$public</fullName><fileName>unfiled$public</fileName><manageableState>unmanaged</manageableState><type>EmailFolder</type></result>`,
		"EmailTemplateFolder": `<result><fullName>unfiled$public</fullName><fileName>unfiled$public</fileName><manageableState>unmanaged</manageableState><type>EmailTemplateFolder</type></result>`,
	}
	for _, version := range []string{"67.0", "62.0"} {
		version := version
		org.handlers["/services/Soap/m/"+version] = func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "text/xml")
			if strings.Contains(string(body), "describeMetadata") {
				io.WriteString(w, soapBody(`<describeMetadataResponse><result>`+
					`<metadataObjects><directoryName>classes</directoryName><inFolder>false</inFolder><suffix>cls</suffix><xmlName>ApexClass</xmlName></metadataObjects>`+
					`<metadataObjects><childXmlNames>CustomField</childXmlNames><directoryName>objects</directoryName><inFolder>false</inFolder><suffix>object</suffix><xmlName>CustomObject</xmlName></metadataObjects>`+
					`<metadataObjects><childXmlNames>CustomLabel</childXmlNames><directoryName>labels</directoryName><inFolder>false</inFolder><suffix>labels</suffix><xmlName>CustomLabels</xmlName></metadataObjects>`+
					`<metadataObjects><directoryName>reports</directoryName><inFolder>true</inFolder><suffix>report</suffix><xmlName>Report</xmlName></metadataObjects>`+
					`<organizationNamespace></organizationNamespace><partialSaveAllowed>true</partialSaveAllowed><testRequired>false</testRequired></result></describeMetadataResponse>`))
				return
			}
			var results strings.Builder
			for _, match := range listQueryPattern.FindAllStringSubmatch(string(body), -1) {
				query := match[2]
				if match[1] != "" {
					query += ":" + match[1]
				}
				queries = append(queries, query)
				versions = append(versions, version)
				results.WriteString(listed[query])
			}
			io.WriteString(w, soapBody(`<listMetadataResponse>`+results.String()+`</listMetadataResponse>`))
		}
	}
	return &queries, &versions
}

func TestProjectGenerateManifestFromOrg(t *testing.T) {
	org := loggedInFakeOrg(t)
	queries, versions := serveOrgMetadata(org)
	project := newProject(t, os.Getenv("HOME"))
	os.WriteFile(project+"/sfdx-project.json", []byte(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`), 0o644)

	// Compared with sf against a scratch org: the layout of the manifest,
	// the folder entries, the labels and the components of packages.
	expectRun(t, 0, "successfully wrote package.xml\n", "project", "generate", "manifest", "--from-org", "dev")
	manifest, _ := os.ReadFile(project + "/package.xml")
	want := `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>Greeter</members>
        <name>ApexClass</name>
    </types>
    <types>
        <members>Invoice__c.Amount__c</members>
        <name>CustomField</name>
    </types>
    <types>
        <members>Welcome</members>
        <name>CustomLabel</name>
    </types>
    <types>
        <members>CustomLabels</members>
        <name>CustomLabels</name>
    </types>
    <types>
        <members>Invoice__c</members>
        <name>CustomObject</name>
    </types>
    <types>
        <members>unfiled$public</members>
        <name>EmailTemplate</name>
    </types>
    <types>
        <members>Sales/</members>
        <members>Sales/Pipeline</members>
        <name>Report</name>
    </types>
    <version>62.0</version>
</Package>
`
	if string(manifest) != want {
		t.Errorf("package.xml =\n%s\nwant\n%s", manifest, want)
	}
	// The org is asked with the API version of the project.
	for _, version := range *versions {
		if version != "62.0" {
			t.Errorf("a query used API version %s", version)
			break
		}
	}
	joined := strings.Join(*queries, " ")
	for _, query := range []string{"ApexClass", "ReportFolder", "Report:Sales", "CustomField", "CustomLabel"} {
		if !strings.Contains(" "+joined+" ", " "+query+" ") {
			t.Errorf("no query for %s: %s", query, joined)
		}
	}

	// The components of packages are listed when their kind is included.
	run("project", "generate", "manifest", "--from-org", "dev", "-c", "managed")
	manifest, _ = os.ReadFile(project + "/package.xml")
	if !strings.Contains(string(manifest), "<members>acme__Billing</members>") || strings.Contains(string(manifest), "<members>Tools</members>") {
		t.Errorf("managed packages:\n%s", manifest)
	}
	run("project", "generate", "manifest", "--from-org", "dev", "-c", "managed", "-c", "unlocked")
	manifest, _ = os.ReadFile(project + "/package.xml")
	if !strings.Contains(string(manifest), "<members>acme__Billing</members>") || !strings.Contains(string(manifest), "<members>Tools</members>") {
		t.Errorf("all packages:\n%s", manifest)
	}

	// --metadata chooses among the components of the org, by type and by
	// a pattern for the name, and --excluded-metadata leaves types out.
	*queries = nil
	run("project", "generate", "manifest", "--from-org", "dev", "-m", "ApexClass:Gr*")
	manifest, _ = os.ReadFile(project + "/package.xml")
	if !strings.Contains(string(manifest), "<members>Greeter</members>\n        <name>ApexClass</name>") || strings.Contains(string(manifest), "Gr*") || strings.Contains(string(manifest), "CustomObject") || strings.Join(*queries, " ") != "ApexClass" {
		t.Errorf("--metadata: queries %q\n%s", *queries, manifest)
	}
	run("project", "generate", "manifest", "--from-org", "dev", "--excluded-metadata", "ApexClass", "--excluded-metadata", "Report")
	manifest, _ = os.ReadFile(project + "/package.xml")
	if strings.Contains(string(manifest), "<name>ApexClass</name>") || strings.Contains(string(manifest), "<name>Report</name>") || !strings.Contains(string(manifest), "<name>CustomObject</name>") {
		t.Errorf("--excluded-metadata:\n%s", manifest)
	}

	_, errOut, status := run("project", "generate", "manifest", "-c", "managed", "-m", "ApexClass")
	if status != 2 || !strings.Contains(errOut, "All of the following must be provided when using --include-packages: --from-org") {
		t.Errorf("--include-packages without --from-org: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("project", "generate", "manifest", "--from-org", "nobody")
	if status != 2 || !strings.Contains(errOut, "Error (NamedOrgNotFoundError): Parsing --from-org \n\tNo authorization information found for nobody.") {
		t.Errorf("unknown org: status %d, stderr %q", status, errOut)
	}
}

// Compared with sf: the names suggested for a metadata type name that sf
// does not know, in the order sf lists names that are equally close.
func TestTypeNameSuggestions(t *testing.T) {
	for name, want := range map[string]string{
		"ApexClas":   "Did you mean one of the following types? [ApexClass]",
		"Layuot":     "Did you mean one of the following types? [Layout]",
		"custmfield": "Did you mean one of the following types? [CustomLabels,CustomSite]",
	} {
		got := typeNameSuggestions(name)
		if len(got) != 2 || got[0] != want || got[1] != "" {
			t.Errorf("typeNameSuggestions(%q) = %q, want %q and an empty line", name, got, want)
		}
	}
}
