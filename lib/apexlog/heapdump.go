package apexlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// HeapDump is the state of the heap that Salesforce saves when execution
// reaches a checkpoint (ApexExecutionOverlayAction with IsDumpingHeap). It
// is the HeapDump field of an ApexExecutionOverlayResult record. The log
// marks where it was taken with a HEAP_DUMP event that gives the record's
// ID.
type HeapDump struct {
	ClassName string       `json:"className"`
	Namespace string       `json:"namespace"`
	Extents   []HeapExtent `json:"extents"`
}

// HeapExtent holds the objects of one type.
type HeapExtent struct {
	TypeName       string       `json:"typeName"`
	CollectionType *string      `json:"collectionType"`
	Extent         []HeapObject `json:"extent"`
}

// HeapObject is one object in the heap. Symbols are the names of the
// variables that refer to it: local variable names, and Class.name for
// static variables.
type HeapObject struct {
	Address  string    `json:"address"`
	IsStatic bool      `json:"isStatic"`
	Symbols  []string  `json:"symbols"`
	Value    HeapValue `json:"value"`
}

// HeapValue is either a value (a scalar, an object address, or a list of
// HeapValues) or a list of named entries (the fields of an object or the
// entries of a map).
type HeapValue struct {
	Value json.RawMessage `json:"value"`
	Entry []HeapEntry     `json:"entry"`
}

type HeapEntry struct {
	Key   string    `json:"keyDisplayValue"`
	Value HeapValue `json:"value"`
}

// ParseHeapDump parses the HeapDump field of an ApexExecutionOverlayResult.
func ParseHeapDump(raw []byte) (*HeapDump, error) {
	var d HeapDump
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("parsing heap dump: %w", err)
	}
	return &d, nil
}

// HeapDumpIDs returns the IDs of the ApexExecutionOverlayResult records
// named by the log's HEAP_DUMP events, in the order they appear.
func HeapDumpIDs(log *Log) []string {
	var ids []string
	seen := map[string]bool{}
	for _, e := range log.Events {
		if e.Type != "HEAP_DUMP" || len(e.Fields) < 2 {
			continue
		}
		id := e.Fields[1]
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// AddHeapDump makes a heap dump available to the replay. When the replay
// reaches the dump's HEAP_DUMP event, the variables and objects in the dump
// replace the values reconstructed from the log, including the contents of
// collections, which the log does not record when they change.
func (s *Session) AddHeapDump(id string, dump *HeapDump) {
	if s.heapDumps == nil {
		s.heapDumps = map[string]*HeapDump{}
	}
	s.heapDumps[id] = dump
}

// HeapDumpCount returns the number of heap dumps added to the replay.
func (s *Session) HeapDumpCount() int {
	return len(s.heapDumps)
}

// AtHeapDump reports whether a heap dump was taken before the current
// statement, so that its variables show the dump's contents.
func (s *Session) AtHeapDump() bool {
	return s.atHeapDump
}

// ObjectType returns the type of an object from a heap dump, or "" if the
// object is not from a heap dump.
func (s *Session) ObjectType(ref string) string {
	return s.objectTypes[ref]
}

// applyHeapDump replaces the replayed state with the contents of a heap
// dump taken before the statement on line runs.
func (s *Session) applyHeapDump(d *HeapDump, line int) {
	scalars := map[string]string{}
	for _, extent := range d.Extents {
		for _, o := range extent.Extent {
			if len(o.Value.Entry) == 0 && !isJSONArray(o.Value.Value) {
				scalars[o.Address] = heapScalar(o.Value.Value)
			}
		}
	}
	resolve := func(v HeapValue) (string, string) {
		if len(v.Entry) > 0 || isJSONArray(v.Value) {
			return "", ""
		}
		value := heapScalar(v.Value)
		if isRef(value) {
			if scalar, ok := scalars[value]; ok {
				return scalar, ""
			}
			return value, value
		}
		return value, ""
	}

	for _, extent := range d.Extents {
		for _, o := range extent.Extent {
			var entries []HeapEntry
			switch {
			case len(o.Value.Entry) > 0:
				entries = o.Value.Entry
			case isJSONArray(o.Value.Value):
				var elements []HeapValue
				if err := json.Unmarshal(o.Value.Value, &elements); err != nil {
					continue
				}
				for i, el := range elements {
					entries = append(entries, HeapEntry{Key: "[" + strconv.Itoa(i) + "]", Value: el})
				}
			default:
				continue
			}
			scope := newScope()
			for _, entry := range entries {
				value, ref := resolve(entry.Value)
				scope.assign(entry.Key, value, ref, line)
			}
			s.objects[o.Address] = scope
			s.objectTypes[o.Address] = extent.TypeName
		}
	}

	top := s.Top()
	for _, extent := range d.Extents {
		for _, o := range extent.Extent {
			value, ref := scalars[o.Address], ""
			if _, ok := s.objects[o.Address]; ok {
				value, ref = o.Address, o.Address
			}
			for _, symbol := range o.Symbols {
				class, name := splitLast(symbol)
				switch {
				case class != "":
					s.staticScope(class).declare(name, extent.TypeName)
					s.staticScope(class).assign(name, value, ref, line)
				case top != nil:
					v := top.Locals.declare(name, extent.TypeName)
					if v.Type == "" {
						v.Type = extent.TypeName
					}
					top.Locals.assign(name, value, ref, line)
				}
			}
		}
	}
}

func isJSONArray(raw json.RawMessage) bool {
	return strings.HasPrefix(strings.TrimSpace(string(raw)), "[")
}

// heapScalar formats a scalar heap value as the log formats values:
// strings in single quotes, object addresses without quotes.
func heapScalar(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return "null"
	}
	return jsonValue(raw)
}
