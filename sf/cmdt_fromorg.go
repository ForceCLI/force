package sf

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// cmdtConvertibleTypes are the field types a custom metadata type can
// have.  A field of another type becomes a text field.
var cmdtConvertibleTypes = map[string]bool{
	"Checkbox": true, "Date": true, "DateTime": true, "Email": true, "Number": true, "Percent": true,
	"Phone": true, "Picklist": true, "Text": true, "TextArea": true, "LongTextArea": true, "Url": true,
}

func asList(value interface{}) []interface{} {
	switch v := value.(type) {
	case nil:
		return nil
	case []interface{}:
		return v
	}
	return []interface{}{value}
}

// cmdtFieldFromOrg returns the field file sf writes for a field of an
// object in an org.  The values are written as they are, without XML
// escaping, as sf writes them.
func cmdtFieldFromOrg(field *orderedMap, defaultToText bool) (string, error) {
	content, _, err := cmdtFieldFileFromOrg(field, defaultToText)
	return content, err
}

// cmdtFieldFileFromOrg returns the field file and the type written in it.
func cmdtFieldFileFromOrg(field *orderedMap, defaultToText bool) (string, string, error) {
	get := func(key string) string { return text(field.values[key]) }
	has := func(key string) bool { _, present := field.values[key]; return present }
	fieldType := get("type")
	convertible := cmdtConvertibleTypes[fieldType]
	// A text formula field has no length and becomes a long text area.
	formulaText := fieldType == "Text" && !has("length")
	convertType := "Text"
	if fieldType == "Html" || fieldType == "MultiselectPicklist" || formulaText {
		convertType = "LongTextArea"
	}
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n")
	name := get("fullName")
	if !strings.HasSuffix(name, "__c") {
		name += "__c"
	}
	fmt.Fprintf(&b, "\t<fullName>%s</fullName>\n", name)
	if value := get("description"); value != "" {
		fmt.Fprintf(&b, "\t<description>%s</description>\n", value)
	}
	if value := get("externalId"); value != "" && value != "false" {
		fmt.Fprintf(&b, "\t<externalId>%s</externalId>\n", value)
	}
	manageability := get("fieldManageability")
	if manageability == "" {
		manageability = "DeveloperControlled"
	}
	fmt.Fprintf(&b, "\t<fieldManageability>%s</fieldManageability>\n", manageability)
	if value := get("inlineHelpText"); value != "" {
		fmt.Fprintf(&b, "\t<inlineHelpText>%s</inlineHelpText>\n", value)
	}
	fmt.Fprintf(&b, "\t<label>%s</label>\n", get("label"))
	writtenType := fieldType
	switch {
	case convertible && formulaText:
		writtenType = "LongTextArea"
	case convertible:
	case defaultToText:
		writtenType = convertType
	default:
		return "", "", fmt.Errorf("'%s' is not a valid field type.", fieldType)
	}
	fmt.Fprintf(&b, "\t<type>%s</type>\n", writtenType)
	defaultValue := get("defaultValue")
	switch {
	case fieldType == "Currency":
		if defaultValue != "" {
			fmt.Fprintf(&b, "\t<defaultValue>'%s'</defaultValue>\n", defaultValue)
		}
	case fieldType == "Checkbox" && !has("defaultValue"):
		b.WriteString("\t<defaultValue>false</defaultValue>\n")
	case defaultValue != "":
		fmt.Fprintf(&b, "\t<defaultValue>%s</defaultValue>\n", defaultValue)
	}
	if value := get("unique"); value == "true" || value == "false" {
		fmt.Fprintf(&b, "\t<unique>%s</unique>\n", value)
	}
	switch {
	case fieldType == "MultiselectPicklist" || formulaText:
		b.WriteString("\t<length>32768</length>\n")
	case get("length") != "" && get("length") != "0":
		fmt.Fprintf(&b, "\t<length>%s</length>\n", get("length"))
	case !convertible && convertType == "Text":
		b.WriteString("\t<length>100</length>\n")
	}
	switch {
	case formulaText:
		b.WriteString("\t<visibleLines>3</visibleLines>\n")
	case get("visibleLines") != "" && get("visibleLines") != "0":
		fmt.Fprintf(&b, "\t<visibleLines>%s</visibleLines>\n", get("visibleLines"))
	}
	if convertible {
		if valueSet, ok := field.values["valueSet"].(*orderedMap); ok {
			b.WriteString("\t<valueSet>\n")
			restricted := text(valueSet.values["restricted"])
			if restricted == "" {
				restricted = "false"
			}
			fmt.Fprintf(&b, "\t\t<restricted>%s</restricted>\n\t\t<valueSetDefinition>\n", restricted)
			definition, _ := valueSet.values["valueSetDefinition"].(*orderedMap)
			sorted := "false"
			var values []interface{}
			if definition != nil {
				if value := text(definition.values["sorted"]); value != "" {
					sorted = value
				}
				values = asList(definition.values["value"])
			}
			fmt.Fprintf(&b, "\t\t\t<sorted>%s</sorted>\n", sorted)
			for _, item := range values {
				value, ok := item.(*orderedMap)
				if !ok || text(value.values["fullName"]) == "" || text(value.values["label"]) == "" {
					continue
				}
				isDefault := text(value.values["default"])
				if isDefault == "" {
					isDefault = "false"
				}
				fmt.Fprintf(&b, "\t\t\t<value>\n\t\t\t\t<fullName>%s</fullName>\n\t\t\t\t<default>%s</default>\n\t\t\t\t<label>%s</label>\n\t\t\t</value>\n", text(value.values["fullName"]), isDefault, text(value.values["label"]))
			}
			b.WriteString("\t\t</valueSetDefinition>\n\t</valueSet>\n")
		}
		if value := get("precision"); value != "" && value != "0" {
			fmt.Fprintf(&b, "\t<precision>%s</precision>\n", value)
		}
		if has("scale") {
			fmt.Fprintf(&b, "\t<scale>%s</scale>\n", get("scale"))
		}
	}
	b.WriteString("</CustomField>\n")
	return b.String(), writtenType, nil
}

var (
	sobjectAPIName     = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*(_[a-zA-Z0-9]+)*(__[cC])?$`)
	cmdtRecordNameRule = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*(_[a-zA-Z0-9]+)*$`)
	spaces             = regexp.MustCompile(` +`)
)

// jsValue formats a value of a queried record the way JavaScript prints
// it: a whole number has no fraction.
func jsValue(value interface{}) string {
	if number, ok := value.(json.Number); ok {
		return strings.TrimSuffix(string(number), ".0")
	}
	return displayValue(value)
}

func newCmdtFromOrgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "fromorg",
		Short:       "Generate a custom metadata type and all its records from a Salesforce object.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Generate"},
		RunE: cmdtRun(func(cmd *cobra.Command, args []string) error {
			devName, _ := cmd.Flags().GetString("dev-name")
			sobject, _ := cmd.Flags().GetString("sobject")
			var missing []string
			if devName == "" {
				missing = append(missing, "dev-name")
			}
			if sobject == "" {
				missing = append(missing, "sobject")
			}
			if len(missing) > 0 {
				return missingFlagsError(missing...)
			}
			devName, err := cmdtTypeName(devName)
			if err != nil {
				if e, ok := err.(*Error); ok {
					e.Message = strings.Replace(e.Message, "Parsing --type-name ", "Parsing --dev-name ", 1)
				}
				return err
			}
			if cleaned := strings.NewReplacer("__c", "", "__C", "").Replace(sobject); len(cleaned) > 40 || !sobjectAPIName.MatchString(cleaned) {
				return flagParseError("SfError", "sobject", fmt.Sprintf("Not a valid custom setting/custom object name: %s", sobject))
			}
			visibility, _ := cmd.Flags().GetString("visibility")
			if visibility != "PackageProtected" && visibility != "Protected" && visibility != "Public" {
				return &Error{Message: fmt.Sprintf("Expected --visibility=%s to be one of: PackageProtected, Protected, Public\nSee more help with --help", visibility), ExitCode: 1}
			}
			ignoreUnsupported, _ := cmd.Flags().GetBool("ignore-unsupported")
			typeDir, _ := cmd.Flags().GetString("type-output-directory")
			recordDir, _ := cmd.Flags().GetString("records-output-dir")
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			// The object as the Metadata API describes it.
			body, err := org.Force.Metadata.ReadMetadataXML("CustomObject", sobject)
			if err != nil {
				return apiError(err)
			}
			var describe *orderedMap
			if results, parseErr := soapResults(body); parseErr == nil && len(results) > 0 {
				for _, record := range asList(results[0].values["records"]) {
					describe, _ = record.(*orderedMap)
					break
				}
			}
			var objectFields []*orderedMap
			if describe != nil {
				for _, item := range asList(describe.values["fields"]) {
					if field, ok := item.(*orderedMap); ok {
						objectFields = append(objectFields, field)
					}
				}
			}
			if len(objectFields) == 0 {
				return newError("sobjectnameNoResultError", "No sObject with name %s found in the org.", sobject)
			}
			if settingsType := text(describe.values["customSettingsType"]); settingsType != "" {
				if settingsType != "List" || text(describe.values["visibility"]) != "Public" {
					return newError("customSettingTypeError", "Cannot generate custom metadata for the custom setting %s. Check type and visibility.", sobject)
				}
			}
			objectName := text(describe.values["fullName"])
			if objectName == "" {
				objectName = sobject
			}
			label, _ := cmd.Flags().GetString("label")
			if label == "" {
				label = devName
			}
			pluralLabel, _ := cmd.Flags().GetString("plural-label")
			if pluralLabel == "" {
				pluralLabel = label
			}
			typeName := devName + "__mdt"
			errOut := cmd.ErrOrStderr()
			progress := func(format string, values ...interface{}) {
				if !jsonRequested(cmd) {
					fmt.Fprintf(errOut, format, values...)
				}
			}
			progress("creating the CMDT object... creating the CMDT fields\n")

			count := 0
			generate := func() error {
				if err := os.MkdirAll(filepath.Join(typeDir, typeName), 0o755); err != nil {
					return err
				}
				objectXML := fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n\t<label>%s</label>\n\t<pluralLabel>%s</pluralLabel>\n\t<visibility>%s</visibility>\n</CustomObject>\n", label, pluralLabel, visibility)
				if err := os.WriteFile(filepath.Join(typeDir, typeName, typeName+".object-meta.xml"), []byte(objectXML), 0o644); err != nil {
					return err
				}
				// The fields that have a name, a label and a type.  A
				// location field becomes a latitude and a longitude text
				// field.
				var describable, fields []*orderedMap
				for _, field := range objectFields {
					if text(field.values["fullName"]) == "" || text(field.values["label"]) == "" || text(field.values["type"]) == "" {
						continue
					}
					describable = append(describable, field)
					fieldType := text(field.values["type"])
					if ignoreUnsupported && !cmdtConvertibleTypes[fieldType] {
						continue
					}
					if fieldType != "Location" {
						fields = append(fields, field)
						continue
					}
					for _, prefix := range []string{"Lat_", "Long_"} {
						part := newOrderedMap()
						for _, key := range []string{"required", "trackHistory", "trackTrending"} {
							if value, present := field.values[key]; present {
								part.Set(key, value)
							}
						}
						part.Set("type", "Text")
						part.Set("length", "40")
						part.Set("fullName", prefix+text(field.values["fullName"]))
						part.Set("label", prefix+text(field.values["label"]))
						fields = append(fields, part)
					}
				}
				// written describes the fields that were written, for the
				// types of the record values.  The files are not read
				// back: sf writes the values without XML escaping, so a
				// file can be one that an XML parser refuses.
				var written []*orderedMap
				for _, field := range fields {
					fieldXML, writtenType, err := cmdtFieldFileFromOrg(field, !ignoreUnsupported)
					if err != nil {
						return err
					}
					name := text(field.values["fullName"])
					if !strings.HasSuffix(name, "__c") {
						name += "__c"
					}
					description := orderedFields("fullName", name, "type", writtenType)
					if scale := text(field.values["scale"]); scale != "" && cmdtConvertibleTypes[text(field.values["type"])] {
						description.Set("scale", json.Number(scale))
					}
					written = append(written, description)
					directory := filepath.Join(typeDir, typeName, "fields")
					if err := os.MkdirAll(directory, 0o755); err != nil {
						return err
					}
					if err := os.WriteFile(filepath.Join(directory, name+".field-meta.xml"), []byte(fieldXML), 0o644); err != nil {
						return err
					}
				}
				progress("creating the CMDT object... creating the CMDT records")
				if err := os.MkdirAll(recordDir, 0o755); err != nil {
					return err
				}
				// sf lists the field files here and fails when the object
				// had no field to write.
				fieldDirectory := filepath.Join(typeDir, typeName, "fields")
				if _, err := os.Stat(fieldDirectory); err != nil {
					return fileSystemError("scandir", fieldDirectory, err)
				}
				names := make([]string, 0, len(describable))
				types := map[string]string{}
				for _, field := range describable {
					names = append(names, text(field.values["fullName"]))
					types[text(field.values["fullName"])] = text(field.values["type"])
				}
				result, err := runQuery(org, "", "query", fmt.Sprintf("SELECT Name, %s FROM %s", strings.Join(names, ","), objectName))
				if err != nil {
					return apiError(err)
				}
				records, _ := result.values["records"].([]interface{})
				for _, item := range records {
					record, ok := item.(*orderedMap)
					if !ok {
						continue
					}
					values := newOrderedMap()
					for _, key := range record.keys {
						if key == "attributes" || key == "Name" {
							continue
						}
						value := record.values[key]
						if types[key] == "Location" {
							latitude, longitude := "", ""
							if location, isLocation := value.(*orderedMap); isLocation {
								latitude, longitude = jsValue(location.values["latitude"]), jsValue(location.values["longitude"])
							}
							values.Set("Lat_"+key, latitude)
							values.Set("Long_"+key, longitude)
							continue
						}
						if ignoreUnsupported && !cmdtConvertibleTypes[text(cmdtFieldType(written, key))] {
							continue
						}
						if value == nil {
							values.Set(key, nil)
						} else {
							values.Set(key, jsValue(value))
						}
					}
					if ignoreUnsupported {
						// Only the fields that were written are kept.
						kept := newOrderedMap()
						for _, key := range values.keys {
							if cmdtField(written, key) != nil {
								kept.Set(key, values.values[key])
							}
						}
						values = kept
					}
					recordLabel := text(record.values["Name"])
					recordName := recordLabel
					if len(recordName) > 40 || !cmdtRecordNameRule.MatchString(recordName) {
						recordName = spaces.ReplaceAllString(recordLabel, "_")
					}
					if err := writeCmdtRecordFromOrg(recordDir, written, devName, recordName, recordLabel, visibility != "Public", values); err != nil {
						return err
					}
				}
				count = len(records)
				return nil
			}
			if err := generate(); err != nil {
				// Remove what was written for this type.
				os.RemoveAll(typeDir + typeName)
				if entries, readErr := os.ReadDir(recordDir); readErr == nil {
					for _, entry := range entries {
						if strings.HasPrefix(entry.Name(), devName) {
							os.Remove(filepath.Join(recordDir, entry.Name()))
						}
					}
				}
				progress("creating the CMDT object... generate command failed to run\n")
				message := err.Error()
				if e, ok := err.(*Error); ok {
					message = e.Message
				}
				return newError("generateError", "Failed to generate custom metadata. Reason: %s.", message)
			}
			progress("creating the CMDT object... custom metadata type and records creation in completed\n")
			result := orderedFields("outputDir", typeDir, "recordsOutputDir", recordDir)
			return emit(cmd, result, func(w io.Writer) {
				fmt.Fprintf(w, "Congrats! Created a %s custom metadata type with %d records!\n", devName, count)
			})
		}),
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("dev-name", "n", "", "Name of the custom metadata type.")
	cmd.Flags().StringP("label", "l", "", "Label for the custom metadata type.")
	cmd.Flags().StringP("plural-label", "p", "", "Plural version of the label value; if blank, uses label.")
	cmd.Flags().StringP("visibility", "v", "Public", "Who can see the custom metadata type.")
	cmd.Flags().StringP("sobject", "s", "", "API name of the source Salesforce object used to generate the custom metadata type.")
	cmd.Flags().BoolP("ignore-unsupported", "i", false, "Ignore unsupported field types.")
	cmd.Flags().StringP("type-output-directory", "d", filepath.Join("force-app", "main", "default", "objects"), "Directory to store newly-created custom metadata type files.")
	cmd.Flags().StringP("records-output-dir", "r", filepath.Join("force-app", "main", "default", "customMetadata"), "Directory to store newly-created custom metadata record files.")
	addTargetOrgFlag(cmd)
	return cmd
}

// cmdtFieldType returns the type in the field file of a field, or nil.
func cmdtFieldType(fields []*orderedMap, name string) interface{} {
	if field := cmdtField(fields, name); field != nil {
		value, _ := field.Get("type")
		return value
	}
	return nil
}

// writeCmdtRecordFromOrg writes a record file with the values of a record
// of an org.  A value that is nil or empty is written as xsi:nil.
func writeCmdtRecordFromOrg(outputDir string, fields []*orderedMap, typeName, name, label string, protected bool, values *orderedMap) error {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomMetadata xmlns=\"http://soap.sforce.com/2006/04/metadata\" xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" xmlns:xsd=\"http://www.w3.org/2001/XMLSchema\">\n")
	fmt.Fprintf(&b, "    <label>%s</label>\n    <protected>%t</protected>", label, protected)
	escape := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	for _, key := range values.keys {
		value := "<value xsi:nil=\"true\"/>"
		if content, isText := values.values[key].(string); isText && content != "" {
			value = fmt.Sprintf("<value xsi:type=\"xsd:%s\">%s</value>", cmdtValueType(fields, key), escape.Replace(content))
		}
		fmt.Fprintf(&b, "\n    <values>\n        <field>%s</field>\n        %s\n    </values>", key, value)
	}
	b.WriteString("\n</CustomMetadata>")
	return os.WriteFile(filepath.Join(outputDir, typeName+"."+name+".md-meta.xml"), []byte(b.String()), 0o644)
}
