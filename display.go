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

func DisplayForceRecordsf(records []ForceRecord, format string) {
	switch format {
	case "csv":
		fmt.Println(RenderForceRecordsCSV(records, format))
	default:
		fmt.Printf("Format %s not supported\n\n", format)
	}
}

func DisplayForceRecords(records []ForceRecord) {
	fmt.Println(RenderForceRecords(records))
}

func recordColumns(records []ForceRecord) (columns []string) {
	for _, record := range records {
		for key, _ := range record {
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

func coerceForceRecords(uncoerced []map[string]interface{}) (records []ForceRecord) {
	records = make([]ForceRecord, len(uncoerced))
	for i, record := range uncoerced {
		records[i] = ForceRecord(record)
	}
	return
}

func columnLengths(records []ForceRecord, prefix string) (lengths map[string]int) {
	lengths = make(map[string]int)

	columns := recordColumns(records)
	for _, column := range columns {
		lengths[fmt.Sprintf("%s.%s", prefix, column)] = len(column) + 2
	}

	for _, record := range records {
		for column, value := range record {
			key := fmt.Sprintf("%s.%s", prefix, column)
			length := 0
			switch value := value.(type) {
			case []ForceRecord:
				lens := columnLengths(value, key)
				for k, l := range lens {
					length += l
					if l > lengths[k] {
						lengths[k] = l
					}
				}
				length += len(lens) - 1
			default:
				if value == nil {
					length = len(" (null) ")
				} else {
					length = len(fmt.Sprintf(" %v ", value))
				}
			}
			if length > lengths[key] {
				lengths[key] = length
			}
		}
	}
	return
}

func recordHeader(columns []string, lengths map[string]int, prefix string) (out string) {
	headers := make([]string, len(columns))
	for i, column := range columns {
		key := fmt.Sprintf("%s.%s", prefix, column)
		headers[i] = fmt.Sprintf(fmt.Sprintf(" %%-%ds ", lengths[key]-2), column)
	}
	out = strings.Join(headers, "|")
	return
}

func recordSeparator(columns []string, lengths map[string]int, prefix string) (out string) {
	separators := make([]string, len(columns))
	for i, column := range columns {
		key := fmt.Sprintf("%s.%s", prefix, column)
		separators[i] = strings.Repeat("-", lengths[key])
	}
	out = strings.Join(separators, "+")
	return
}

func recordRow(record ForceRecord, columns []string, lengths map[string]int, prefix string) (out string) {
	values := make([]string, len(columns))
	for i, column := range columns {
		value := record[column]
		switch value := value.(type) {
		case []ForceRecord:
			values[i] = strings.TrimSuffix(renderForceRecords(value, fmt.Sprintf("%s.%s", prefix, column), lengths), "\n")
		default:
			if value == nil {
				values[i] = fmt.Sprintf(fmt.Sprintf(" %%-%ds ", lengths[column]-2), "(null)")
			} else {
				values[i] = fmt.Sprintf(fmt.Sprintf(" %%-%dv ", lengths[column]-2), value)
			}
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
			key := fmt.Sprintf("%s.%s", prefix, column)
			parts := strings.Split(values[j], "\n")
			if i < len(parts) {
				rowvalues[j] = fmt.Sprintf(fmt.Sprintf("%%-%ds", lengths[key]), parts[i])
			} else {
				rowvalues[j] = strings.Repeat(" ", lengths[key])
			}
		}
		rows[i] = strings.Join(rowvalues, "|")
	}
	out = strings.Join(rows, "\n")
	return
}

<<<<<<< HEAD
func RenderForceRecordsCSV(records []ForceRecord, format string) string {
	var out bytes.Buffer

	var keys []string
	var flattenedRecords []map[string]interface{}
	for _, record := range records {
		flattenedRecord := FlattenForceRecord(record)
		flattenedRecords = append(flattenedRecords, flattenedRecord)
		for key, _ := range flattenedRecord {
			if !StringSliceContains(keys, key) {
				keys = append(keys, key)
			}
		}
	}
	keys = RemoveTransientRelationships(keys)

	if len(records) > 0 {
		lengths := make([]int, len(keys))
		for i, key := range keys {
			lengths[i] = len(key)
		}

		formatter_parts := make([]string, len(keys))
		for i, length := range lengths {
			formatter_parts[i] = fmt.Sprintf(`"%%-%ds"`, length)
		}

		formatter := strings.Join(formatter_parts,`,`)
		out.WriteString(fmt.Sprintf(formatter+"\n", StringSliceToInterfaceSlice(keys)...))
		for _, record := range flattenedRecords {
			values := make([][]string, len(keys))
			for i, key := range keys {
				values[i] = strings.Split(fmt.Sprintf(`%v`, record[key]), `\n`)
			}

			maxLines := 0
			for _, value := range values {
				lines := len(value)
				if lines > maxLines {
					maxLines = lines
				}
			}

			for li := 0; li < maxLines; li++ {
				line := make([]string, len(values))
				for i, value := range values {
					if len(value) > li {
						line[i] = value[li]
					}
				}
				out.WriteString(fmt.Sprintf(formatter+"\n", StringSliceToInterfaceSlice(line)...))
			}
		}
	}
	return out.String()
}

func FlattenForceRecord(record ForceRecord) map[string]interface{} {
	fieldValues := make(map[string]interface{})
	for key, _ := range record {
		value := record[key]
=======
func flattenForceRecord(record ForceRecord) (flattened ForceRecord) {
	flattened = make(ForceRecord)
	for key, value := range record {
>>>>>>> 7b71603a08103adea72f0164428f695558f9a787
		if key == "attributes" {
			continue
		}
		switch value := value.(type) {
		case map[string]interface{}:
			if value["records"] != nil {
				unflattened := value["records"].([]interface{})
				subflattened := make([]ForceRecord, len(unflattened))
				for i, record := range unflattened {
					subflattened[i] = (map[string]interface{})(flattenForceRecord(ForceRecord(record.(map[string]interface{}))))
				}
				flattened[key] = subflattened
			} else {
				for k, v := range flattenForceRecord(value) {
					flattened[fmt.Sprintf("%s.%s", key, k)] = v
				}
			}
		default:
			flattened[key] = value
		}
	}
	return
}

func recordsHaveSubRows(records []ForceRecord) bool {
	for _, record := range records {
		for _, value := range record {
			switch value := value.(type) {
			case []ForceRecord:
				if len(value) > 0 {
					return true
				}
			}
		}
	}
	return false
}

func renderForceRecords(records []ForceRecord, prefix string, lengths map[string]int) string {
	var out bytes.Buffer

	columns := recordColumns(records)

	out.WriteString(recordHeader(columns, lengths, prefix) + "\n")
	out.WriteString(recordSeparator(columns, lengths, prefix) + "\n")

	for _, record := range records {
		out.WriteString(recordRow(record, columns, lengths, prefix) + "\n")
		if recordsHaveSubRows(records) {
			out.WriteString(recordSeparator(columns, lengths, prefix) + "\n")
		}
	}

	return out.String()
}

func RenderForceRecords(records []ForceRecord) string {
	flattened := make([]ForceRecord, len(records))
	for i, record := range records {
		flattened[i] = flattenForceRecord(record)
	}
	lengths := columnLengths(flattened, "")
	return renderForceRecords(flattened, "", lengths)
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
