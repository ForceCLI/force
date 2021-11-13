package internal

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"reflect"
)

type Unmarshaler func([]byte, interface{}) error
type Marsaler func(interface{}) ([]byte, error)

// JsonUnmarshal unmarshals a json body and errors with a nicer error.
func JsonUnmarshal(b []byte, ptr interface{}) error {
	err := json.Unmarshal(b, ptr)
	if err == nil {
		return nil
	}
	return fmt.Errorf("error unmarshaling json: %s. first 10 characters: %s", err.Error(), slice(string(b), 10))
}

// XmlUnmarshal unmarshals an xml body and errors with a nicer error.
func XmlUnmarshal(b []byte, ptr interface{}) error {
	err := xml.Unmarshal(b, ptr)
	if err == nil {
		return nil
	}
	return fmt.Errorf("error unmarshaling xml: %s. first 10 characters: %s", err.Error(), slice(string(b), 10))
}

func XmlMarshal(o interface{}) ([]byte, error) {
	b, err := xml.Marshal(o)
	if err == nil {
		return b, nil
	}
	objsummary := fmt.Sprintf("%s(%s...)", reflect.TypeOf(o).Name(), slice(fmt.Sprintf("%v", o), 10))
	return nil, fmt.Errorf("error marshaling xml: %s. object summary: %s", err.Error(), objsummary)
}

func slice(s string, length int) string {
	if len(s) > length {
		return s[:length]
	}
	return s
}
