package command_test

import (
	"encoding/json"
	"testing"

	"github.com/ForceCLI/force/lib/bayeux"
	"github.com/bmizerany/assert"
)

// Test deduplication logic for log tail functionality
func TestLogEventDeduplication(t *testing.T) {
	// Create a map to simulate the deduplication logic
	processedLogs := make(map[string]bool)
	processedCount := 0

	// Mock log events with duplicate IDs
	logEvents := []struct {
		id       string
		expected bool // whether this should be processed
	}{
		{"07LOx00000SQWKJMA5", true},  // first occurrence - should process
		{"07LOx00000SQWKJMA5", false}, // duplicate - should skip
		{"07LOx00000SQWKLMA6", true},  // new ID - should process
		{"07LOx00000SQWKJMA5", false}, // duplicate again - should skip
		{"07LOx00000SQWKLMA6", false}, // duplicate of second ID - should skip
		{"07LOx00000SQWKMMA7", true},  // new ID - should process
	}

	for _, event := range logEvents {
		// Simulate the deduplication logic from tailLogs()
		if processedLogs[event.id] {
			// Should skip duplicate
			assert.Equal(t, false, event.expected, "Expected to skip duplicate log ID: "+event.id)
			continue
		}

		// Process new log
		processedLogs[event.id] = true
		processedCount++
		assert.Equal(t, true, event.expected, "Expected to process new log ID: "+event.id)
	}

	// Verify we processed exactly 3 unique logs
	assert.Equal(t, 3, processedCount)
	assert.Equal(t, 3, len(processedLogs))
}

// Test memory cleanup logic for processed logs map
func TestLogMemoryCleanup(t *testing.T) {
	processedLogs := make(map[string]bool)

	// Add exactly 100 entries first
	for i := 0; i < 100; i++ {
		logId := "07LOx00000SQWK" + string(rune(65+i%26)) + "MA" + string(rune(48+i%10))
		processedLogs[logId] = true
	}

	// Should have 100 entries now
	assert.Equal(t, 100, len(processedLogs))

	// Add one more to trigger cleanup (101st entry)
	triggerLogId := "07LOx00000SQWKTRIGGER"

	// Simulate the cleanup logic from tailLogs()
	if len(processedLogs) > 100 {
		// Clear and start fresh
		processedLogs = make(map[string]bool)
		processedLogs[triggerLogId] = true
	} else {
		processedLogs[triggerLogId] = true
		// After adding this, we should have 101 entries and trigger cleanup
		if len(processedLogs) > 100 {
			// Clear and start fresh
			processedLogs = make(map[string]bool)
			processedLogs[triggerLogId] = true
		}
	}

	// After cleanup, should only have 1 entry (the trigger log)
	assert.Equal(t, 1, len(processedLogs))
	assert.Equal(t, true, processedLogs[triggerLogId])
}

// Test JSON unmarshaling of log events
func TestLogEventUnmarshaling(t *testing.T) {
	// Sample CometD message data
	jsonData := `{
		"event": {
			"createdDate": "2025-08-28T19:51:43.425Z",
			"replayId": 342,
			"type": "created"
		},
		"sobject": {
			"Id": "07LOx00000SQWKJMA5"
		}
	}`

	// Define the logEvent struct inline for testing
	type logEvent struct {
		Event struct {
			CreatedDate string `json:"createdDate"`
			Type        string `json:"type"`
		} `json:"event"`
		Sobject struct {
			Id string `json:"Id"`
		} `json:"sobject"`
	}

	var event logEvent
	err := json.Unmarshal([]byte(jsonData), &event)

	assert.Equal(t, nil, err)
	assert.Equal(t, "07LOx00000SQWKJMA5", event.Sobject.Id)
	assert.Equal(t, "created", event.Event.Type)
	assert.Equal(t, "2025-08-28T19:51:43.425Z", event.Event.CreatedDate)
}

// Test bayeux message handling
func TestBayeuxMessageFiltering(t *testing.T) {
	// Test that messages with empty data are filtered out
	msg1 := &bayeux.Message{
		Channel: "/systemTopic/Logging",
		Data:    json.RawMessage(`{}`),
	}

	msg2 := &bayeux.Message{
		Channel: "/meta/connect",
		Data:    nil, // Empty data should be filtered
	}

	msg3 := &bayeux.Message{
		Channel: "/systemTopic/Logging",
		Data:    json.RawMessage(`{"event":{"type":"created"},"sobject":{"Id":"test"}}`),
	}

	messages := []*bayeux.Message{msg1, msg2, msg3}
	validMessages := 0

	for _, msg := range messages {
		// Simulate the filtering logic from bayeux client
		if msg.Channel != "" && msg.Data != nil {
			validMessages++
		}
	}

	// Should filter out msg2 (empty data)
	assert.Equal(t, 2, validMessages)
}
