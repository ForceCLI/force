package sf

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newCmdtCmd() *cobra.Command {
	return newGroup("cmdt", "Generate custom metadata types and their records.",
		newGroup("generate", "Generate custom metadata types, fields and records.",
			newCmdtObjectCmd(),
			newCmdtFieldCmd(),
			newCmdtRecordCmd(),
			newCmdtRecordsCmd(),
			newCmdtFromOrgCmd(),
		),
	)
}

// cmdtRun wraps the body of a cmdt command.  sf installs the plugin for
// these commands on first use, and that plugin prints one blank line after
// an error where the commands bundled with sf print two.
func cmdtRun(run func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := requireProject(cmd); err != nil {
			return err
		}
		err := run(cmd, args)
		if err == nil {
			return nil
		}
		if _, ok := err.(exitStatus); ok {
			return err
		}
		e := asError(err)
		e.Compact = true
		return e
	}
}

// requireProject returns an error when the working directory is not in a
// Salesforce project.  It has printed the error, in the form sf prints it
// for these commands: no "Error (name):" line, and no JSON envelope when
// --json is given.
func requireProject(cmd *cobra.Command) error {
	if _, err := loadProject(); err != nil {
		fmt.Fprint(cmd.ErrOrStderr(), "    RequiresProjectError: This command is required to run from within a \n    Salesforce project directory.\n    Code: RequiresProjectError\n")
		return exitStatus(1)
	}
	return nil
}

// flagParseError is the error sf reports when the value of a flag fails
// the check the command has for it.
func flagParseError(name, flag, message string) error {
	return &Error{Name: name, Message: fmt.Sprintf("Parsing --%s \n\t%s\nSee more help with --help", flag, message), ExitCode: 2}
}

// fileSystemError reports a failed file operation the way Node.js names it.
func fileSystemError(operation, path string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return newError("ENOENT", "ENOENT: no such file or directory, %s '%s'", operation, path)
	}
	return err
}

var (
	metadataTypeSuffix = regexp.MustCompile(`(?i)__mdt$`)
	metadataTypeName   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*(_[a-zA-Z0-9]+)*(__[mdtT])?$`)
	metadataRecordName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*(_[a-zA-Z0-9]+)*$`)
	customFieldName    = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*(_[a-zA-Z0-9]+)*(__[cC])?$`)
)

// cmdtTypeName checks the value of --type-name and returns it without the
// __mdt suffix.
func cmdtTypeName(value string) (string, error) {
	trimmed := metadataTypeSuffix.ReplaceAllString(value, "")
	if len(trimmed) > 40 || !metadataTypeName.MatchString(trimmed) {
		return "", flagParseError("SfError", "type-name", fmt.Sprintf("'%s' is not a valid API name for a custom metadata type. Metadata names can contain only underscores and alphanumeric characters, must begin with a letter, cannot end with an underscore, and cannot contain two consecutive underscore characters.", value))
	}
	return trimmed, nil
}

// cmdtLabel checks a label flag, when it is set.  noun is "label" or
// "plural label".
func cmdtLabel(cmd *cobra.Command, flag, noun string) (string, bool, error) {
	if !cmd.Flags().Changed(flag) {
		return "", false, nil
	}
	value, _ := cmd.Flags().GetString(flag)
	if len(value) > 40 {
		return "", false, flagParseError("SfError", flag, fmt.Sprintf("'%s' is too long to be a %s. The maximum length of the %s is 40 characters.", value, noun, noun))
	}
	return value, true, nil
}

func withSuffix(name, suffix string) string {
	if strings.HasSuffix(name, suffix) {
		return name
	}
	return name + suffix
}

// writeCmdtFile writes a type or field file and prints the two lines sf
// prints for it.
func writeCmdtFile(cmd *cobra.Command, directory, fileName, content string) error {
	path := filepath.Join(directory, fileName)
	verb := "created"
	if _, err := os.Stat(path); err == nil {
		verb = "updated"
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	if !jsonRequested(cmd) {
		fmt.Fprintf(cmd.OutOrStdout(), "target dir = %s\n%s %s\n", directory, verb, fileName)
	}
	return nil
}

func newCmdtObjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "object",
		Short:       "Generate a new custom metadata type in the current project.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Create"},
		RunE: cmdtRun(func(cmd *cobra.Command, args []string) error {
			value, _ := cmd.Flags().GetString("type-name")
			if value == "" {
				return missingFlagError("type-name")
			}
			typeName, err := cmdtTypeName(value)
			if err != nil {
				return err
			}
			label, set, err := cmdtLabel(cmd, "label", "label")
			if err != nil {
				return err
			}
			if !set {
				label = strings.Replace(typeName, "__mdt", "", 1)
			}
			pluralLabel, set, err := cmdtLabel(cmd, "plural-label", "plural label")
			if err != nil {
				return err
			}
			if !set {
				pluralLabel = label
			}
			visibility, _ := cmd.Flags().GetString("visibility")
			switch visibility {
			case "PackageProtected", "Protected", "Public":
			default:
				return &Error{Message: fmt.Sprintf("Expected --visibility=%s to be one of: PackageProtected, Protected, Public\nSee more help with --help", visibility), ExitCode: 1}
			}
			apiName := typeName
			if strings.HasSuffix(apiName, "__c") {
				apiName = strings.Replace(apiName, "__c", "__mdt", 1)
			}
			apiName = withSuffix(apiName, "__mdt")
			outputDir, _ := cmd.Flags().GetString("output-directory")
			// sf writes the labels into the file without escaping them.
			content := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n" +
				"\t<label>" + label + "</label>\n\t<pluralLabel>" + pluralLabel + "</pluralLabel>\n\t<visibility>" + visibility + "</visibility>\n</CustomObject>\n"
			if err := writeCmdtFile(cmd, filepath.Join(strings.TrimRight(outputDir, "/"), apiName), apiName+".object-meta.xml", content); err != nil {
				return err
			}
			result := newOrderedMap()
			result.Set("typename", typeName)
			result.Set("label", label)
			result.Set("pluralLabel", pluralLabel)
			result.Set("visibility", visibility)
			return emit(cmd, result, func(io.Writer) {})
		}),
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("type-name", "n", "", "Unique object name for the custom metadata type.")
	cmd.Flags().StringP("label", "l", "", "Label for the custom metadata type.")
	cmd.Flags().StringP("plural-label", "p", "", "Plural version of the label name; if blank, uses label.")
	cmd.Flags().StringP("visibility", "v", "Public", "Who can see the custom metadata type.")
	cmd.Flags().StringP("output-directory", "d", "", "Directory to store the newly-created custom metadata type files")
	return cmd
}

var cmdtFieldTypes = []string{"Checkbox", "Date", "DateTime", "Email", "Number", "Percent", "Phone", "Picklist", "Text", "TextArea", "LongTextArea", "Url"}

// cmdtFieldXML returns the field file sf writes for a field of a type.
func cmdtFieldXML(name, fieldType, label string, picklistValues []string, decimalPlaces int) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n")
	fmt.Fprintf(&b, "\t<fullName>%s</fullName>\n", withSuffix(name, "__c"))
	b.WriteString("\t<fieldManageability>DeveloperControlled</fieldManageability>\n")
	fmt.Fprintf(&b, "\t<label>%s</label>\n", label)
	fmt.Fprintf(&b, "\t<type>%s</type>\n", fieldType)
	switch fieldType {
	case "Checkbox":
		b.WriteString("\t<defaultValue>false</defaultValue>\n")
	case "Email", "Number":
		b.WriteString("\t<unique>false</unique>\n")
	case "Text":
		b.WriteString("\t<unique>false</unique>\n\t<length>100</length>\n")
	case "LongTextArea":
		b.WriteString("\t<length>32768</length>\n\t<visibleLines>3</visibleLines>\n")
	case "Picklist":
		b.WriteString("\t<valueSet>\n\t\t<restricted>true</restricted>\n\t\t<valueSetDefinition>\n\t\t\t<sorted>false</sorted>\n")
		for _, value := range picklistValues {
			fmt.Fprintf(&b, "\t\t\t<value>\n\t\t\t\t<fullName>%s</fullName>\n\t\t\t\t<default>false</default>\n\t\t\t\t<label>%s</label>\n\t\t\t</value>\n", value, value)
		}
		b.WriteString("\t\t</valueSetDefinition>\n\t</valueSet>\n")
	}
	if fieldType == "Number" || fieldType == "Percent" {
		// sf omits the precision when 18 decimal places leave none.
		if precision := 18 - decimalPlaces; precision != 0 {
			fmt.Fprintf(&b, "\t<precision>%d</precision>\n", precision)
		}
		fmt.Fprintf(&b, "\t<scale>%d</scale>\n", decimalPlaces)
	}
	b.WriteString("</CustomField>\n")
	return b.String()
}

func newCmdtFieldCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "field",
		Short:       "Generate a field for a custom metadata type based on the provided field type.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Create"},
		RunE: cmdtRun(func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			fieldType, _ := cmd.Flags().GetString("type")
			if name == "" {
				return missingFlagError("name")
			}
			if fieldType == "" {
				return missingFlagError("type")
			}
			cleaned := strings.Replace(strings.Replace(name, "__c", "", 1), "__C", "", 1)
			if len(cleaned) > 40 || !customFieldName.MatchString(cleaned) {
				return flagParseError("SfError", "name", fmt.Sprintf("'%s' is an invalid field. Custom fields must contain only alphanumeric characters, must begin with a letter, can't end with an underscore, and can't contain two consecutive underscore characters.", name))
			}
			known := false
			for _, option := range cmdtFieldTypes {
				if fieldType == option {
					known = true
				}
			}
			if !known {
				return &Error{Message: fmt.Sprintf("Expected --type=%s to be one of: %s\nSee more help with --help", fieldType, strings.Join(cmdtFieldTypes, ", ")), ExitCode: 1}
			}
			decimalPlaces, _ := cmd.Flags().GetInt("decimal-places")
			if decimalPlaces < 0 {
				return flagParseError("", "decimal-places", fmt.Sprintf("Expected an integer greater than or equal to 0 but received: %d", decimalPlaces))
			}
			picklistValues := splitList(cmd, "picklist-values")
			if fieldType == "Picklist" && len(picklistValues) == 0 {
				return newError("SfError", "You must specify at least one picklist value when the field type is picklist.")
			}
			label := name
			if cmd.Flags().Changed("label") {
				label, _ = cmd.Flags().GetString("label")
			}
			outputDir, _ := cmd.Flags().GetString("output-directory")
			content := cmdtFieldXML(name, fieldType, label, picklistValues, decimalPlaces)
			if err := writeCmdtFile(cmd, filepath.Join(strings.TrimRight(outputDir, "/"), "fields"), withSuffix(name, "__c")+".field-meta.xml", content); err != nil {
				return err
			}
			result := newOrderedMap()
			result.Set("fieldName", name)
			result.Set("label", label)
			result.Set("fieldtype", fieldType)
			return emit(cmd, result, func(io.Writer) {})
		}),
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("name", "n", "", "Unique name for the field.")
	cmd.Flags().StringP("type", "f", "", "Type of the field.")
	cmd.Flags().StringArrayP("picklist-values", "p", nil, "Picklist values; required for picklist fields.")
	cmd.Flags().IntP("decimal-places", "s", 0, "Number of decimal places to use for number or percent fields.")
	cmd.Flags().StringP("label", "l", "", "Label for the field.")
	cmd.Flags().StringP("output-directory", "d", "", "Directory to store newly-created field definition files.")
	return cmd
}

// A number as the XML parser of sf reads it from the text of an element.
var xmlNumber = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)

// xmlValue converts the content of an XML element to the value sf has for
// it after parsing: an object for child elements, a list for a repeated
// child, and a boolean, a number or a string for text.
func xmlValue(decoder *xml.Decoder) (interface{}, error) {
	var text strings.Builder
	var children *orderedMap
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.CharData:
			text.Write(t)
		case xml.StartElement:
			child, err := xmlValue(decoder)
			if err != nil {
				return nil, err
			}
			if children == nil {
				children = newOrderedMap()
			}
			if existing, ok := children.Get(t.Name.Local); ok {
				if list, isList := existing.([]interface{}); isList {
					children.Set(t.Name.Local, append(list, child))
				} else {
					children.Set(t.Name.Local, []interface{}{existing, child})
				}
			} else {
				children.Set(t.Name.Local, child)
			}
		case xml.EndElement:
			if children != nil {
				return children, nil
			}
			value := strings.TrimSpace(text.String())
			switch {
			case value == "true":
				return true, nil
			case value == "false":
				return false, nil
			case xmlNumber.MatchString(value):
				if number, err := strconv.ParseFloat(value, 64); err == nil {
					return json.Number(strconv.FormatFloat(number, 'f', -1, 64)), nil
				}
			}
			return value, nil
		}
	}
}

// readCmdtFields reads the field files of a custom metadata type, in the
// order of their file names.
func readCmdtFields(inputDir, typeName string) ([]*orderedMap, error) {
	directory := filepath.Join(inputDir, withSuffix(typeName, "__mdt"), "fields")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fileSystemError("scandir", directory, err)
	}
	fields := []*orderedMap{}
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, err
		}
		decoder := xml.NewDecoder(bytes.NewReader(content))
		for {
			token, err := decoder.Token()
			if err != nil {
				return nil, fmt.Errorf("%s: no CustomField element: %v", filepath.Join(directory, entry.Name()), err)
			}
			if start, ok := token.(xml.StartElement); ok && start.Name.Local == "CustomField" {
				break
			}
		}
		value, err := xmlValue(decoder)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", filepath.Join(directory, entry.Name()), err)
		}
		if field, ok := value.(*orderedMap); ok {
			fields = append(fields, field)
		}
	}
	return fields, nil
}

func cmdtField(fields []*orderedMap, name string) *orderedMap {
	for _, field := range fields {
		if fullName, _ := field.Get("fullName"); fullName == name {
			return field
		}
	}
	return nil
}

// cmdtValueType returns the XML schema type of a value of a field in a
// record.  A field the type does not have is a string.
func cmdtValueType(fields []*orderedMap, name string) string {
	field := cmdtField(fields, name)
	if field == nil {
		return "string"
	}
	fieldType, _ := field.Get("type")
	switch fieldType {
	case "Number", "Percent":
		if scale, _ := field.Get("scale"); scale == json.Number("0") {
			return "int"
		}
		return "double"
	case "Checkbox":
		return "boolean"
	case "Date":
		return "date"
	case "DateTime":
		return "dateTime"
	}
	return "string"
}

// cmdtRecord is a custom metadata record to write.  values holds the field
// values in the order they are written.
type cmdtRecord struct {
	typeName, name, label string
	protected             bool
	values                *orderedMap
}

func writeCmdtRecord(outputDir string, fields []*orderedMap, record cmdtRecord) error {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomMetadata xmlns=\"http://soap.sforce.com/2006/04/metadata\" xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" xmlns:xsd=\"http://www.w3.org/2001/XMLSchema\">\n")
	fmt.Fprintf(&b, "    <label>%s</label>\n    <protected>%t</protected>", record.label, record.protected)
	escape := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	for _, name := range record.values.keys {
		value, _ := record.values.Get(name)
		fmt.Fprintf(&b, "\n    <values>\n        <field>%s</field>\n        <value xsi:type=\"xsd:%s\">%s</value>\n    </values>", name, cmdtValueType(fields, name), escape.Replace(value.(string)))
	}
	b.WriteString("\n</CustomMetadata>")
	return os.WriteFile(filepath.Join(outputDir, record.typeName+"."+record.name+".md-meta.xml"), []byte(b.String()), 0o644)
}

// cmdtDirectories returns the values of --input-directory and
// --output-directory.  The input directory must exist.
func cmdtDirectories(cmd *cobra.Command) (string, string, error) {
	inputDir, _ := cmd.Flags().GetString("input-directory")
	outputDir, _ := cmd.Flags().GetString("output-directory")
	if info, err := os.Stat(inputDir); cmd.Flags().Changed("input-directory") && (err != nil || !info.IsDir()) {
		return "", "", flagParseError("", "input-directory", "No directory found at "+inputDir)
	}
	return inputDir, outputDir, nil
}

func addCmdtDirectoryFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("input-directory", "i", filepath.Join("force-app", "main", "default", "objects"), "Directory from which to get the custom metadata type definition from.")
	cmd.Flags().StringP("output-directory", "d", filepath.Join("force-app", "main", "default", "customMetadata"), "Directory to store newly-created custom metadata record files.")
}

// nameValueArguments reads arguments of the form name=value, in order.  A
// name with an empty value is left out, as sf leaves it out.
func nameValueArguments(args []string) (*orderedMap, error) {
	values := newOrderedMap()
	if len(args) == 2 && !strings.Contains(args[0], "=") {
		values.Set(args[0], args[1])
		return values, nil
	}
	for _, arg := range args {
		parts := strings.Split(arg, "=")
		if len(parts) != 2 {
			return nil, newError("InvalidArgumentFormatError", "Error in the following argument\n%s\nSet varargs with this format: key=value or key=\"value with spaces\"", arg)
		}
		if _, exists := values.Get(parts[0]); exists {
			return nil, newError("DuplicateArgumentError", "Found duplicate argument %s.", parts[0])
		}
		if parts[1] != "" {
			values.Set(parts[0], parts[1])
		}
	}
	return values, nil
}

func newCmdtRecordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "record",
		Short:       "Generate a new record for a given custom metadata type in the current project.",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{commandNameAnnotation: "Create"},
		RunE: cmdtRun(func(cmd *cobra.Command, args []string) error {
			value, _ := cmd.Flags().GetString("type-name")
			recordName, _ := cmd.Flags().GetString("record-name")
			if value == "" {
				return missingFlagError("type-name")
			}
			if recordName == "" {
				return missingFlagError("record-name")
			}
			typeName, err := cmdtTypeName(value)
			if err != nil {
				return err
			}
			if len(recordName) > 40 || !metadataRecordName.MatchString(recordName) {
				return flagParseError("SfError", "record-name", fmt.Sprintf("'%s' is not a valid record name for a custom metadata record. Record names can only contain alphanumeric characters, must begin with a letter, cannot end with an underscore, and cannot contain two consecutive underscore characters.", recordName))
			}
			label, set, err := cmdtLabel(cmd, "label", "label")
			if err != nil {
				return err
			}
			if !set {
				label = recordName
			}
			protectedValue, _ := cmd.Flags().GetString("protected")
			if protectedValue != "true" && protectedValue != "false" {
				return &Error{Message: fmt.Sprintf("Expected --protected=%s to be one of: true, false\nSee more help with --help", protectedValue), ExitCode: 1}
			}
			protected := protectedValue == "true"
			inputDir, outputDir, err := cmdtDirectories(cmd)
			if err != nil {
				return err
			}
			values, err := nameValueArguments(args)
			if err != nil {
				return err
			}
			fields, err := readCmdtFields(inputDir, typeName)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return err
			}
			if err := writeCmdtRecord(outputDir, fields, cmdtRecord{typeName, recordName, label, protected, values}); err != nil {
				return err
			}
			result := newOrderedMap()
			result.Set("typename", typeName)
			result.Set("recordname", recordName)
			result.Set("label", label)
			result.Set("inputdir", inputDir)
			result.Set("outputdir", outputDir)
			result.Set("protectedFlag", protected)
			result.Set("varargs", values)
			result.Set("fileData", fields)
			return emit(cmd, result, func(w io.Writer) {
				fmt.Fprintf(w, "Created custom metadata record of the type '%s' with record name '%s', label '%s', and protected '%t' at '%s'.\n", typeName, recordName, label, protected, outputDir)
			})
		}),
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("type-name", "t", "", "API name of the custom metadata type to create a record for; must end in \"__mdt\".")
	cmd.Flags().StringP("record-name", "n", "", "Name of the new record.")
	cmd.Flags().StringP("label", "l", "", "Label for the new record.")
	cmd.Flags().StringP("protected", "p", "false", "Protect the record when it's in a managed package.")
	addCmdtDirectoryFlags(cmd)
	return cmd
}

var (
	notAlphanumeric     = regexp.MustCompile(`[^a-zA-Z0-9]`)
	repeatedUnderscores = regexp.MustCompile(`_{2,}`)
)

// cmdtRecordNameFor derives the name of a record from the value of the
// name column of a CSV row.
func cmdtRecordNameFor(value string) string {
	name := notAlphanumeric.ReplaceAllString(value, "_")
	if name != "" && name[0] >= '0' && name[0] <= '9' {
		name = "X" + name
	}
	name = repeatedUnderscores.ReplaceAllString(name, "_")
	return strings.TrimSuffix(name, "_")
}

func newCmdtRecordsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "records",
		Short:       "Generate new custom metadata type records from a CSV file.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Insert"},
		RunE: cmdtRun(func(cmd *cobra.Command, args []string) error {
			csvPath, _ := cmd.Flags().GetString("csv")
			typeName, _ := cmd.Flags().GetString("type-name")
			if csvPath == "" {
				return missingFlagError("csv")
			}
			if typeName == "" {
				return missingFlagError("type-name")
			}
			if strings.HasSuffix(typeName, "__mdt") {
				typeName = strings.Replace(typeName, "__mdt", "", 1)
			}
			nameColumn, _ := cmd.Flags().GetString("name-column")
			inputDir, outputDir, err := cmdtDirectories(cmd)
			if err != nil {
				return err
			}
			fields, err := readCmdtFields(inputDir, typeName)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return err
			}
			content, err := os.ReadFile(csvPath)
			if err != nil {
				return fileSystemError("open", csvPath, err)
			}
			rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(content, []byte("\xef\xbb\xbf")))).ReadAll()
			if err != nil {
				return newError("SfError", "%v", err)
			}
			if len(rows) == 0 {
				rows = [][]string{{}}
			}
			header := rows[0]
			nameIndex := -1
			for i, column := range header {
				if column == nameColumn {
					nameIndex = i
				} else if cmdtField(fields, column) == nil {
					return newError("SfError", "The column %s is not found on the custom metadata type %s.", column, typeName)
				}
			}
			if nameIndex < 0 && len(rows) > 1 {
				return newError("SfError", "The CSV file has no %s column, which gives the name of each record.", nameColumn)
			}
			var records []cmdtRecord
			seen := map[string]bool{}
			var duplicates []string
			for _, row := range rows[1:] {
				record := cmdtRecord{typeName: typeName, name: cmdtRecordNameFor(row[nameIndex]), label: row[nameIndex], values: newOrderedMap()}
				// Values are written in the order of the field files, and
				// an empty value is left out.
				for _, field := range fields {
					fullName, _ := field.Get("fullName")
					for i, column := range header {
						if column == fullName && i != nameIndex && row[i] != "" {
							record.values.Set(column, row[i])
						}
					}
				}
				if seen[record.name] {
					alreadyListed := false
					for _, duplicate := range duplicates {
						alreadyListed = alreadyListed || duplicate == record.name
					}
					if !alreadyListed {
						duplicates = append(duplicates, record.name)
					}
				}
				seen[record.name] = true
				records = append(records, record)
			}
			if len(duplicates) > 0 {
				return newError("SfError", "Your CSV has duplicate values:  %s.  CMDT require unique names in the name field.", strings.Join(duplicates, ", "))
			}
			result := []*orderedMap{}
			for _, record := range records {
				if err := writeCmdtRecord(outputDir, fields, record); err != nil {
					return err
				}
				item := newOrderedMap()
				item.Set("typename", typeName)
				item.Set("recordname", record.name)
				item.Set("label", record.label)
				item.Set("inputdir", inputDir)
				item.Set("outputdir", outputDir)
				item.Set("protected", false)
				item.Set("varargs", record.values)
				item.Set("fileData", fields)
				result = append(result, item)
			}
			return emit(cmd, result, func(w io.Writer) {
				fmt.Fprintf(w, "Created custom metadata type records from '%s' at '%s'.\n", csvPath, outputDir)
			})
		}),
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("csv", "f", "", "Pathname of the CSV file.")
	cmd.Flags().StringP("type-name", "t", "", "API name of the custom metadata type to create a record for.")
	cmd.Flags().StringP("name-column", "n", "Name", "Column used to determine the name of the record.")
	addCmdtDirectoryFlags(cmd)
	return cmd
}
