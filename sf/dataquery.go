package sf

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// queryField is a column of a query, as sf derives it from the column
// metadata that the query resource returns with columns=true.
type queryField struct {
	// subquery is set for a child relationship query; fields holds its
	// columns.
	subquery bool
	// function is set for an aggregate function.  alias is the alias given
	// in the query, when there is one.
	function bool
	name     string
	alias    string
	fields   []queryField
}

// label is the name of the column in query output.
func (f queryField) label() string {
	if f.function && f.alias != "" {
		return f.alias
	}
	return f.name
}

var generatedAlias = regexp.MustCompile(`expr[0-9]+`)

// queryColumns asks the org for the columns of a query.  It returns nil
// when the response has no column metadata; the output is then laid out
// from the fields of the records.
func queryColumns(org *targetOrg, base, resource, soql string) []queryField {
	body, err := org.Force.GetREST(base + "/" + resource + "?q=" + url.QueryEscape(soql) + "&columns=true")
	if err != nil {
		return nil
	}
	parsed, err := parseOrderedJSON([]byte(body))
	if err != nil {
		return nil
	}
	response, ok := parsed.(*orderedMap)
	if !ok {
		return nil
	}
	metadata, ok := response.values["columnMetadata"].([]interface{})
	if !ok {
		return nil
	}
	return fieldsOfColumns(metadata)
}

// fieldsOfColumns follows the function of sf that converts column metadata
// to fields: a column with join columns is a subquery when it is marked as
// an aggregate and a parent relationship otherwise.
func fieldsOfColumns(columns []interface{}) []queryField {
	var fields []queryField
	for _, item := range columns {
		column, ok := item.(*orderedMap)
		if !ok {
			continue
		}
		name, _ := column.values["columnName"].(string)
		aggregate, _ := column.values["aggregate"].(bool)
		joins, _ := column.values["joinColumns"].([]interface{})
		switch {
		case len(joins) > 0 && aggregate:
			field := queryField{subquery: true, name: name}
			for _, join := range joins {
				field.fields = append(field.fields, fieldsOfColumns([]interface{}{join})...)
			}
			fields = append(fields, field)
		case len(joins) > 0:
			for _, join := range joins {
				for _, path := range parentColumnPaths(join) {
					fields = append(fields, queryField{name: name + "." + path})
				}
			}
		case aggregate:
			field := queryField{function: true}
			field.name, _ = column.values["displayName"].(string)
			if !generatedAlias.MatchString(name) {
				field.alias = name
			}
			fields = append(fields, field)
		default:
			fields = append(fields, queryField{name: name})
		}
	}
	return fields
}

// parentColumnPaths returns the field paths under a column of a parent
// relationship.
func parentColumnPaths(item interface{}) []string {
	column, ok := item.(*orderedMap)
	if !ok {
		return nil
	}
	name, _ := column.values["columnName"].(string)
	joins, _ := column.values["joinColumns"].([]interface{})
	if len(joins) == 0 {
		return []string{name}
	}
	paths := make([]string, 0, len(joins))
	for _, join := range joins {
		paths = append(paths, name+"."+strings.Join(parentColumnPaths(join), "."))
	}
	return paths
}

// lookupPath returns the value at a dotted path of keys and list indexes.
func lookupPath(value interface{}, path string) (interface{}, bool) {
	for _, part := range strings.Split(path, ".") {
		switch v := value.(type) {
		case *orderedMap:
			next, ok := v.values[part]
			if !ok {
				return nil, false
			}
			value = next
		case []interface{}:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(v) {
				return nil, false
			}
			value = v[index]
		default:
			return nil, false
		}
	}
	return value, true
}

var exprKey = regexp.MustCompile(`expr(\d+)`)

// renameAggregates renames the exprN keys of a record to the labels of the
// aggregate functions of the query.
func renameAggregates(record *orderedMap, aggregates []queryField) *orderedMap {
	if len(aggregates) == 0 {
		return record
	}
	renamed := newOrderedMap()
	for _, key := range record.keys {
		name := key
		if match := exprKey.FindStringSubmatch(key); match != nil {
			if index, err := strconv.Atoi(match[1]); err == nil && index < len(aggregates) {
				name = aggregates[index].label()
			}
		}
		renamed.Set(name, record.values[key])
	}
	return renamed
}

// flattenEntry adds a value to row under key.  The fields of an object are
// added under key.field, without the attributes entry.
func flattenEntry(row *orderedMap, key string, value interface{}) {
	if object, ok := value.(*orderedMap); ok {
		for _, child := range object.keys {
			if child != "attributes" {
				flattenEntry(row, key+"."+child, object.values[child])
			}
		}
		return
	}
	row.Set(key, value)
}

// subqueryRows returns the table rows of a record: one row with the fields
// of the record and the first record of each subquery, then one row for
// each further record of each subquery.
func subqueryRows(record *orderedMap, children []string) []*orderedMap {
	if len(children) == 0 {
		return []*orderedMap{record}
	}
	isChild := map[string]bool{}
	parent := newOrderedMap()
	for _, child := range children {
		isChild[child] = true
	}
	for _, key := range record.keys {
		if !isChild[key] {
			parent.Set(key, record.values[key])
		}
	}
	rows := []*orderedMap{parent}
	for _, child := range children {
		value, _ := lookupPath(record, child)
		list, _ := lookupPath(value, "records")
		childRecords, _ := list.([]interface{})
		for i, item := range childRecords {
			childRecord, ok := item.(*orderedMap)
			if !ok {
				continue
			}
			row := parent
			if i > 0 {
				row = newOrderedMap()
			}
			for _, key := range childRecord.keys {
				if key != "attributes" {
					flattenEntry(row, child+"."+key, childRecord.values[key])
				}
			}
			if i > 0 && len(row.keys) > 0 {
				rows = append(rows, row)
			}
		}
	}
	return rows
}

// writeQueryTable prints records as sf prints them for a query with known
// columns.
func writeQueryTable(w io.Writer, result *orderedMap, fields []queryField) {
	records, _ := result.values["records"].([]interface{})
	var columns, children []string
	var aggregates []queryField
	plain := map[string]bool{}
	for _, field := range fields {
		switch {
		case field.subquery:
			children = append(children, field.name)
			for _, sub := range field.fields {
				columns = append(columns, field.name+"."+sub.name)
			}
		case field.function:
			aggregates = append(aggregates, field)
			columns = append(columns, field.label())
		default:
			plain[field.name] = true
			columns = append(columns, field.name)
		}
	}
	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = strings.ToUpper(column)
	}
	var rows [][]string
	for _, item := range records {
		record, ok := item.(*orderedMap)
		if !ok {
			continue
		}
		record = renameAggregates(record, aggregates)
		for _, values := range subqueryRows(record, children) {
			row := make([]string, len(columns))
			for i, column := range columns {
				value, found := values.values[column]
				if !found {
					value, found = lookupPath(values, column)
				}
				if !found {
					continue
				}
				// A field whose value is an object, such as an address,
				// is shown as indented JSON.
				if object, isObject := value.(*orderedMap); isObject && plain[column] {
					if encoded, err := json.MarshalIndent(object, "", "  "); err == nil {
						row[i] = string(encoded)
						continue
					}
				}
				row[i] = displayValue(value)
			}
			rows = append(rows, row)
		}
	}
	printTable(w, "", headers, rows)
	fmt.Fprintf(w, "Total number of records retrieved: %d.\n", len(records))
}

// csvCell quotes a value that contains a comma, a quote or a line feed.
func csvCell(value string) string {
	if strings.ContainsAny(value, ",\"\n") {
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	}
	return value
}

// writeQueryCSVColumns prints records as CSV for a query with known
// columns.  When a subquery returned records, sf writes its columns once
// for each record of the largest subquery result, each time preceded by a
// totalSize column.
func writeQueryCSVColumns(w io.Writer, result *orderedMap, fields []queryField) {
	items, _ := result.values["records"].([]interface{})
	var aggregates []queryField
	for _, field := range fields {
		if field.function {
			aggregates = append(aggregates, field)
		}
	}
	var records []*orderedMap
	for _, item := range items {
		if record, ok := item.(*orderedMap); ok {
			records = append(records, renameAggregates(record, aggregates))
		}
	}
	largest := map[string]int{}
	anyChildRecords := false
	for _, field := range fields {
		if !field.subquery {
			continue
		}
		for _, record := range records {
			if size, ok := lookupPath(record, field.name+".totalSize"); ok {
				if number, isNumber := size.(json.Number); isNumber {
					if n, err := strconv.Atoi(number.String()); err == nil && n > largest[field.name] {
						largest[field.name] = n
					}
				}
			}
		}
		anyChildRecords = anyChildRecords || largest[field.name] > 0
	}
	var columns []string
	for _, field := range fields {
		switch {
		case !anyChildRecords:
			columns = append(columns, field.name)
		case field.subquery:
			for index := 0; index < largest[field.name]; index++ {
				columns = append(columns, field.name+".totalSize")
				for _, sub := range field.fields {
					columns = append(columns, fmt.Sprintf("%s.records.%d.%s", field.name, index, sub.name))
				}
			}
		default:
			columns = append(columns, field.label())
		}
	}
	header := make([]string, len(columns))
	for i, column := range columns {
		header[i] = csvCell(column)
	}
	fmt.Fprintln(w, strings.Join(header, ","))
	for _, record := range records {
		cells := make([]string, len(columns))
		for i, column := range columns {
			value, found := lookupPath(record, column)
			if !found {
				value, found = record.values[column]
			}
			if !found || value == nil {
				continue
			}
			cells[i] = csvCell(displayValue(value))
		}
		fmt.Fprintln(w, strings.Join(cells, ","))
	}
}
