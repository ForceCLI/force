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
	if len(records) > 1 {
		for key, _ := range records[0] {
			if key != "attributes" {
				keys = append(keys, key)
			}
		}
		lengths := make([]int, len(keys))
		separators := make([]string, len(keys))
		for i, key := range keys {
			lengths[i] = 0
			for _, record := range records {
				l := len(record[key].(string))
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
		for _, record := range records {
			values := make([]string, len(keys))
			for i, key := range keys {
				values[i] = record[key].(string)
			}
			fmt.Printf(formatter+"\n", StringSliceToInterfaceSlice(values)...)
		}
		fmt.Printf(strings.Join(separators, "+") + "\n")
	}
	fmt.Printf("%d results returned\n", len(records))
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
