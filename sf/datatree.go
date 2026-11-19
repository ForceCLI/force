package sf

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// sobjectShape is the part of an sObject describe the tree export uses.
type sobjectShape struct {
	// references maps a reference field to the object types it can refer
	// to.
	references map[string][]string
	// childFields maps a child relationship name to the child's lookup
	// field.
	childFields map[string]string
}

func describeShape(org *targetOrg, sobject string) (*sobjectShape, error) {
	body, err := org.Force.GetREST("/sobjects/" + sobject + "/describe")
	if err != nil {
		return nil, apiError(err)
	}
	var describe struct {
		Fields []struct {
			Name        string   `json:"name"`
			Type        string   `json:"type"`
			ReferenceTo []string `json:"referenceTo"`
		} `json:"fields"`
		ChildRelationships []struct {
			Field            string `json:"field"`
			RelationshipName string `json:"relationshipName"`
		} `json:"childRelationships"`
	}
	if err := json.Unmarshal([]byte(body), &describe); err != nil {
		return nil, err
	}
	shape := &sobjectShape{references: map[string][]string{}, childFields: map[string]string{}}
	for _, field := range describe.Fields {
		if strings.EqualFold(field.Type, "reference") {
			shape.references[field.Name] = field.ReferenceTo
		}
	}
	for _, child := range describe.ChildRelationships {
		if child.RelationshipName != "" {
			shape.childFields[child.RelationshipName] = child.Field
		}
	}
	return shape, nil
}

// treeExport holds the state of a "data export tree" run.
type treeExport struct {
	org *targetOrg
	// refs maps a record Id to its reference id, such as AccountRef1.
	refs map[string]string
	// counts holds the number of references given out for each type.
	counts map[string]int
	// types lists the object types in the order they were first seen.
	types  []string
	shapes map[string]*sobjectShape
}

// recordID returns the Id of a queried record: the Id field, or the last
// part of the URL in its attributes when the query did not select the Id.
func recordID(record *orderedMap) string {
	if id := text(record.values["Id"]); id != "" {
		return id
	}
	if attributes, ok := record.values["attributes"].(*orderedMap); ok {
		url := text(attributes.values["url"])
		return url[strings.LastIndex(url, "/")+1:]
	}
	return ""
}

func recordType(record *orderedMap) string {
	if attributes, ok := record.values["attributes"].(*orderedMap); ok {
		return text(attributes.values["type"])
	}
	return ""
}

// nestedRecords returns the child records of a subquery value.
func nestedRecords(value interface{}) ([]*orderedMap, bool) {
	object, ok := value.(*orderedMap)
	if !ok {
		return nil, false
	}
	items, ok := object.values["records"].([]interface{})
	if !ok {
		return nil, false
	}
	records := make([]*orderedMap, 0, len(items))
	for _, item := range items {
		if record, ok := item.(*orderedMap); ok {
			records = append(records, record)
		}
	}
	return records, true
}

// register gives a record and its nested records reference ids and reads
// the describe of each type.
func (e *treeExport) register(record *orderedMap) error {
	objectType := recordType(record)
	if _, known := e.shapes[objectType]; !known {
		shape, err := describeShape(e.org, objectType)
		if err != nil {
			return err
		}
		e.shapes[objectType] = shape
		e.types = append(e.types, objectType)
	}
	id := recordID(record)
	if _, known := e.refs[id]; !known && id != "" {
		e.counts[objectType]++
		e.refs[id] = fmt.Sprintf("%sRef%d", objectType, e.counts[objectType])
	}
	for _, key := range record.keys {
		if children, ok := nestedRecords(record.values[key]); ok {
			for _, child := range children {
				if err := e.register(child); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// convert returns a record in the form sf writes: attributes with the type
// and reference id, lookup fields that point at an exported record replaced
// by "@<reference id>", and no Id, null values or empty child lists.  With
// keepChildren the nested records are converted too; otherwise they are
// left out.
func (e *treeExport) convert(record *orderedMap, keepChildren bool) *orderedMap {
	objectType := recordType(record)
	shape := e.shapes[objectType]
	converted := newOrderedMap()
	for _, key := range record.keys {
		value := record.values[key]
		switch {
		case key == "attributes":
			attributes := newOrderedMap()
			attributes.Set("type", objectType)
			attributes.Set("referenceId", e.refs[recordID(record)])
			converted.Set(key, attributes)
		case key == "Id" || value == nil:
		default:
			if children, ok := nestedRecords(value); ok {
				if !keepChildren || len(children) == 0 {
					continue
				}
				list := make([]interface{}, 0, len(children))
				for _, child := range children {
					list = append(list, e.convert(child, true))
				}
				nested := newOrderedMap()
				nested.Set("records", list)
				converted.Set(key, nested)
				continue
			}
			if id, isText := value.(string); isText && shape != nil {
				if _, isReference := shape.references[key]; isReference {
					if ref, known := e.refs[id]; known {
						value = "@" + ref
					}
				}
			}
			converted.Set(key, value)
		}
	}
	return converted
}

// flatten returns a record and its nested records as separate records.  A
// child gets its lookup field set to the Id of its parent.
func (e *treeExport) flatten(record *orderedMap) []*orderedMap {
	result := []*orderedMap{record}
	shape := e.shapes[recordType(record)]
	for _, key := range record.keys {
		children, ok := nestedRecords(record.values[key])
		if !ok {
			continue
		}
		for _, child := range children {
			if field := shape.childFields[key]; field != "" {
				if _, present := child.Get(field); !present {
					child.Set(field, recordID(record))
				}
			}
			result = append(result, e.flatten(child)...)
		}
	}
	return result
}

func indentJSON(value interface{}) ([]byte, error) {
	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buffer.Bytes(), "\n"), nil
}

func newDataExportTreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "tree",
		Short:       "Export data from an org into one or more JSON files.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Export"},
		RunE: func(cmd *cobra.Command, args []string) error {
			queries, _ := cmd.Flags().GetStringArray("query")
			if len(queries) == 0 {
				return missingFlagError("query")
			}
			prefix, _ := cmd.Flags().GetString("prefix")
			if strings.ContainsAny(prefix, `/\`) {
				return newError("PrefixSlashError", "`--prefix` cannot contain a forward slash or backslash.")
			}
			for i, query := range queries {
				if content, err := os.ReadFile(query); err == nil {
					queries[i] = strings.TrimSpace(string(content))
				}
				if !strings.HasPrefix(strings.ToLower(queries[i]), "select") {
					return newError("soqlInvalid", "Invalid SOQL query: %s", queries[i])
				}
			}
			outputDir, _ := cmd.Flags().GetString("output-dir")
			plan, _ := cmd.Flags().GetBool("plan")
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			if outputDir != "" {
				if err := os.MkdirAll(outputDir, 0o755); err != nil {
					return err
				}
			}
			export := &treeExport{org: org, refs: map[string]string{}, counts: map[string]int{}, shapes: map[string]*sobjectShape{}}
			results := make([][]*orderedMap, len(queries))
			for i, query := range queries {
				result, err := runQuery(org, "", "query", query)
				if err != nil {
					return queryError(err)
				}
				items, _ := result.values["records"].([]interface{})
				for _, item := range items {
					if record, ok := item.(*orderedMap); ok {
						if err := export.register(record); err != nil {
							return err
						}
						results[i] = append(results[i], record)
					}
				}
			}
			fileName := func(name string) string {
				if prefix != "" {
					name = prefix + "-" + name
				}
				return filepath.Join(outputDir, name)
			}
			out := cmd.OutOrStdout()
			log := func(format string, values ...interface{}) {
				if !jsonRequested(cmd) {
					fmt.Fprintf(out, format, values...)
				}
			}
			write := func(path string, contents interface{}) error {
				data, err := indentJSON(contents)
				if err != nil {
					return err
				}
				return os.WriteFile(path, data, 0o644)
			}
			if plan {
				// One file for each type, and a plan file that lists them.
				byType := map[string][]interface{}{}
				for _, records := range results {
					for _, record := range records {
						for _, flat := range export.flatten(record) {
							objectType := recordType(flat)
							byType[objectType] = append(byType[objectType], export.convert(flat, false))
						}
					}
				}
				planContent := []*orderedMap{}
				for _, objectType := range export.types {
					records := byType[objectType]
					if len(records) == 0 {
						continue
					}
					file := objectType + ".json"
					if prefix != "" {
						file = prefix + "-" + file
					}
					contents := newOrderedMap()
					contents.Set("records", records)
					if err := write(filepath.Join(outputDir, file), contents); err != nil {
						return err
					}
					log("wrote %d records to %s\n", len(records), filepath.Join(outputDir, file))
					part := newOrderedMap()
					part.Set("sobject", objectType)
					part.Set("files", []string{file})
					planContent = append(planContent, part)
				}
				planName := "plan.json"
				if len(queries) == 1 {
					planName = strings.Join(append(append([]string{}, export.types...), "plan.json"), "-")
				}
				if err := write(fileName(planName), planContent); err != nil {
					return err
				}
				return emit(cmd, planContent, nil)
			}
			all := []interface{}{}
			for i, records := range results {
				if len(records) > 200 {
					warn(cmd, fmt.Sprintf("Query returned more than 200 records. Run the command using the --plan flag instead.\nRecord Count: %d\nQuery: %s", len(records), queries[i]))
				}
				if len(records) == 0 {
					// sf names the object of the query.
					object := ""
					if match := queryObjectPattern.FindStringSubmatch(queries[i]); match != nil {
						object = match[1]
					}
					warn(cmd, fmt.Sprintf("The query for %s returned 0 records.", object))
					continue
				}
				converted := make([]interface{}, 0, len(records))
				for _, record := range records {
					converted = append(converted, export.convert(record, true))
				}
				all = append(all, converted...)
				name := strings.Join(export.types, "-")
				if len(queries) > 1 {
					name = recordType(records[0])
				}
				path := fileName(name + ".json")
				contents := newOrderedMap()
				contents.Set("records", converted)
				if err := write(path, contents); err != nil {
					return err
				}
				log("wrote %d records to %s\n", len(records), path)
			}
			result := newOrderedMap()
			result.Set("records", all)
			return emit(cmd, result, nil)
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringArrayP("query", "q", nil, "SOQL query, or filepath of a file that contains the query, to retrieve records.")
	cmd.Flags().BoolP("plan", "p", false, "Generate multiple sObject tree files and a plan definition file for aggregated import.")
	cmd.Flags().StringP("prefix", "x", "", "Prefix of generated files.")
	cmd.Flags().StringP("output-dir", "d", "", "Directory in which to generate the JSON files; default is current directory.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

var queryObjectPattern = regexp.MustCompile(`(?i)from (\w+)`)

// resolveRefs replaces "@<reference id>" values with the Ids of records
// that were already imported.
func resolveRefs(value interface{}, ids map[string]string) interface{} {
	switch v := value.(type) {
	case string:
		if strings.HasPrefix(v, "@") {
			if id, ok := ids[v[1:]]; ok {
				return id
			}
		}
	case *orderedMap:
		for _, key := range v.keys {
			if key != "attributes" {
				v.values[key] = resolveRefs(v.values[key], ids)
			}
		}
	case []interface{}:
		for i, item := range v {
			v[i] = resolveRefs(item, ids)
		}
	}
	return value
}

// importTreeFile posts the records of one sObject tree file and records the
// Id of each reference.
func importTreeFile(org *targetOrg, path string, ids map[string]string) ([]*orderedMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fileSystemError("open", path, err)
	}
	parsed, err := parseOrderedJSON(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	contents, _ := parsed.(*orderedMap)
	var records []interface{}
	if contents != nil {
		records, _ = contents.values["records"].([]interface{})
	}
	if len(records) == 0 {
		return nil, newError("dataFileEmpty", "Data file is empty: %s.", path)
	}
	first, _ := records[0].(*orderedMap)
	objectType := ""
	if first != nil {
		objectType = recordType(first)
	}
	types := map[string]string{}
	var collect func(value interface{})
	collect = func(value interface{}) {
		switch v := value.(type) {
		case *orderedMap:
			if attributes, ok := v.values["attributes"].(*orderedMap); ok {
				types[text(attributes.values["referenceId"])] = text(attributes.values["type"])
			}
			for _, key := range v.keys {
				collect(v.values[key])
			}
		case []interface{}:
			for _, item := range v {
				collect(item)
			}
		}
	}
	collect(records)
	var results []*orderedMap
	// The composite tree resource accepts 200 records in one request.
	for start := 0; start < len(records); start += 200 {
		end := start + 200
		if end > len(records) {
			end = len(records)
		}
		request := newOrderedMap()
		request.Set("records", resolveRefs(records[start:end], ids))
		content, err := encodeJSON(request)
		if err != nil {
			return nil, err
		}
		body, err := org.Force.PostREST("/composite/tree/"+objectType, content)
		if err != nil {
			if failed := treeImportErrors(err); failed != nil {
				return nil, failed
			}
			// sf reports a failed request under the API error, without
			// the "data" field.
			converted := apiError(err)
			if e, ok := converted.(*Error); ok {
				e.Label, e.Data = "", nil
			}
			return nil, converted
		}
		var response struct {
			HasErrors bool `json:"hasErrors"`
			Results   []struct {
				ReferenceId string `json:"referenceId"`
				Id          string `json:"id"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(body), &response); err != nil {
			return nil, err
		}
		if response.HasErrors {
			return nil, newError("dataImportFailed", "Import failed from file: %s. Results: %s.", path, body)
		}
		for _, item := range response.Results {
			ids[item.ReferenceId] = item.Id
			result := newOrderedMap()
			result.Set("refId", item.ReferenceId)
			result.Set("type", types[item.ReferenceId])
			result.Set("id", item.Id)
			results = append(results, result)
		}
	}
	return results, nil
}

// treeImportFailure is a refused tree import: the errors of the records by
// reference id.
type treeImportFailure struct {
	rows []*orderedMap
}

func (f *treeImportFailure) Error() string { return "Data Import failed" }

// treeImportErrors reads the errors of the records from the response the
// org gives when it refuses a tree import, or returns nil for another
// error.
func treeImportErrors(err error) *treeImportFailure {
	message := err.Error()
	start := strings.Index(message, "{")
	if start < 0 {
		return nil
	}
	var response struct {
		HasErrors bool `json:"hasErrors"`
		Results   []struct {
			ReferenceId string `json:"referenceId"`
			Errors      []struct {
				StatusCode string   `json:"statusCode"`
				Message    string   `json:"message"`
				Fields     []string `json:"fields"`
			} `json:"errors"`
		} `json:"results"`
	}
	if json.Unmarshal([]byte(message[start:]), &response) != nil || !response.HasErrors {
		return nil
	}
	failure := &treeImportFailure{}
	for _, result := range response.Results {
		for _, problem := range result.Errors {
			fields := strings.Join(problem.Fields, ", ")
			if fields == "" {
				fields = "N/A"
			}
			failure.rows = append(failure.rows, orderedFields("referenceId", result.ReferenceId, "StatusCode", problem.StatusCode, "Message", problem.Message, "fields", fields))
		}
	}
	return failure
}

func newDataImportTreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "tree",
		Short:       "Import data from one or more JSON files into an org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Import"},
		RunE: func(cmd *cobra.Command, args []string) error {
			files := splitList(cmd, "files")
			plan, _ := cmd.Flags().GetString("plan")
			if len(files) > 0 && plan != "" {
				return &Error{Message: "The following errors occurred:\n  --files cannot also be provided when using --plan\n  --plan cannot also be provided when using --files\nSee more help with --help", ExitCode: 2}
			}
			if len(files) == 0 && plan == "" {
				return exactlyOneError("files", "plan")
			}
			if plan != "" {
				data, err := os.ReadFile(plan)
				if err != nil {
					return fileSystemError("open", plan, err)
				}
				var parts []struct {
					Sobject     string        `json:"sobject"`
					Files       []interface{} `json:"files"`
					SaveRefs    *bool         `json:"saveRefs"`
					ResolveRefs *bool         `json:"resolveRefs"`
				}
				if err := json.Unmarshal(data, &parts); err != nil {
					return &Error{
						Name:     "InvalidDataImportError",
						Message:  fmt.Sprintf("Data plan file %s did not validate against the schema. Errors: %s.", plan, err.Error()),
						ExitCode: 1,
						Actions: []string{
							"Did you run the \"" + binName + " data export tree\" command with the --plan flag?",
							"Make sure you're importing a plan definition file.",
							"Get help with the import plan schema by running \"" + binName + " data import tree --help\".",
						},
					}
				}
				for _, part := range parts {
					if part.SaveRefs != nil || part.ResolveRefs != nil {
						warn(cmd, "The plan contains the 'saveRefs' and/or 'resolveRefs' properties. These properties will be ignored and can be removed.")
						break
					}
				}
				// Files in a plan are relative to the plan file.
				for _, part := range parts {
					for _, file := range part.Files {
						name, ok := file.(string)
						if !ok {
							return newError("NonStringFilesError", "The `files` property of the plan objects must contain only strings")
						}
						files = append(files, filepath.Join(filepath.Dir(plan), name))
					}
				}
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			ids := map[string]string{}
			results := []*orderedMap{}
			for _, file := range files {
				imported, err := importTreeFile(org, file, ids)
				if failure, refused := err.(*treeImportFailure); refused {
					// sf lists the errors of the records, then fails.
					if !jsonRequested(cmd) {
						table := make([][]string, 0, len(failure.rows))
						for _, row := range failure.rows {
							table = append(table, []string{text(row.values["referenceId"]), text(row.values["StatusCode"]), text(row.values["Message"]), text(row.values["fields"])})
						}
						printTable(cmd.OutOrStdout(), "Tree import errors", []string{"Reference ID", "Status Code", "Error Message", "Fields"}, table)
					}
					return &Error{Name: "SfError", Message: "Data Import failed", ExitCode: 1, Data: failure.rows}
				}
				if err != nil {
					return err
				}
				results = append(results, imported...)
			}
			rows := make([][]string, 0, len(results))
			for _, result := range results {
				rows = append(rows, []string{text(result.values["refId"]), text(result.values["type"]), text(result.values["id"])})
			}
			return emit(cmd, results, func(w io.Writer) {
				printTable(w, "Import Results", []string{"Reference ID", "Type", "ID"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringArrayP("files", "f", nil, "Comma-separated and in-order JSON files that contain the records, in sObject tree format, that you want to insert.")
	cmd.Flags().StringP("plan", "p", "", "Plan definition file to insert multiple data files.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

func newDataCreateFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "file",
		Short:       "Upload a local file to an org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DataCreateFile"},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			if file == "" {
				return missingFlagError("file")
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return flagParseError("", "file", "No file found at "+file)
			}
			title, _ := cmd.Flags().GetString("title")
			parentID, _ := cmd.Flags().GetString("parent-id")
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			if title == "" {
				title = filepath.Base(file)
			}
			request := newOrderedMap()
			request.Set("Title", title)
			request.Set("PathOnClient", filepath.Base(file))
			request.Set("VersionData", base64.StdEncoding.EncodeToString(data))
			if parentID != "" {
				request.Set("FirstPublishLocationId", parentID)
			}
			content, err := encodeJSON(request)
			if err != nil {
				return err
			}
			body, err := org.Force.PostREST("/sobjects/ContentVersion", content)
			if err != nil {
				converted := apiError(err)
				if e, ok := converted.(*Error); ok && parentID != "" && e.Name == "INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY" {
					e.Actions = []string{"Check that the record ID is correct and that you have access to it."}
				}
				return converted
			}
			var created struct {
				Id string `json:"id"`
			}
			if err := json.Unmarshal([]byte(body), &created); err != nil || created.Id == "" {
				return fmt.Errorf("the org did not return a ContentVersion Id")
			}
			queried, err := runQuery(org, "", "query", fmt.Sprintf("SELECT Id, ContentDocumentId, Title, FileExtension FROM ContentVersion WHERE Id='%s'", created.Id))
			if err != nil {
				return apiError(err)
			}
			result := newOrderedMap()
			result.Set("Id", created.Id)
			if records, _ := queried.values["records"].([]interface{}); len(records) > 0 {
				if record, ok := records[0].(*orderedMap); ok {
					result = record
				}
			}
			return emit(cmd, result, func(w io.Writer) {
				fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Created file with ContentDocumentId %s.", text(result.values["ContentDocumentId"]))))
				if parentID != "" {
					fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("File attached to record with ID %s.", parentID)))
				}
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("title", "t", "", "New title given to the file (ContentDocument) after it's uploaded.")
	cmd.Flags().StringP("file", "f", "", "Path of file to upload.")
	cmd.Flags().StringP("parent-id", "i", "", "ID of the record to attach the file to.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}
