package pubsub

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/boljen/go-bitmap"

	. "github.com/ForceCLI/force/lib"
)

var ErrUnexpectedType = fmt.Errorf("unexpected type")

// ProcessBitMap takes the bitmap-encoded list of changed fields and returns a list of field names.
// See Salesforce pub-sub docs for more details:
// https://developer.salesforce.com/docs/platform/pub-sub-api/guide/event-deserialization-considerations.html
func ProcessBitMap(avroSchema map[string]any, bitmapFields []string) ([]string, error) {
	changedFieldNames := []string{}
	if len(bitmapFields) == 0 {
		return changedFieldNames, nil
	}

	// Replace top field level bitmap with list of fields
	if strings.HasPrefix(bitmapFields[0], "0x") {
		// Convert hex to bytes
		hexStr := bitmapFields[0][2:]
		bytes, err := hex.DecodeString(hexStr)
		if err != nil {
			return nil, fmt.Errorf("decode hex string '%s': %w", hexStr, err)
		}

		// Reverse to little-endian
		reversedBytes := []byte{}
		for i := len(bytes) - 1; i >= 0; i-- {
			reversedBytes = append(reversedBytes, bytes[i])
		}

		bitMap := bitmap.Bitmap(reversedBytes)

		schemaFieldNames, err := getSchemaFieldNames(avroSchema)
		if err != nil {
			return nil, fmt.Errorf("get schema field names: %w", err)
		}

		changedFieldNames = append(
			changedFieldNames,
			getFieldNamesFromBitString(bitMap, schemaFieldNames)...,
		)
		bitmapFields = bitmapFields[1:] // shift off the front
	}

	// There can be other bitmaps present in the message of the form:
	// parentPos-childBitMap
	// If we end up needing that, we can implement it here.
	// Check the example repository for Python or Java examples (no Go examples exist yet).

	return changedFieldNames, nil
}

func getSchemaFieldNames(schema map[string]any) ([]string, error) {
	fields := []string{}
	fieldsSlice, ok := schema["fields"].([]any)
	if !ok {
		return nil, ErrCouldNotConvertType(schema["fields"], []any{})
	}

	for _, fieldObj := range fieldsSlice {
		field, ok := fieldObj.(map[string]any)
		if !ok {
			return nil, ErrCouldNotConvertType(fieldObj, map[string]any{})
		}

		asString, ok := field["name"].(string)
		if !ok {
			return nil, ErrCouldNotConvertType(field["name"], "")
		}
		fields = append(fields, asString)
	}
	return fields, nil
}

func ErrCouldNotConvertType(from any, to any) error {
	return fmt.Errorf("could not convert %T to type %T: %w", from, to, ErrUnexpectedType)
}

func getFieldNamesFromBitString(
	bitMap bitmap.Bitmap,
	// A list of all of the schema field names
	schemaFieldNames []string,
) []string {
	// Find indices of "1" bits
	oneIndices := []int{}
	for i := 0; i < bitMap.Len(); i++ {
		if bitMap.Get(i) {
			oneIndices = append(oneIndices, i)
		}
	}

	// And then pick the schema field names with those indices
	changedFieldNames := []string{}
	for _, index := range oneIndices {
		changedFieldNames = append(changedFieldNames, schemaFieldNames[index])
	}
	return changedFieldNames
}

// Parse Change Data Capture events using the bitmaps in the ChangeEventHeader
// to return the changed fields
func parseBody(body map[string]any, schema map[string]any) map[string]any {
	if body["ChangeEventHeader"] == nil {
		return body
	}
	changeEventHeader, ok := body["ChangeEventHeader"].(map[string]any)
	if !ok {
		Log.Info("Unexpected ChangeEventHeader is not map")
		return body
	}
	parsedBody := make(map[string]any)
	parsedBody["ChangeEventHeader"] = changeEventHeader
	for _, bf := range []string{"changedFields", "diffFields", "nulledFields"} {
		bfs := changeEventHeader[bf].([]any)
		var bitmaps []string
		for _, i := range bfs {
			bitmaps = append(bitmaps, i.(string))
		}

		fields, err := ProcessBitMap(schema, bitmaps)
		if err != nil {
			Log.Info(fmt.Sprintf("failed to process bitmap for %s", bf))
			return body
		}
		for _, f := range fields {
			if body[f] == nil {
				parsedBody[f] = nil
				break
			}
			values := body[f].(map[string]any)
			for _, v := range values {
				parsedBody[f] = v
				break
			}
		}
	}
	return parsedBody
}
