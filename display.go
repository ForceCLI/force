package main

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

func DisplayForceSobjects(sobjects []ForceSobject) {
	names := make([]string, len(sobjects))
	for i, sobject := range sobjects {
		names[i] = sobject["name"].(string)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Println(name)
	}
}

func DisplayForceRecords(records []ForceRecord) {
	fmt.Println(RenderForceRecords(records))
}

func hasColumn(haystack []string, needle string) bool {
	for _, column := range haystack {
		if column == needle {
			return true
		}
	}
	return false
}

func recordColumns(records []ForceRecord) (columns []string) {
	for _, record := range records {
		for key, _ := range record {
			if key == "attributes" {
				continue
			}
			found := false
			for _, column := range columns {
				if column == key {
					found = true
					break
				}
			}
			if !found {
				columns = append(columns, key)
			}
		}
	}
	return
}

func coerceForceRecords(value map[string]interface{}) (records []ForceRecord) {
	if value["records"] == nil {
		records = make([]ForceRecord, 1)
		records[0] = ForceRecord(value)
	} else {
		records = make([]ForceRecord, len(value["records"].([]interface{})))
		for i, record := range value["records"].([]interface{}) {
			records[i] = ForceRecord(record.(map[string]interface{}))
		}
	}
	return
}

func columnLengths(records []ForceRecord) (lengths map[string]int) {
	lengths = make(map[string]int)

	columns := recordColumns(records)
	for _, column := range columns {
		lengths[column] = len(column) + 2
	}

	for _, record := range records {
		for key, value := range record {
			if key == "attributes" {
				continue
			}
			length := 0
			switch value := value.(type) {
			case map[string]interface{}:
				records := coerceForceRecords(value)
				lengths := columnLengths(records)
				for _, l := range lengths {
					length += l
				}
				length += len(lengths) - 1
			default:
				length = len(fmt.Sprintf(" %v ", value))
			}
			if length > lengths[key] {
				lengths[key] = length
			}
		}
	}
	return
}

func recordHeader(columns []string, lengths map[string]int) (out string) {
	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = fmt.Sprintf(fmt.Sprintf(" %%-%ds ", lengths[column]-2), column)
	}
	out = strings.Join(headers, "|")
	return
}

func recordSeparator(columns []string, lengths map[string]int) (out string) {
	separators := make([]string, len(columns))
	for i, column := range columns {
		separators[i] = strings.Repeat("-", lengths[column])
	}
	out = strings.Join(separators, "+")
	return
}

func recordRow(record ForceRecord, columns []string, lengths map[string]int) (out string) {
	values := make([]string, len(columns))
	for i, column := range columns {
		value := record[column]
		switch value := value.(type) {
		case map[string]interface{}:
			records := coerceForceRecords(value)
			values[i] = RenderForceRecords(records)
		default:
			values[i] = fmt.Sprintf(fmt.Sprintf(" %%-%ds ", lengths[column]-2), value)
		}
	}
	maxrows := 1
	for _, value := range values {
		rows := len(strings.Split(value, "\n"))
		if rows > maxrows {
			maxrows = rows
		}
	}
	rows := make([]string, maxrows)
	for i := 0; i < maxrows; i++ {
		rowvalues := make([]string, len(columns))
		for j, column := range columns {
			parts := strings.Split(values[j], "\n")
			if i < len(parts) {
				rowvalues[j] = fmt.Sprintf(fmt.Sprintf("%%-%ds", lengths[column]), parts[i])
			} else {
				rowvalues[j] = strings.Repeat(" ", lengths[column])
			}
		}
		rows[i] = strings.Join(rowvalues, "|")
	}
	out = strings.Join(rows, "\n")
	return
}

func RenderForceRecords(records []ForceRecord) string {
	var out bytes.Buffer

	columns := recordColumns(records)
	lengths := columnLengths(records)

	out.WriteString(recordHeader(columns, lengths) + "\n")
	out.WriteString(recordSeparator(columns, lengths) + "\n")

	for _, record := range records {
		out.WriteString(recordRow(record, columns, lengths) + "\n")
	}

	return out.String()
}

func DisplayForceRecord(record ForceRecord) {
	DisplayInterfaceMap(record, 0)
}

func DisplayInterfaceMap(object map[string]interface{}, indent int) {
	keys := make([]string, len(object))
	i := 0
	for key, _ := range object {
		keys[i] = key
		i++
	}
	sort.Strings(keys)
	for _, key := range keys {
		for i := 0; i < indent; i++ {
			fmt.Printf("  ")
		}
		fmt.Printf("%s: ", key)
		switch v := object[key].(type) {
		case map[string]interface{}:
			fmt.Printf("\n")
			DisplayInterfaceMap(v, indent+1)
		default:
			fmt.Printf("%v\n", v)
		}
	}
}

func StringSliceToInterfaceSlice(s []string) (i []interface{}) {
	for _, str := range s {
		i = append(i, interface{}(str))
	}
	return
}

type ForceSobjectFields []interface{}

func DisplayForceSobject(sobject ForceSobject) {
	fields := ForceSobjectFields(sobject["fields"].([]interface{}))
	sort.Sort(fields)
	for _, f := range fields {
		field := f.(map[string]interface{})
		switch field["type"] {
		case "picklist":
			var values []string
			for _, value := range field["picklistValues"].([]interface{}) {
				values = append(values, value.(map[string]interface{})["value"].(string))
			}
			fmt.Printf("%s: %s (%s)\n", field["name"], field["type"], strings.Join(values, ", "))
		case "reference":
			var refs []string
			for _, ref := range field["referenceTo"].([]interface{}) {
				refs = append(refs, ref.(string))
			}
			fmt.Printf("%s: %s (%s)\n", field["name"], field["type"], strings.Join(refs, ", "))
		default:
			fmt.Printf("%s: %s\n", field["name"], field["type"])
		}
	}
}
