package pubsub

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFlattenAvroUnions(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name: "flattens_string_union",
			input: map[string]interface{}{
				"field1": map[string]interface{}{"string": "value1"},
				"field2": "direct_value",
			},
			expected: map[string]interface{}{
				"field1": "value1",
				"field2": "direct_value",
			},
		},
		{
			name: "flattens_multiple_union_types",
			input: map[string]interface{}{
				"stringField": map[string]interface{}{"string": "test"},
				"intField":    map[string]interface{}{"int": 42},
				"longField":   map[string]interface{}{"long": int64(12345)},
				"floatField":  map[string]interface{}{"float": 3.14},
				"boolField":   map[string]interface{}{"boolean": true},
				"nullField":   map[string]interface{}{"null": nil},
			},
			expected: map[string]interface{}{
				"stringField": "test",
				"intField":    42,
				"longField":   int64(12345),
				"floatField":  3.14,
				"boolField":   true,
				"nullField":   nil,
			},
		},
		{
			name: "handles_nested_objects",
			input: map[string]interface{}{
				"parent": map[string]interface{}{
					"child1": map[string]interface{}{"string": "value1"},
					"child2": map[string]interface{}{
						"grandchild": map[string]interface{}{"int": 100},
					},
				},
			},
			expected: map[string]interface{}{
				"parent": map[string]interface{}{
					"child1": "value1",
					"child2": map[string]interface{}{
						"grandchild": 100,
					},
				},
			},
		},
		{
			name: "handles_arrays_with_unions",
			input: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"string": "item1"},
					map[string]interface{}{"string": "item2"},
					"direct_item",
				},
			},
			expected: map[string]interface{}{
				"items": []interface{}{
					"item1",
					"item2",
					"direct_item",
				},
			},
		},
		{
			name: "preserves_non_union_maps",
			input: map[string]interface{}{
				"regularMap": map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expected: map[string]interface{}{
				"regularMap": map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
		{
			name: "handles_primitive_values",
			input: map[string]interface{}{
				"string": "test",
				"int":    42,
				"float":  3.14,
				"bool":   true,
				"null":   nil,
			},
			expected: map[string]interface{}{
				"string": "test",
				"int":    42,
				"float":  3.14,
				"bool":   true,
				"null":   nil,
			},
		},
		{
			name: "handles_salesforce_event_example",
			input: map[string]interface{}{
				"CreatedById":   "00540000000x7fqAAA",
				"CreatedDate":   1753881296606,
				"Record_Id__c":  map[string]interface{}{"string": "a04Rt000003s65DIAQ"},
				"Queue_Name__c": map[string]interface{}{"string": "My_Queue"},
			},
			expected: map[string]interface{}{
				"CreatedById":   "00540000000x7fqAAA",
				"CreatedDate":   1753881296606,
				"Record_Id__c":  "a04Rt000003s65DIAQ",
				"Queue_Name__c": "My_Queue",
			},
		},
		{
			name: "handles_empty_maps",
			input: map[string]interface{}{
				"emptyMap": map[string]interface{}{},
			},
			expected: map[string]interface{}{
				"emptyMap": map[string]interface{}{},
			},
		},
		{
			name: "handles_map_with_non_union_single_key",
			input: map[string]interface{}{
				"notUnion": map[string]interface{}{"customKey": "value"},
			},
			expected: map[string]interface{}{
				"notUnion": map[string]interface{}{"customKey": "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := flattenAvroUnions(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				expectedJSON, _ := json.MarshalIndent(tt.expected, "", "  ")
				resultJSON, _ := json.MarshalIndent(result, "", "  ")
				t.Errorf("flattenAvroUnions() failed:\nExpected:\n%s\nGot:\n%s", expectedJSON, resultJSON)
			}
		})
	}
}

func TestFlattenAvroUnions_EdgeCases(t *testing.T) {
	t.Run("handles_nil_input", func(t *testing.T) {
		result := flattenAvroUnions(nil)
		if result != nil {
			t.Errorf("Expected nil, got %v", result)
		}
	})

	t.Run("handles_empty_array", func(t *testing.T) {
		input := []interface{}{}
		result := flattenAvroUnions(input)
		if !reflect.DeepEqual(result, []interface{}{}) {
			t.Errorf("Expected empty array, got %v", result)
		}
	})

	t.Run("handles_deeply_nested_unions", func(t *testing.T) {
		input := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": map[string]interface{}{
						"value": map[string]interface{}{"string": "deep_value"},
					},
				},
			},
		}
		expected := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": map[string]interface{}{
						"value": "deep_value",
					},
				},
			},
		}
		result := flattenAvroUnions(input)
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Failed to handle deeply nested unions")
		}
	})
}
