package main

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

var BatchInfoTemplate = `
Id 			%s
JobId 			%s
State 			%s
CreatedDate 		%s
SystemModstamp 		%s
NumberRecordsProcessed  %d
`
func DisplayBatchList(batchInfos []BatchInfo) {
	
	for i, batchInfo := range batchInfos {
		fmt.Printf("Batch %d", i)
		DisplayBatchInfo(batchInfo)
		fmt.Println()
	}
}

func DisplayBatchInfo(batchInfo BatchInfo) {
	
	fmt.Printf(BatchInfoTemplate, batchInfo.Id, batchInfo.JobId, batchInfo.State,
				batchInfo.CreatedDate, batchInfo.SystemModstamp, 
				batchInfo.NumberRecordsProcessed)
}

func DisplayJobInfo(jobInfo JobInfo) {
	var msg = `
Id				%s
State 				%s
Operation			%s
Object 				%s
Api Version 			%s

Created By Id 			%s
Created Date 			%s
System Mod Stamp		%s
Content Type 			%s
Concurrency Mode 		%s

Number Batches Queued 		%d
Number Batches In Progress	%d
Number Batches Completed 	%d
Number Batches Failed 		%d
Number Batches Total 		%d
Number Records Processed 	%d
Number Retries 			%d

Number Records Failed 		%d
Total Processing Time 		%d
Api Active Processing Time 	%d
Apex Processing Time 		%d
`
	fmt.Printf(msg, jobInfo.Id, jobInfo.State, jobInfo.Operation, jobInfo.Object, jobInfo.ApiVersion,
				jobInfo.CreatedById, jobInfo.CreatedDate, jobInfo.SystemModStamp, 
				jobInfo.ContentType, jobInfo.ConcurrencyMode,
				jobInfo.NumberBatchesQueued, jobInfo.NumberBatchesInProgress,
				jobInfo.NumberBatchesCompleted, jobInfo.NumberBatchesFailed,
				jobInfo.NumberBatchesTotal, jobInfo.NumberRecordsProcessed,
				jobInfo.NumberRetries, 
				jobInfo.NumberRecordsFailed, jobInfo.TotalProcessingTime,
				jobInfo.ApiActiveProcessingTime, jobInfo.ApexProcessingTime)
}

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

func DisplayForceRecords(result ForceQueryResult) {
	if len(result.Records) > 0 {
		fmt.Print(RenderForceRecords(result.Records))
	}
	fmt.Println(fmt.Sprintf(" (%d records)", result.TotalSize))
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

func flattenForceRecord(record ForceRecord) (flattened ForceRecord) {
	flattened = make(ForceRecord)
	for key, value := range record {
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
