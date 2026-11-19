package sf

import (
	"bytes"
	"encoding/json"
)

// orderedMap is a JSON object whose keys are written in insertion order.
type orderedMap struct {
	keys   []string
	values map[string]interface{}
}

func newOrderedMap() *orderedMap {
	return &orderedMap{values: map[string]interface{}{}}
}

// Set stores a value.  A new key is added after the existing keys.
func (m *orderedMap) Set(key string, value interface{}) {
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

func (m *orderedMap) Get(key string) (interface{}, bool) {
	value, ok := m.values[key]
	return value, ok
}

func (m *orderedMap) Delete(key string) {
	if _, ok := m.values[key]; !ok {
		return
	}
	delete(m.values, key)
	for i, existing := range m.keys {
		if existing == key {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			return
		}
	}
}

func (m *orderedMap) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, key := range m.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		encodedKey, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		b.Write(encodedKey)
		b.WriteByte(':')
		var value bytes.Buffer
		enc := json.NewEncoder(&value)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(m.values[key]); err != nil {
			return nil, err
		}
		b.Write(bytes.TrimRight(value.Bytes(), "\n"))
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
