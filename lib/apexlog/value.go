package apexlog

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// jsonChildren returns the entries of a JSON object or array value in the
// order they appear. Object entries are named by key and array entries by
// index, e.g. "[0]". It returns false if the value is not a JSON object or
// array.
func jsonChildren(value string) ([]*Variable, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "{") && !strings.HasPrefix(value, "[") {
		return nil, false
	}
	dec := json.NewDecoder(strings.NewReader(value))
	dec.UseNumber()
	open, err := dec.Token()
	if err != nil {
		return nil, false
	}
	var children []*Variable
	index := 0
	for dec.More() {
		name := "[" + strconv.Itoa(index) + "]"
		if open == json.Delim('{') {
			key, err := dec.Token()
			if err != nil {
				return nil, false
			}
			k, ok := key.(string)
			if !ok {
				return nil, false
			}
			name = k
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, false
		}
		children = append(children, &Variable{Name: name, Value: jsonValue(raw)})
		index++
	}
	return children, true
}

// jsonValue converts a JSON value to the form shown to the user. Strings
// are shown in single quotes, except object references such as
// "0x6ed02744", which the log writes as strings and which are returned
// without quotes. Objects and arrays are kept as compact JSON so their
// entries can be listed.
func jsonValue(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if isRef(s) {
			return s
		}
		return "'" + s + "'"
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err == nil {
		return compact.String()
	}
	return string(raw)
}
