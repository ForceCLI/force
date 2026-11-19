package sf

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/desktop"
)

const securityWarning = "This command will expose sensitive information that allows for subsequent activity using your current authenticated session.\nSharing this information is equivalent to logging someone in under the current credential, resulting in unintended access and escalation of privilege.\nFor additional information, please review the authorization section of the https://developer.salesforce.com/docs/atlas.en-us.sfdx_dev.meta/sfdx_dev/sfdx_dev_auth_web_flow.htm."

type orgOpenResult struct {
	OrgId    string `json:"orgId"`
	URL      string `json:"url"`
	Username string `json:"username"`
}

// openBrowser is replaced in tests.
var openBrowser = desktop.Open

// browserCommands gives, for each value of --browser, the programs that
// start that browser on Linux.  On macOS and Windows the browser is started
// by its application name.
var browserCommands = map[string][]string{
	"chrome":  {"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"},
	"edge":    {"microsoft-edge", "microsoft-edge-stable", "microsoft-edge-dev"},
	"firefox": {"firefox"},
}

// openInBrowser opens a URL in the named browser.  Tests replace it.
var openInBrowser = func(browser, uri string) error {
	switch runtime.GOOS {
	case "darwin":
		app := map[string]string{"chrome": "google chrome", "edge": "microsoft edge", "firefox": "firefox"}[browser]
		return exec.Command("open", "-a", app, uri).Start()
	case "windows":
		app := map[string]string{"chrome": "chrome", "edge": "msedge", "firefox": "firefox"}[browser]
		return exec.Command("cmd", "/c", "start", "", app, uri).Start()
	}
	for _, name := range browserCommands[browser] {
		if path, err := exec.LookPath(name); err == nil {
			return exec.Command(path, uri).Start()
		}
	}
	return newError("BrowserNotFoundError", "Unable to find the %s browser. Tried: %s", browser, strings.Join(browserCommands[browser], ", "))
}

// sourceFilePath returns the page of the org that shows the metadata in a
// source file: the setup page of an Apex class or a custom object, the
// builder of a flow, a bot or a Lightning page, or a Visualforce page
// itself.  Another metadata file gives the list of Lightning pages, and a
// file that is not metadata gives the home page of the org, as in sf.
func sourceFilePath(org *targetOrg, file string) (string, error) {
	if _, err := os.Stat(file); err != nil {
		return "", nil
	}
	name := filepath.Base(file)
	single := func(tooling bool, soql string) (string, bool) {
		var records []*orderedMap
		var err error
		if tooling {
			records, err = toolingRecords(org, soql)
		} else {
			records, err = restRecords(org, soql)
		}
		if err != nil || len(records) != 1 {
			return "", false
		}
		for _, key := range []string{"Id", "DurableId"} {
			if id := text(records[0].values[key]); id != "" {
				return id, true
			}
		}
		return "", false
	}
	switch {
	case strings.HasSuffix(name, ".cls") || strings.HasSuffix(name, ".cls-meta.xml"):
		id, ok := single(true, fmt.Sprintf("SELECT Id FROM ApexClass WHERE Name = '%s'", strings.TrimSuffix(name, ".cls")))
		if !ok {
			return "", newError("ApexClassIdNotFoundError", "ID not found for Apex class %s.", file)
		}
		return "lightning/setup/ApexClasses/page?address=%2F" + id, nil
	case strings.HasSuffix(name, ".object-meta.xml"):
		id, ok := single(true, fmt.Sprintf("SELECT Id FROM CustomObject WHERE DeveloperName = '%s'", strings.TrimSuffix(strings.ReplaceAll(name, "__c", ""), ".object-meta.xml")))
		if !ok {
			return "", newError("CustomObjectIdNotFoundError", "ID not found for custom object %s.", file)
		}
		return "lightning/setup/ObjectManager/" + id + "/Details/view", nil
	case strings.HasSuffix(name, ".bot-meta.xml"):
		id, ok := single(false, fmt.Sprintf("SELECT id FROM BotDefinition WHERE DeveloperName='%s'", strings.TrimSuffix(name, ".bot-meta.xml")))
		if !ok {
			return "", nil
		}
		return "/AiCopilot/copilotStudio.app#/copilot/builder?copilotId=" + id, nil
	case strings.HasSuffix(name, ".page") || strings.HasSuffix(name, ".page-meta.xml"):
		return "/apex/" + strings.TrimSuffix(strings.TrimSuffix(name, ".page-meta.xml"), ".page"), nil
	case strings.HasSuffix(name, ".flow-meta.xml"):
		id, ok := single(false, fmt.Sprintf("SELECT DurableId FROM FlowVersionView WHERE FlowDefinitionView.ApiName = '%s' ORDER BY VersionNumber DESC LIMIT 1", strings.TrimSuffix(name, ".flow-meta.xml")))
		if !ok {
			return "", newError("FlowIdNotFoundError", "ID not found for Flow %s.", file)
		}
		return "/builder_platform_interaction/flowBuilder.app?flowId=" + id, nil
	case strings.HasSuffix(name, ".flexipage-meta.xml"):
		id, ok := single(true, fmt.Sprintf("SELECT id FROM flexipage WHERE DeveloperName='%s'", strings.TrimSuffix(name, ".flexipage-meta.xml")))
		if !ok {
			return "", nil
		}
		return "/visualEditor/appBuilder.app?pageId=" + id, nil
	}
	if strings.HasSuffix(name, "-meta.xml") {
		return "/lightning/setup/FlexiPageList/home", nil
	}
	if _, err := os.Stat(file + "-meta.xml"); err == nil {
		return "/lightning/setup/FlexiPageList/home", nil
	}
	return "", nil
}

func newOrgOpenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open your default scratch org, or another specified org, in a browser.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := sandboxFlagConflicts(cmd, [][2]string{{"private", "url-only"}, {"private", "browser"}, {"browser", "url-only"}}); err != nil {
				return err
			}
			browser, _ := cmd.Flags().GetString("browser")
			if browser != "" && browserCommands[browser] == nil {
				return &Error{Message: fmt.Sprintf("Expected --browser=%s to be one of: chrome, edge, firefox\nSee more help with --help", browser), ExitCode: 2}
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			path, _ := cmd.Flags().GetString("path")
			if path == "" {
				path = os.Getenv("FORCE_OPEN_URL")
			}
			if sourceFile, _ := cmd.Flags().GetString("source-file"); sourceFile != "" {
				if path, err = sourceFilePath(org, sourceFile); err != nil {
					return err
				}
			}
			// sf requests a single-use URL from the UI Bridge API.
			request := "/services/oauth2/singleaccess"
			if path != "" {
				request += "?redirect_uri=" + url.QueryEscape(path)
			}
			body, err := org.Force.GetAbsolute(request)
			if err != nil {
				return apiError(err)
			}
			var response struct {
				FrontdoorURI string `json:"frontdoor_uri"`
			}
			if err := json.Unmarshal([]byte(body), &response); err != nil || response.FrontdoorURI == "" {
				return newError("FrontdoorURLError", "Failed to generate a frontdoor URL.")
			}
			result := orgOpenResult{OrgId: org.Fields.String("orgId"), URL: response.FrontdoorURI, Username: org.Username}
			urlOnly, _ := cmd.Flags().GetBool("url-only")
			if urlOnly {
				warn(cmd, securityWarning)
				return emit(cmd, result, func(w io.Writer) {
					fmt.Fprintln(w)
					fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Access org %s as user %s with the following URL: %s", result.OrgId, result.Username, result.URL)))
				})
			}
			if err := emit(cmd, result, func(w io.Writer) {
				fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Opening org %s as user %s", result.OrgId, result.Username)))
			}); err != nil {
				return err
			}
			if browser != "" {
				return openInBrowser(browser, result.URL)
			}
			if private, _ := cmd.Flags().GetBool("private"); private {
				return openPrivately(result.URL)
			}
			return openBrowser(result.URL)
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().Bool("private", false, "Open the org in the default browser using private (incognito) mode.")
	cmd.Flags().StringP("browser", "b", "", "Browser where the org opens.")
	cmd.Flags().StringP("path", "p", "", "Navigation URL path to open a specific page.")
	cmd.Flags().BoolP("url-only", "r", false, "Display navigation URL, but don’t launch browser.")
	cmd.Flags().StringP("source-file", "f", "", "Path to an ApexPage or FlexiPage to open in Lightning App Builder.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

func newOrgListLimitsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "limits",
		Short:       "Display information about limits in your org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "LimitsApiDisplayCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			body, err := org.Force.GetREST("/limits")
			if err != nil {
				// sf wraps this error, which drops the "data" field
				// and keeps the error name as the code.
				converted := apiError(err)
				if e, ok := converted.(*Error); ok {
					e.Label = ""
					e.Data = nil
				}
				return converted
			}
			parsed, err := parseOrderedJSON([]byte(body))
			if err != nil {
				return err
			}
			limits, ok := parsed.(*orderedMap)
			if !ok {
				return fmt.Errorf("unexpected response from the limits resource")
			}
			result := make([]*orderedMap, 0, len(limits.keys))
			rows := make([][]string, 0, len(limits.keys))
			for _, name := range limits.keys {
				limit, ok := limits.values[name].(*orderedMap)
				if !ok {
					continue
				}
				item := newOrderedMap()
				item.Set("name", name)
				item.Set("max", limit.values["Max"])
				item.Set("remaining", limit.values["Remaining"])
				result = append(result, item)
				rows = append(rows, []string{name, displayValue(limit.values["Remaining"]), displayValue(limit.values["Max"])})
			}
			return emit(cmd, result, func(w io.Writer) {
				printTable(w, "", []string{"Name", "Remaining", "Max"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

// soapResults returns the <result> elements of a SOAP response as ordered
// maps of their child elements.  A child element that repeats becomes an
// array.
func soapResults(body []byte) ([]*orderedMap, error) {
	dec := xml.NewDecoder(bytes.NewReader(body))
	var results []*orderedMap
	for {
		token, err := dec.Token()
		if err == io.EOF {
			return results, nil
		}
		if err != nil {
			return nil, err
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "result" {
			value, err := decodeXMLElement(dec)
			if err != nil {
				return nil, err
			}
			if object, ok := value.(*orderedMap); ok {
				results = append(results, object)
			}
		}
	}
}

// decodeXMLElement reads the content of the element whose start tag was
// just read.  An element with child elements becomes an ordered map; other
// elements become their text.
func decodeXMLElement(dec *xml.Decoder) (interface{}, error) {
	object := newOrderedMap()
	var text strings.Builder
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			child, err := decodeXMLElement(dec)
			if err != nil {
				return nil, err
			}
			// An element marked xsi:nil has no value.
			for _, attr := range t.Attr {
				if attr.Name.Local == "nil" && attr.Value == "true" {
					child = nil
				}
			}
			name := t.Name.Local
			if existing, ok := object.Get(name); ok {
				if list, isList := existing.([]interface{}); isList {
					object.Set(name, append(list, child))
				} else {
					object.Set(name, []interface{}{existing, child})
				}
			} else {
				object.Set(name, child)
			}
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			if len(object.keys) > 0 {
				return object, nil
			}
			return text.String(), nil
		}
	}
}

func stringField(object *orderedMap, key string) string {
	value, _ := object.values[key].(string)
	return value
}

func newOrgListMetadataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "metadata",
		Short:       "List the metadata components and properties of a specified type.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "ListMetadata"},
		RunE: func(cmd *cobra.Command, args []string) error {
			metadataType, _ := cmd.Flags().GetString("metadata-type")
			if metadataType == "" {
				return missingFlagError("metadata-type")
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			query := metadataType
			if folder, _ := cmd.Flags().GetString("folder"); folder != "" {
				query += ":" + folder
			}
			body, err := org.Force.Metadata.ListMetadata(query)
			if err != nil {
				return apiError(err)
			}
			result, err := soapResults(body)
			if err != nil {
				return err
			}
			if result == nil {
				result = []*orderedMap{}
			}
			// sf reports these properties as empty strings when the
			// response leaves them out.
			for _, item := range result {
				for _, key := range []string{"createdById", "createdByName", "createdDate", "fileName", "fullName", "id", "lastModifiedById", "lastModifiedByName", "lastModifiedDate", "type"} {
					if _, ok := item.Get(key); !ok {
						item.Set(key, "")
					}
				}
			}
			if outputFile, _ := cmd.Flags().GetString("output-file"); outputFile != "" {
				var encoded bytes.Buffer
				enc := json.NewEncoder(&encoded)
				enc.SetEscapeHTML(false)
				enc.SetIndent("", "  ")
				if err := enc.Encode(result); err != nil {
					return err
				}
				if err := os.WriteFile(outputFile, bytes.TrimRight(encoded.Bytes(), "\n"), 0o644); err != nil {
					return err
				}
				return emit(cmd, result, func(w io.Writer) {
					fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Wrote result file to %s.", outputFile)))
				})
			}
			if len(result) == 0 {
				warn(cmd, fmt.Sprintf("No metadata found for type: %s in org: %s.", metadataType, org.Username))
				return emit(cmd, result, nil)
			}
			date := func(value string) string {
				day, _, _ := strings.Cut(value, "T")
				return day
			}
			sorted := append([]*orderedMap{}, result...)
			sort.SliceStable(sorted, func(i, j int) bool {
				return stringField(sorted[i], "manageableState") < stringField(sorted[j], "manageableState")
			})
			rows := make([][]string, 0, len(sorted))
			for _, item := range sorted {
				rows = append(rows, []string{
					stringField(item, "createdByName"),
					date(stringField(item, "createdDate")),
					stringField(item, "fullName"),
					stringField(item, "id"),
					stringField(item, "lastModifiedByName"),
					date(stringField(item, "lastModifiedDate")),
					stringField(item, "manageableState"),
					stringField(item, "namespacePrefix"),
				})
			}
			return emit(cmd, result, func(w io.Writer) {
				printTable(w, metadataType, []string{"Created By", "Created Date", "Full Name", "Id", "Last Modified By", "Last Modified", "Manageable State", "Namespace Prefix"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("output-file", "f", "", "File where the results of the command are stored.")
	cmd.Flags().StringP("metadata-type", "m", "", "Metadata type to be retrieved, such as CustomObject.")
	cmd.Flags().String("folder", "", "Folder associated with the component.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

type metadataTypeResult struct {
	DirectoryName string   `json:"directoryName"`
	InFolder      bool     `json:"inFolder"`
	MetaFile      bool     `json:"metaFile"`
	Suffix        string   `json:"suffix,omitempty"`
	XmlName       string   `json:"xmlName"`
	ChildXmlNames []string `json:"childXmlNames"`
}

type metadataTypesResult struct {
	MetadataObjects       []metadataTypeResult `json:"metadataObjects"`
	OrganizationNamespace string               `json:"organizationNamespace"`
	PartialSaveAllowed    bool                 `json:"partialSaveAllowed"`
	TestRequired          bool                 `json:"testRequired"`
}

func newOrgListMetadataTypesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "metadata-types",
		Short:       "Display details about the metadata types that are enabled for your org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "ListMetadataTypes"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			describe, err := org.Force.Metadata.DescribeMetadata()
			if err != nil {
				return apiError(err)
			}
			result := metadataTypesResult{
				OrganizationNamespace: describe.NamespacePrefix,
				PartialSaveAllowed:    describe.PartialSaveAllowed,
				TestRequired:          describe.TestRequired,
				MetadataObjects:       make([]metadataTypeResult, 0, len(describe.MetadataObjects)),
			}
			for _, object := range describe.MetadataObjects {
				children := object.ChildXmlNames
				if children == nil {
					children = []string{}
				}
				result.MetadataObjects = append(result.MetadataObjects, metadataTypeResult{
					DirectoryName: object.DirectoryName,
					InFolder:      object.InFolder,
					MetaFile:      object.MetaFile,
					Suffix:        object.Suffix,
					XmlName:       object.XmlName,
					ChildXmlNames: children,
				})
			}
			if outputFile, _ := cmd.Flags().GetString("output-file"); outputFile != "" {
				encoded, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return err
				}
				if err := os.WriteFile(outputFile, encoded, 0o644); err != nil {
					return err
				}
				return emit(cmd, result, func(w io.Writer) {
					fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Wrote result file to %s.", outputFile)))
				})
			}
			sorted := append([]metadataTypeResult{}, result.MetadataObjects...)
			sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].XmlName < sorted[j].XmlName })
			rows := make([][]string, 0, len(sorted))
			for _, object := range sorted {
				children := ""
				if len(object.ChildXmlNames) > 0 {
					children = "[ " + strings.Join(object.ChildXmlNames, " ") + " ]"
				}
				rows = append(rows, []string{
					object.XmlName, children, object.DirectoryName,
					fmt.Sprintf("%t", object.InFolder), fmt.Sprintf("%t", object.MetaFile), object.Suffix,
				})
			}
			return emit(cmd, result, func(w io.Writer) {
				printTable(w, "Metadata", []string{"Xml Names", "Child Xml Names", "Directory Name", "In Folder", "Meta File", "Suffix"}, rows)
				fmt.Fprintf(w, "Organizational Namespace: %s\n", result.OrganizationNamespace)
				fmt.Fprintf(w, "Partial Save Allowed: %t\n", result.PartialSaveAllowed)
				fmt.Fprintf(w, "Test Required: %t\n", result.TestRequired)
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("output-file", "f", "", "File where the results of the command are stored.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}
