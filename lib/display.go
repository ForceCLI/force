package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	. "github.com/ForceCLI/force/error"
)

var BatchInfoTemplate = `
Id 			%s
JobId 			%s
State 			%s
StateMessage		%s
CreatedDate 		%s
SystemModstamp 		%s
NumberRecordsProcessed  %d
`

type NullValue struct{}

var (
	nullFieldValue = NullValue{}
)

type ByXmlName []DescribeMetadataObject

func (a ByXmlName) Len() int           { return len(a) }
func (a ByXmlName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByXmlName) Less(i, j int) bool { return a[i].XmlName < a[j].XmlName }

type ByFullName []MDFileProperties

func (a ByFullName) Len() int           { return len(a) }
func (a ByFullName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByFullName) Less(i, j int) bool { return a[i].FullName < a[j].FullName }

func DisplayListMetadataResponse(resp ListMetadataResponse) {
	sort.Sort(ByFullName(resp.Result))
	for _, result := range resp.Result {
		fmt.Println(result.FullName + " - " + result.Type)
	}
}

func DisplayListMetadataResponseJson(resp ListMetadataResponse) {
	sort.Sort(ByFullName(resp.Result))
	b, err := json.MarshalIndent(resp.Result, "", "   ")
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("%s\n", string(b))
}

func DisplayMetadataList(metadataObjects []DescribeMetadataObject) {

	sort.Sort(ByXmlName(metadataObjects))

	for _, obj := range metadataObjects {
		fmt.Printf("%s ==> %s\n", obj.XmlName, obj.DirectoryName)
		if len(obj.ChildXmlNames) > 0 {
			sort.Strings(obj.ChildXmlNames)
			for _, child := range obj.ChildXmlNames {
				fmt.Printf("\t%s\n", child)
			}
		}
	}
}

func DisplayMetadataListJson(metadataObjects []DescribeMetadataObject) {

	sort.Sort(ByXmlName(metadataObjects))

	for _, obj := range metadataObjects {
		if len(obj.ChildXmlNames) > 0 {
			sort.Strings(obj.ChildXmlNames)
		}
	}

	b, err := json.MarshalIndent(metadataObjects, "", "   ")
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("%s\n", string(b))
}

func DisplayBatchList(batchInfos []BatchInfo) {
	for i, batchInfo := range batchInfos {
		fmt.Printf("Batch %d", i)
		DisplayBatchInfo(batchInfo, os.Stdout)
		fmt.Println()
	}
}

func DisplayBatchInfo(batchInfo BatchInfo, w io.Writer) {
	fmt.Fprintf(w, BatchInfoTemplate, batchInfo.Id, batchInfo.JobId, batchInfo.State, batchInfo.StateMessage,
		batchInfo.CreatedDate, batchInfo.SystemModstamp,
		batchInfo.NumberRecordsProcessed)
}

func DisplayJobInfo(jobInfo JobInfo, w io.Writer) {
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
	fmt.Fprintf(w, msg, jobInfo.Id, jobInfo.State, jobInfo.Operation, jobInfo.Object, jobInfo.ApiVersion,
		jobInfo.CreatedById, jobInfo.CreatedDate, jobInfo.SystemModStamp,
		jobInfo.ContentType, jobInfo.ConcurrencyMode,
		jobInfo.NumberBatchesQueued, jobInfo.NumberBatchesInProgress,
		jobInfo.NumberBatchesCompleted, jobInfo.NumberBatchesFailed,
		jobInfo.NumberBatchesTotal, jobInfo.NumberRecordsProcessed,
		jobInfo.NumberRetries,
		jobInfo.NumberRecordsFailed, jobInfo.TotalProcessingTime,
		jobInfo.ApiActiveProcessingTime, jobInfo.ApexProcessingTime)
}

func DisplayForceSobjectDescribe(sobject string) {
	var d interface{}
	b := []byte(sobject)
	err := json.Unmarshal(b, &d)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	out, err := json.MarshalIndent(d, "", "    ")
	fmt.Println(string(out))
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

func DisplayForceSobjectsJson(sobjects []ForceSobject) {
	names := make([]string, len(sobjects))
	for i, sobject := range sobjects {
		names[i] = sobject["name"].(string)
	}
	b, err := json.MarshalIndent(names, "", "   ")
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("%s\n", string(b))
}

func (f *Force) DisplayAllForceRecordsf(result ForceQueryResult, format string) {
	currentResult := result
	var err error
	records := make(chan ForceRecord)
	done := make(chan bool)
	go DisplayForceRecordsf(records, format, done)
	for {
		for _, record := range currentResult.Records {
			records <- record
		}
		if currentResult.Done {
			close(records)
			<-done
			return
		}
		currentResult, err = f.getForceResult(currentResult.NextRecordsUrl)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
}

func DisplayForceRecordsf(records <-chan ForceRecord, format string, done chan<- bool) {
	switch format {
	case "csv":
		RenderForceRecordsCSV(records, done)
	case "json":
		for record := range records {
			recs, _ := json.Marshal(record)
			fmt.Println(string(recs))
		}
		done <- true
	case "json-pretty":
		for record := range records {
			recs, _ := json.MarshalIndent(record, "", "  ")
			fmt.Println(string(recs))
		}
		done <- true
	default:
		fmt.Printf("Format %s not supported\n\n", format)
		done <- true
	}
}

func (f *Force) DisplayAllForceRecords(result ForceQueryResult) {
	currentResult := result
	var err error
	for {
		DisplayForceRecords(currentResult)
		if currentResult.Done {
			return
		}
		currentResult, err = f.getForceResult(currentResult.NextRecordsUrl)
		if err != nil {
			ErrorAndExit(err.Error())
		}
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
		var keys []string
		for key, _ := range record {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
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
			switch value.(type) {
			case NullValue:
				values[i] = fmt.Sprintf(fmt.Sprintf(" %%-%ds ", lengths[column]-2), "(null)")
			case nil:
				values[i] = fmt.Sprintf(fmt.Sprintf(" %%-%ds ", lengths[column]-2), "")
			default:
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

// returns first index of a given string
func StringSlicePos(slice []string, value string) int {
	for p, v := range slice {
		if v == value {
			return p
		}
	}
	return -1
}

// returns true if a slice contains given string
func StringSliceContains(slice []string, value string) bool {
	return StringSlicePos(slice, value) > -1
}

func recordKeys(record ForceRecord) []string {
	var keys []string
	for key, _ := range record {
		if !StringSliceContains(keys, key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func RenderForceRecordsCSV(records <-chan ForceRecord, done chan<- bool) {
	var keys []string

	firstRow := true

	for record := range records {
		flattenedRecord := flattenForceRecord(record)
		if firstRow {
			keys = recordKeys(flattenedRecord)
			os.Stdout.WriteString(fmt.Sprintf(`"%s"%s`, strings.Join(keys, `","`), "\n"))
			firstRow = false
		}
		myvalues := make([]string, len(keys))
		for i, key := range keys {
			var value string
			switch v := flattenedRecord[key].(type) {
			case NullValue:
				value = ""
			default:
				value = fmt.Sprintf(`%v`, v)
				value = strings.Replace(value, "<nil>", "", -1)
				value = strings.Replace(value, `"`, `""`, -1)
			}
			myvalues[i] = value
		}
		os.Stdout.WriteString(fmt.Sprintf(`"%s"%s`, strings.Join(myvalues, `","`), "\n"))
	}
	done <- true
}

func flattenForceRecord(record ForceRecord) (flattened ForceRecord) {
	flattened = make(ForceRecord)
	for key, value := range record {
		if key == "attributes" {
			continue
		}
		if value == nil {
			flattened[key] = nullFieldValue
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
		case "picklist", "multipicklist":
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

func DisplayFieldTypes() {
	var msg = `
	FIELD									 DEFAULTS
	=========================================================================
  text/string            (length = 255)
  textarea               (length = 255)
  longtextarea           (length = 32768, visibleLines = 5)
  richtextarea           (length = 32768, visibleLines = 5)
  checkbox/bool/boolean  (defaultValue = false)
  datetime               ()
  email                  ()
  url                    ()
  float/double/currency  (precision = 16, scale = 2)
  number/int             (precision = 18, scale = 0)
  autonumber             (displayFormat = "AN {00000}", startingNumber = 0)
  geolocation            (displayLocationInDecimal = true, scale = 5)
  lookup                 (will be prompted for Object and label)
  masterdetail           (will be prompted for Object and label)
  picklist               ()

  *To create a formula field add a formula argument to the command.
  force field create <objectname> <fieldName>:text formula:'LOWER("HEY MAN")'
  `
	fmt.Println(msg)
}

func DisplayFieldDetails(fieldType string) {
	var msg = ``
	switch fieldType {
	case "picklist":
		msg = DisplayPicklistFieldDetails()
		break
	case "text", "string":
		msg = DisplayTextFieldDetails()
		break
	case "textarea":
		msg = DisplayTextAreaFieldDetails()
		break
	case "longtextarea":
		msg = DisplayLongTextAreaFieldDetails()
		break
	case "richtextarea":
		msg = DisplayRichTextAreaFieldDetails()
		break
	case "checkbox", "bool", "boolean":
		msg = DisplayCheckboxFieldDetails()
		break
	case "datetime":
		msg = DisplayDatetimeFieldDetails()
		break
	case "float", "double", "currency":
		if fieldType == "currency" {
			msg = DisplayCurrencyFieldDetails()
		} else {
			msg = DisplayDoubleFieldDetails()
		}
		break
	case "number", "int":
		msg = DisplayDoubleFieldDetails()
		break
	case "autonumber":
		msg = DisplayAutonumberFieldDetails()
		break
	case "geolocation":
		msg = DisplayGeolocationFieldDetails()
		break
	case "lookup":
		msg = DisplayLookupFieldDetails()
		break
	case "masterdetail":
		msg = DisplayMasterDetailFieldDetails()
		break
	default:
		msg = `
  Sorry, that is not a valid field type.
`
	}
	fmt.Println(msg)
}

func DisplayTextFieldDetails() (message string) {
	return fmt.Sprintf(`
  Allows users to enter any combination of letters and numbers.

    %s
      label            - defaults to name
      length           - defaults to 255
      name

    %s
      description
      helptext
      required         - defaults to false
      unique           - defaults to false
      caseSensistive   - defaults to false
      externalId       - defaults to false
      defaultValue
      formula          - defaultValue must be blask
      formulaTreatBlanksAs  - defaults to "BlankAsZero"
`, "\x1b[31;1mrequired attributes\x1b[0m", "\x1b[31;1moptional attributes\x1b[0m")
}

func DisplayPicklistFieldDetails() (message string) {
	return fmt.Sprintf(`
   List of options to coose from

    %s
     label            - defaults to name
     name

    %s
     description
     helptext
     required         - defaults to false
     defaultValue
     picklist         - comma separated list of values
    `, "\x1b[31;1mrequired attributes\x1b[0m", "\x1b[31;1moptional attributes\x1b[0m")
}

func DisplayTextAreaFieldDetails() (message string) {
	return fmt.Sprintf(`
  Allows users to enter up to 255 characters on separate lines.

    %s
      label            - defaults to name
      name

    %s
      description
      helptext
      required         - defaults to false
      defaultValue
`, "\x1b[31;1mrequired attributes\x1b[0m", "\x1b[31;1moptional attributes\x1b[0m")
}
func DisplayLongTextAreaFieldDetails() (message string) {
	return fmt.Sprintf(`
  Allows users to enter up to 32,768 characters on separate lines.

    %s
      label            - defaults to name
      length           - defaults to 32,768
      name
      visibleLines     - defaults to 3

    %s
      description
      helptext
      defaultValue
`, "\x1b[31;1mrequired attributes\x1b[0m", "\x1b[31;1moptional attributes\x1b[0m")
}
func DisplayRichTextAreaFieldDetails() (message string) {
	return fmt.Sprintf(`
  Allows users to enter formatted text, add images and links. Up to 32,768 characters on separate lines.

    %s
      label            - defaults to name
      length           - defaults to 32,768
      name
      visibleLines     - defaults to 25

    %s
      description
      helptext
`, "\x1b[31;1mrequired attributes\x1b[0m", "\x1b[31;1moptional attributes\x1b[0m")
}
func DisplayCheckboxFieldDetails() (message string) {
	return fmt.Sprintf(`
  Allows users to select a True (checked) or False (unchecked) value.

    %s
      label            - defaults to name
      name

    %s
      description
      helptext
      defaultValue     - defaults to unchecked or false
      formula          - defaultValue must be blask
      formulaTreatBlanksAs  - defaults to "BlankAsZero"
`, "\x1b[31;1mrequired attributes\x1b[0m", "\x1b[31;1moptional attributes\x1b[0m")
}
func DisplayDatetimeFieldDetails() (message string) {
	return fmt.Sprintf(`
  Allows users to enter a date and time.

    %s
      label            - defaults to name
      name

    %s
      description
      helptext
      defaultValue
      required         - defaults to false
      formula          - defaultValue must be blask
      formulaTreatBlanksAs  - defaults to "BlankAsZero"
`, "\x1b[31;1mrequired attributes\x1b[0m", "\x1b[31;1moptional attributes\x1b[0m")
}
func DisplayDoubleFieldDetails() (message string) {
	return fmt.Sprintf(`
  Allows users to enter any number. Leading zeros are removed.

    %s
      label            - defaults to name
      name
      precision        - digits left of decimal (defaults to 18)
      scale            - decimal places (defaults to 0)

    %s
      description
      helptext
      required         - defaults to false
      unique           - defaults to false
      externalId       - defaults to false
      defaultValue
      formula          - defaultValue must be blask
      formulaTreatBlanksAs  - defaults to "BlankAsZero"
`, "\x1b[31;1mrequired attributes\x1b[0m", "\x1b[31;1moptional attributes\x1b[0m")
}
func DisplayCurrencyFieldDetails() (message string) {
	return fmt.Sprintf(`
  Allows users to enter a dollar or other currency amount and automatically formats the field as a currency amount.

    %s
      label            - defaults to name
      name
      precision        - digits left of decimal (defaults to 18)
      scale            - decimal places (defaults to 0)

    %s
      description
      helptext
      required         - defaults to false
      defaultValue
      formula          - defaultValue must be blask
      formulaTreatBlanksAs  - defaults to "BlankAsZero"
`, "\x1b[31;1mrequired attributes\x1b[0m", "\x1b[31;1moptional attributes\x1b[0m")
}
func DisplayAutonumberFieldDetails() (message string) {
	return fmt.Sprintf(`
  A system-generated sequence number that uses a display format you define. The number is automatically incremented for each new record.

    %s
      label            - defaults to name
      name
      displayFormat    - defaults to "AN-{00000}"
      startingNumber   - defaults to 0

    %s
      description
      helptext
      externalId       - defaults to false
`, "\x1b[31;1mrequired attributes\x1b[0m", "\x1b[31;1moptional attributes\x1b[0m")
}
func DisplayGeolocationFieldDetails() (message string) {
	return fmt.Sprintf(`
   Allows users to define locations.

    %s
      label                       - defaults to name
      name
      DisplayLocationInDecimal    - defaults false
      scale                       - defaults to 5 (number of decimals to the right)

    %s
      description
      helptext
      required                    - defaults to false
`, "\x1b[31;1mrequired attributes\x1b[0m", "\x1b[31;1moptional attributes\x1b[0m")
}
func DisplayLookupFieldDetails() (message string) {
	return fmt.Sprintf(`
   Creates a relationship that links this object to another object.

    %s
      label            - defaults to name
      name
      referenceTo      - Name of related object
      relationshipName - defaults to referenceTo value

    %s
      description
      helptext
      required         - defaults to false
      relationShipLabel
`, "\x1b[31;1mrequired attributes\x1b[0m", "\x1b[31;1moptional attributes\x1b[0m")
}
func DisplayMasterDetailFieldDetails() (message string) {
	return fmt.Sprintf(`
   Creates a special type of parent-child relationship between this object (the child, or "detail") and another object (the parent, or "master") where:
     The relationship field is required on all detail records.
     The ownership and sharing of a detail record are determined by the master record.
     When a user deletes the master record, all detail records are deleted.
     You can create rollup summary fields on the master record to summarize the detail records.

    %s
      label            - defaults to name
      name
      referenceTo      - Name of related object
      relationshipName - defaults to referenceTo value

    %s
      description
      helptext
      required         - defaults to false
      relationShipLabel
`, "\x1b[31;1mrequired attributes\x1b[0m", "\x1b[31;1moptional attributes\x1b[0m")
}
