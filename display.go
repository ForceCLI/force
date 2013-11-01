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

func RenderForceRecords(records []ForceRecord) string {
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
		separators := make([]string, len(keys))
		for i, key := range keys {
			lengths[i] = len(key)
			for _, record := range flattenedRecords {
				v := fmt.Sprintf("%v", record[key])
				l := len(v)
				if index := strings.Index(v, "\n"); index > -1 {
					l = index + 1
				}
				if l > lengths[i] {
					lengths[i] = l
				}
			}
			separators[i] = strings.Repeat("-", lengths[i]+2)
		}
		formatter_parts := make([]string, len(keys))
		for i, length := range lengths {
			formatter_parts[i] = fmt.Sprintf(" %%-%ds ", length)
		}
		formatter := strings.Join(formatter_parts, "|")
		out.WriteString(fmt.Sprintf(formatter+"\n", StringSliceToInterfaceSlice(keys)...))
		out.WriteString(fmt.Sprintf(strings.Join(separators, "+") + "\n"))
		for _, record := range flattenedRecords {
			values := make([][]string, len(keys))
			for i, key := range keys {
				values[i] = strings.Split(fmt.Sprintf("%v", record[key]), "\n")
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
		out.WriteString(fmt.Sprintf(strings.Join(separators, "+") + "\n"))
	}
	out.WriteString(fmt.Sprintf(" (%d records)\n", len(records)))
	return out.String()
}

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
		if key == "attributes" {
			continue
		} else if relationship, isRelationship := value.(map[string]interface{}); isRelationship {
			if _, ok := relationship["records"]; ok {
				fieldValues[key] = RenderForceRecords(ChildRelationshipToQueryResult(relationship).Records)
			} else {
				for parentKey, parentValue := range FlattenForceRecord(relationship) {
					fieldValues[key+"."+parentKey] = parentValue
				}
			}
		} else {
			fieldValues[key] = value
		}
	}
	return fieldValues
}

func ChildRelationshipToQueryResult(relationship map[string]interface{}) ForceQueryResult {
	done := relationship["done"].(bool)
	var records []ForceRecord
	for _, cr := range relationship["records"].([]interface{}) {
		records = append(records, ForceRecord(cr.(map[string]interface{})))
	}
	totalSize := int(relationship["totalSize"].(float64))
	return ForceQueryResult{done, records, totalSize}
}

func StringSliceContains(slice []string, e string) bool {
	for _, s := range slice {
		if s == e {
			return true
		}
	}
	return false
}

func RemoveTransientRelationships(slice []string) []string {
	var transientRelationships []string
	for _, s1 := range slice {
		for _, s2 := range slice {
			if strings.HasPrefix(s1, s2+".") {
				transientRelationships = append(transientRelationships, s2)
			}
		}
	}

	var flattened []string
	for _, s := range slice {
		if !StringSliceContains(transientRelationships, s) {
			flattened = append(flattened, s)
		}
	}
	return flattened
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
