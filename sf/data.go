package sf

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newDataCmd() *cobra.Command {
	return newGroup("data", "Manage records in your org.",
		newDataQueryCmd(),
		newDataSearchCmd(),
		newLegacyBulkStatusCmd("resume", "View the status of a bulk data load job or batch. Uses Bulk API 1.0."),
		newGroup("bulk", "Get the results of bulk jobs.", newDataBulkResultsCmd()),
		newGroup("create", "Create a record or upload a file.", newDataCreateRecordCmd(), newDataCreateFileCmd()),
		newGroup("get", "Retrieve a record.", newDataGetRecordCmd()),
		newGroup("import", "Import records.",
			newDataBulkIngestCmd(bulkOperations["import"], "Bulk import records into a Salesforce object from a CSV file. Uses Bulk API 2.0."),
			newDataBulkResumeCmd(bulkOperations["import"], "Resume a bulk import job that you previously started. Uses Bulk API 2.0."),
			newDataImportTreeCmd(),
		),
		newGroup("export", "Export records.", newDataExportBulkCmd(), newDataExportResumeCmd(), newDataExportTreeCmd()),
		newGroup("update", "Update records.",
			newDataUpdateRecordCmd(),
			newDataBulkIngestCmd(bulkOperations["update"], "Bulk update records to an org from a CSV file. Uses Bulk API 2.0."),
			newDataBulkResumeCmd(bulkOperations["update"], "Resume a bulk update job that you previously started. Uses Bulk API 2.0."),
		),
		newGroup("upsert", "Upsert records.",
			newDataBulkIngestCmd(bulkOperations["upsert"], "Bulk upsert records to an org from a CSV file. Uses Bulk API 2.0."),
			newDataBulkResumeCmd(bulkOperations["upsert"], "Resume a bulk upsert job that you previously started. Uses Bulk API 2.0."),
		),
		newGroup("delete", "Delete records.",
			newDataDeleteRecordCmd(),
			newDataBulkIngestCmd(bulkOperations["delete"], "Bulk delete records from an org using a CSV file. Uses Bulk API 2.0."),
			newDataBulkResumeCmd(bulkOperations["delete"], "Resume a bulk delete job that you previously started. Uses Bulk API 2.0."),
		),
	)
}

// restBase returns the path prefix for REST or Tooling API resources,
// relative to /services/data/vXX.0.
func restBase(cmd *cobra.Command) string {
	if tooling, _ := cmd.Flags().GetBool("use-tooling-api"); tooling {
		return "/tooling"
	}
	return ""
}

// runQuery runs a SOQL query and follows nextRecordsUrl until every record
// is read.  Records keep the field order of the response.
func runQuery(org *targetOrg, base string, resource string, soql string) (*orderedMap, error) {
	body, err := org.Force.GetREST(base + "/" + resource + "?q=" + url.QueryEscape(soql))
	if err != nil {
		return nil, err
	}
	result := newOrderedMap()
	records := []interface{}{}
	for {
		parsed, err := parseOrderedJSON([]byte(body))
		if err != nil {
			return nil, err
		}
		page, ok := parsed.(*orderedMap)
		if !ok {
			return nil, fmt.Errorf("unexpected response from the query resource")
		}
		if pageRecords, ok := page.values["records"].([]interface{}); ok {
			records = append(records, pageRecords...)
		}
		result.Set("records", records)
		result.Set("totalSize", page.values["totalSize"])
		result.Set("done", true)
		next, _ := page.values["nextRecordsUrl"].(string)
		if next == "" {
			return result, nil
		}
		body, err = org.Force.GetAbsolute(next)
		if err != nil {
			return nil, err
		}
	}
}

// recordColumns returns the column names for query output: the fields of
// the records in order, with the fields of a parent record named
// Parent.Field.  The attributes entries are left out.
func recordColumns(records []interface{}) []string {
	var columns []string
	seen := map[string]bool{}
	var walk func(prefix string, record *orderedMap)
	walk = func(prefix string, record *orderedMap) {
		for _, key := range record.keys {
			if key == "attributes" {
				continue
			}
			name := prefix + key
			if child, ok := record.values[key].(*orderedMap); ok {
				if _, isChildList := child.values["records"]; !isChildList {
					walk(name+".", child)
					continue
				}
			}
			if !seen[name] {
				seen[name] = true
				columns = append(columns, name)
			}
		}
	}
	for _, record := range records {
		if object, ok := record.(*orderedMap); ok {
			walk("", object)
		}
	}
	return columns
}

// recordValue returns the value of a column named by recordColumns.
func recordValue(record *orderedMap, column string) interface{} {
	current := record
	parts := strings.Split(column, ".")
	for i, part := range parts {
		value, ok := current.values[part]
		if !ok {
			return nil
		}
		if i == len(parts)-1 {
			return value
		}
		next, ok := value.(*orderedMap)
		if !ok {
			return nil
		}
		current = next
	}
	return nil
}

// selectClauseColumns returns the field expressions between SELECT and
// FROM.  It is used for the table header when a query returns a count and
// no records.
func selectClauseColumns(soql string) []string {
	upper := strings.ToUpper(soql)
	start := strings.Index(upper, "SELECT")
	end := strings.Index(upper, " FROM ")
	if start < 0 || end < start {
		return nil
	}
	var columns []string
	for _, field := range strings.Split(soql[start+len("SELECT"):end], ",") {
		if field = strings.TrimSpace(field); field != "" {
			columns = append(columns, field)
		}
	}
	return columns
}

func writeQueryHuman(w io.Writer, soql string, result *orderedMap) {
	records, _ := result.values["records"].([]interface{})
	total := displayValue(result.values["totalSize"])
	// A query without records, such as SELECT COUNT(), has no table: sf
	// prints an empty line and the total.
	columns := recordColumns(records)
	if len(columns) == 0 {
		fmt.Fprintln(w)
	} else {
		headers := make([]string, len(columns))
		for i, column := range columns {
			headers[i] = strings.ToUpper(column)
		}
		rows := make([][]string, 0, len(records))
		for _, record := range records {
			object, ok := record.(*orderedMap)
			if !ok {
				continue
			}
			row := make([]string, len(columns))
			for i, column := range columns {
				row[i] = displayValue(recordValue(object, column))
			}
			rows = append(rows, row)
		}
		printTable(w, "", headers, rows)
	}
	fmt.Fprintf(w, "Total number of records retrieved: %s.\n", total)
}

func writeQueryCSV(w io.Writer, result *orderedMap) error {
	records, _ := result.values["records"].([]interface{})
	columns := recordColumns(records)
	if len(columns) == 0 {
		// sf writes the header line, which has no columns.
		fmt.Fprintln(w)
		return nil
	}
	writer := csv.NewWriter(w)
	if err := writer.Write(columns); err != nil {
		return err
	}
	for _, record := range records {
		object, ok := record.(*orderedMap)
		if !ok {
			continue
		}
		row := make([]string, len(columns))
		for i, column := range columns {
			if value := recordValue(object, column); value != nil {
				row[i] = displayValue(value)
			}
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

// exactlyOneError is the error sf reports when none or several of a set of
// exclusive flags are given.
func exactlyOneError(flags ...string) *Error {
	for i, flag := range flags {
		flags[i] = "--" + flag
	}
	return &Error{
		Message:  fmt.Sprintf("The following error occurred:\n  Exactly one of the following must be provided: %s\nSee more help with --help", strings.Join(flags, ", ")),
		ExitCode: 2,
	}
}

// queryText returns the query from --query or from the file named by
// --file.
func queryText(cmd *cobra.Command) (string, error) {
	query, _ := cmd.Flags().GetString("query")
	file, _ := cmd.Flags().GetString("file")
	if (query == "") == (file == "") {
		return "", exactlyOneError("file", "query")
	}
	if file != "" {
		content, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		query = strings.TrimSpace(string(content))
	}
	return query, nil
}

// queryError converts an API error from a query.  sf prints the exit code
// in place of the error name for these errors.
func queryError(err error) error {
	converted := apiError(err)
	if e, ok := converted.(*Error); ok && e.Name != "" {
		e.Label = "1"
	}
	return converted
}

// wrappedAPIError converts an API error for the commands of sf that wrap
// it in an error of their own with the same name and message: "data get
// record" and "data delete record".  The name is then shown in the human
// message and in "code", and the error has no data.
func wrappedAPIError(err error) error {
	converted := apiError(err)
	if e, ok := converted.(*Error); ok {
		e.Label, e.Data = "", nil
	}
	return converted
}

func newDataQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "query",
		Short:       "Execute a SOQL query.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DataSoqlQueryCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			soql, err := queryText(cmd)
			if err != nil {
				return err
			}
			format, _ := cmd.Flags().GetString("result-format")
			if format != "human" && format != "csv" && format != "json" {
				return &Error{Message: fmt.Sprintf("Expected --result-format=%s to be one of: human, csv, json\nSee more help with --help", format), ExitCode: 1}
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			resource := "query"
			if allRows, _ := cmd.Flags().GetBool("all-rows"); allRows {
				resource = "queryAll"
			}
			// sf prints no progress line with --result-format json.
			stop := func(string) {}
			if format != "json" {
				stop = startSpinner(cmd, "Querying Data")
			}
			result, err := runQuery(org, restBase(cmd), resource, soql)
			stop("done")
			if err != nil {
				return queryError(err)
			}
			if jsonRequested(cmd) {
				return emit(cmd, result, nil)
			}
			// sf lays out the table and the CSV file from the column
			// metadata of the query, which it requests when the query
			// returned records.
			var fields []queryField
			records, _ := result.values["records"].([]interface{})
			if total := displayValue(result.values["totalSize"]); total != "0" && format != "json" {
				fields = queryColumns(org, restBase(cmd), resource, soql)
			}
			out := cmd.OutOrStdout()
			var buffer bytes.Buffer
			switch format {
			case "json":
				// sf prints the envelope without "warnings" here.
				envelope := newOrderedMap()
				envelope.Set("status", 0)
				envelope.Set("result", result)
				if err := writeJSON(&buffer, envelope); err != nil {
					return err
				}
			case "csv":
				if len(fields) > 0 {
					writeQueryCSVColumns(&buffer, result, fields)
				} else if err := writeQueryCSV(&buffer, result); err != nil {
					return err
				}
			default:
				if len(fields) > 0 && len(records) > 0 {
					writeQueryTable(&buffer, result, fields)
				} else {
					writeQueryHuman(&buffer, soql, result)
				}
			}
			if outputFile, _ := cmd.Flags().GetString("output-file"); outputFile != "" {
				if err := os.WriteFile(outputFile, buffer.Bytes(), 0o644); err != nil {
					return err
				}
				records, _ := result.values["records"].([]interface{})
				fmt.Fprintf(out, "%d records written to %s\n", len(records), outputFile)
				return nil
			}
			_, err = out.Write(buffer.Bytes())
			return err
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("query", "q", "", "SOQL query to execute.")
	cmd.Flags().StringP("file", "f", "", "File that contains the SOQL query.")
	cmd.Flags().BoolP("use-tooling-api", "t", false, "Use Tooling API so you can run queries on Tooling API objects.")
	cmd.Flags().Bool("all-rows", false, "Include deleted records. By default, deleted records are not returned.")
	cmd.Flags().StringP("result-format", "r", "human", "Format to display the results; the --json flag overrides this flag.")
	cmd.Flags().String("output-file", "", "File where records are written; only CSV and JSON output formats are supported.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

// parseKeyValueSequence splits text such as
//
//	Name='Acme Corp' Phone=555
//
// into key=value pairs.  It follows sf: quotes group words, and a quote
// character is literal when the text contains only one of it.
func parseKeyValueSequence(text string) []string {
	text = strings.TrimSpace(text)
	singleQuotes := strings.Count(text, "'")
	doubleQuotes := strings.Count(text, `"`)
	var (
		pairs         []string
		current       strings.Builder
		inSingleQuote bool
		inDoubleQuote bool
	)
	for _, char := range text {
		if char == '\'' && !inDoubleQuote && singleQuotes >= 2 {
			inSingleQuote = !inSingleQuote
			continue
		}
		if char == '"' && !inSingleQuote && doubleQuotes >= 2 {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if !inSingleQuote && !inDoubleQuote && (char == ' ' || char == '\t' || char == '\n' || char == '\r') {
			if current.Len() > 0 {
				pairs = append(pairs, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(char)
	}
	if current.Len() > 0 {
		pairs = append(pairs, current.String())
	}
	return pairs
}

// parseValues converts key=value text into field values.  "true" and
// "false" become booleans, a value with braces that is valid JSON is used
// as JSON, and other values are strings.
func parseValues(text string) (*orderedMap, error) {
	values := newOrderedMap()
	for _, pair := range parseKeyValueSequence(text) {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, newError("TextUtilMalformedKeyValuePair", "Malformed key=value pair for value: %s.", pair)
		}
		if strings.Contains(pair, "{") && strings.Contains(pair, "}") {
			if parsed, err := parseOrderedJSON([]byte(value)); err == nil {
				values.Set(key, parsed)
				continue
			}
		}
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true":
			values.Set(key, true)
		case "false":
			values.Set(key, false)
		default:
			values.Set(key, value)
		}
	}
	return values, nil
}

// findRecordId returns the Id of the one record of sobject that matches
// the key=value conditions in where.
func findRecordId(org *targetOrg, base string, sobject string, where string) (string, error) {
	conditions, err := parseValues(where)
	if err != nil {
		return "", err
	}
	var clauses []string
	for _, key := range conditions.keys {
		switch value := conditions.values[key].(type) {
		case bool:
			clauses = append(clauses, fmt.Sprintf("%s = %t", key, value))
		case string:
			escaped := strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `'`, `\'`)
			clauses = append(clauses, fmt.Sprintf("%s = '%s'", key, escaped))
		default:
			clauses = append(clauses, fmt.Sprintf("%s = %s", key, displayValue(value)))
		}
	}
	soql := fmt.Sprintf("SELECT Id FROM %s", sobject)
	if len(clauses) > 0 {
		soql += " WHERE " + strings.Join(clauses, " AND ")
	}
	result, err := runQuery(org, base, "query", soql)
	if err != nil {
		return "", apiError(err)
	}
	records, _ := result.values["records"].([]interface{})
	switch len(records) {
	case 0:
		return "", newError("DataRecordGetNoRecord", "No matching record found.")
	case 1:
		record, _ := records[0].(*orderedMap)
		id, _ := record.values["Id"].(string)
		return id, nil
	}
	return "", newError("DataRecordGetMultipleRecords", "%s is not a unique qualifier for %s; %d records were retrieved.", where, sobject, len(records))
}

func addRecordFlags(cmd *cobra.Command, withId bool, withValues bool) {
	addJSONFlag(cmd)
	cmd.Flags().StringP("sobject", "s", "", "API name of the Salesforce or Tooling API object.")
	if withId {
		cmd.Flags().StringP("record-id", "i", "", "ID of the record.")
		cmd.Flags().StringP("where", "w", "", "List of <fieldName>=<value> pairs that identify the record.")
	}
	if withValues {
		cmd.Flags().StringP("values", "v", "", "Values for the flags in the form <fieldName>=<value>, separate multiple pairs with spaces.")
	}
	cmd.Flags().BoolP("use-tooling-api", "t", false, "Use Tooling API so you can work with Tooling API objects.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
}

// recordTarget reads --sobject and resolves the record Id from --record-id
// or --where.
func recordTarget(cmd *cobra.Command, org *targetOrg) (sobject string, id string, err error) {
	sobject, _ = cmd.Flags().GetString("sobject")
	id, _ = cmd.Flags().GetString("record-id")
	where, _ := cmd.Flags().GetString("where")
	if id == "" {
		id, err = findRecordId(org, restBase(cmd), sobject, where)
	}
	return
}

func requireRecordFlags(cmd *cobra.Command, withId bool, withValues bool) error {
	if sobject, _ := cmd.Flags().GetString("sobject"); sobject == "" {
		return missingFlagError("sobject")
	}
	if withId {
		id, _ := cmd.Flags().GetString("record-id")
		where, _ := cmd.Flags().GetString("where")
		if (id == "") == (where == "") {
			return exactlyOneError("where", "record-id")
		}
	}
	if withValues {
		if values, _ := cmd.Flags().GetString("values"); values == "" {
			return missingFlagError("values")
		}
	}
	return nil
}

func encodeJSON(value interface{}) (string, error) {
	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimRight(buffer.String(), "\n"), nil
}

func newDataCreateRecordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "record",
		Short:       "Create and insert a record into a Salesforce or Tooling API object.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Create"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireRecordFlags(cmd, false, true); err != nil {
				return err
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			sobject, _ := cmd.Flags().GetString("sobject")
			text, _ := cmd.Flags().GetString("values")
			values, err := parseValues(text)
			if err != nil {
				return err
			}
			content, err := encodeJSON(values)
			if err != nil {
				return err
			}
			stop := startSpinner(cmd, fmt.Sprintf("Creating record for %s", sobject))
			body, err := org.Force.PostREST(restBase(cmd)+"/sobjects/"+sobject, content)
			if err != nil {
				// sf ends the line of this command with "Error" and
				// those of the update and delete commands with "Failed".
				stop("Error")
				return apiError(err)
			}
			result, err := parseOrderedJSON([]byte(body))
			if err != nil {
				stop("failed")
				return err
			}
			id := "unknown id"
			if object, ok := result.(*orderedMap); ok {
				if value, ok := object.values["id"].(string); ok && value != "" {
					id = value
				}
			}
			err = emit(cmd, result, func(w io.Writer) {
				fmt.Fprintf(w, "Successfully created record: %s.\n", id)
			})
			stop("done")
			return err
		},
	}
	addRecordFlags(cmd, false, true)
	return cmd
}

// writeNestedObject prints "key: value" lines, with the fields of a nested
// object indented by two spaces under its key.
func writeNestedObject(w io.Writer, object *orderedMap, indent string) {
	for _, key := range object.keys {
		if child, ok := object.values[key].(*orderedMap); ok {
			fmt.Fprintf(w, "%s%s:\n", indent, key)
			writeNestedObject(w, child, indent+"  ")
			continue
		}
		fmt.Fprintf(w, "%s%s: %s\n", indent, key, displayValue(object.values[key]))
	}
}

func newDataGetRecordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "record",
		Short:       "Retrieve and display a single record of a Salesforce or Tooling API object.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Get"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireRecordFlags(cmd, true, false); err != nil {
				return err
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			stop := startSpinner(cmd, "Getting Record")
			sobject, id, err := recordTarget(cmd, org)
			if err != nil {
				stop("failed")
				return err
			}
			body, err := org.Force.GetREST(restBase(cmd) + "/sobjects/" + sobject + "/" + id)
			if err != nil {
				stop("failed")
				return wrappedAPIError(err)
			}
			result, err := parseOrderedJSON([]byte(body))
			if err != nil {
				stop("failed")
				return err
			}
			err = emit(cmd, result, func(w io.Writer) {
				if object, ok := result.(*orderedMap); ok {
					writeNestedObject(w, object, "")
				}
			})
			stop("done")
			return err
		},
	}
	addRecordFlags(cmd, true, false)
	return cmd
}

// saveResult is the result sf reports for an update or delete, which the
// API answers with no content.
func saveResult(id string) *orderedMap {
	result := newOrderedMap()
	result.Set("id", id)
	result.Set("success", true)
	result.Set("errors", []interface{}{})
	return result
}

func newDataUpdateRecordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "record",
		Short:       "Updates a single record of a Salesforce or Tooling API object.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Update"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireRecordFlags(cmd, true, true); err != nil {
				return err
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			text, _ := cmd.Flags().GetString("values")
			values, err := parseValues(text)
			if err != nil {
				return err
			}
			content, err := encodeJSON(values)
			if err != nil {
				return err
			}
			stop := startSpinner(cmd, "Updating Record")
			sobject, id, err := recordTarget(cmd, org)
			if err != nil {
				stop("failed")
				return err
			}
			if _, err := org.Force.PatchREST(restBase(cmd)+"/sobjects/"+sobject+"/"+id, content); err != nil {
				stop("Failed")
				return apiError(err)
			}
			err = emit(cmd, saveResult(id), func(w io.Writer) {
				fmt.Fprintf(w, "Successfully updated record: %s.\n", id)
			})
			stop("Success")
			return err
		},
	}
	addRecordFlags(cmd, true, true)
	return cmd
}

func newDataDeleteRecordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "record",
		Short:       "Deletes a single record from a Salesforce or Tooling API object.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Delete"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireRecordFlags(cmd, true, false); err != nil {
				return err
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			stop := startSpinner(cmd, "Deleting Record")
			sobject, id, err := recordTarget(cmd, org)
			if err != nil {
				stop("failed")
				return err
			}
			if _, err := org.Force.DeleteREST(restBase(cmd) + "/sobjects/" + sobject + "/" + id); err != nil {
				stop("Failed")
				return wrappedAPIError(err)
			}
			err = emit(cmd, saveResult(id), func(w io.Writer) {
				fmt.Fprintf(w, "Successfully deleted record: %s.\n", id)
			})
			stop("Success")
			return err
		},
	}
	addRecordFlags(cmd, true, false)
	return cmd
}

func newDataSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "search",
		Short:       "Execute a SOSL text-based search query.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DataSearchCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			sosl, err := queryText(cmd)
			if err != nil {
				return err
			}
			format, _ := cmd.Flags().GetString("result-format")
			if format != "human" && format != "csv" && format != "json" {
				return &Error{Message: fmt.Sprintf("Expected --result-format=%s to be one of: human, csv, json\nSee more help with --help", format), ExitCode: 1}
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			stop := func(string) {}
			if format != "json" {
				stop = startSpinner(cmd, "Querying Data")
			}
			body, err := org.Force.GetREST("/search?q=" + url.QueryEscape(sosl))
			stop("done")
			if err != nil {
				return queryError(err)
			}
			result, err := parseOrderedJSON([]byte(body))
			if err != nil {
				return err
			}
			if jsonRequested(cmd) {
				return emit(cmd, result, nil)
			}
			out := cmd.OutOrStdout()
			if format == "json" {
				return writeJSON(out, result)
			}
			// Records are grouped by object type, in the order the types
			// first appear.
			var types []string
			byType := map[string][]*orderedMap{}
			if object, ok := result.(*orderedMap); ok {
				records, _ := object.values["searchRecords"].([]interface{})
				for _, item := range records {
					record, ok := item.(*orderedMap)
					if !ok {
						continue
					}
					objectType := ""
					if attributes, ok := record.values["attributes"].(*orderedMap); ok {
						objectType = text(attributes.values["type"])
					}
					record.Delete("attributes")
					if _, seen := byType[objectType]; !seen {
						types = append(types, objectType)
					}
					byType[objectType] = append(byType[objectType], record)
				}
			}
			if len(types) == 0 {
				fmt.Fprintln(out, "No Records Found")
				return nil
			}
			for _, objectType := range types {
				records := byType[objectType]
				columns := records[0].keys
				rows := make([][]string, 0, len(records))
				for _, record := range records {
					row := make([]string, len(columns))
					for i, column := range columns {
						row[i] = displayValue(record.values[column])
					}
					rows = append(rows, row)
				}
				if format == "csv" {
					fmt.Fprintf(out, "Written to %s.csv\n", objectType)
					lines := []string{strings.Join(columns, ",")}
					for _, row := range rows {
						lines = append(lines, strings.Join(row, ","))
					}
					if err := os.WriteFile(objectType+".csv", []byte(strings.Join(lines, "\n")), 0o644); err != nil {
						return err
					}
					continue
				}
				printTable(out, objectType, columns, rows)
				fmt.Fprintln(out)
			}
			return nil
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("query", "q", "", "SOSL query to execute.")
	cmd.Flags().StringP("file", "f", "", "File that contains the SOSL query.")
	cmd.Flags().StringP("result-format", "r", "human", "Format to display the results, or to write to disk if you specify \"csv\".")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}
