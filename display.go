package main

import (
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
				l := len(fmt.Sprintf("%v", record[key]))
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
		fmt.Printf(formatter+"\n", StringSliceToInterfaceSlice(keys)...)
		fmt.Printf(strings.Join(separators, "+") + "\n")
		for _, record := range flattenedRecords {
			values := make([]string, len(keys))
			for i, key := range keys {
				values[i] = fmt.Sprintf("%v", record[key])
			}
			fmt.Printf(formatter+"\n", StringSliceToInterfaceSlice(values)...)
		}
		fmt.Printf(strings.Join(separators, "+") + "\n")
	}
	fmt.Printf(" (%d records)\n", len(records))
}

func FlattenForceRecord(record ForceRecord) map[string]interface{} {
	fieldValues := make(map[string]interface{})
	for key, _ := range record {
		value := record[key]
		if key == "attributes" {
			continue
		} else if relationship, isRelationship := value.(map[string]interface{}); isRelationship {
			for subKey, subValue := range FlattenForceRecord(relationship) {
				fieldValues[key+"."+subKey] = subValue
			}
		} else {
			fieldValues[key] = value
		}
	}
	return fieldValues
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
