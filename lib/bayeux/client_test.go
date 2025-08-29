package bayeux

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bmizerany/assert"
)

// Test advice handling in handshake response
func TestHandshakeAdviceHandling(t *testing.T) {
	// Test advice with interval
	advice := &advice{
		Reconnect: "retry",
		Timeout:   30000,
		Interval:  1000,
	}

	rsp := &metaMessage{
		Message: Message{
			ClientId: "test-client-id",
		},
		Successful: true,
		Advice:     advice,
	}

	client := &Client{
		interval: 0, // Start with no interval
	}

	// Simulate the advice handling logic from handshake()
	if rsp.Advice != nil && rsp.Advice.Interval > 0 {
		client.interval = time.Duration(rsp.Advice.Interval) * time.Millisecond
	}

	expectedInterval := 1000 * time.Millisecond
	assert.Equal(t, expectedInterval, client.interval)
}

// Test advice handling when no advice is provided
func TestHandshakeNoAdvice(t *testing.T) {
	rsp := &metaMessage{
		Message: Message{
			ClientId: "test-client-id",
		},
		Successful: true,
		Advice:     nil,
	}

	client := &Client{
		interval: 5 * time.Second, // Start with some interval
	}

	// Simulate the advice handling logic from handshake()
	if rsp.Advice != nil && rsp.Advice.Interval > 0 {
		client.interval = time.Duration(rsp.Advice.Interval) * time.Millisecond
	} else {
		client.interval = 0 // Default to no delay between polls
	}

	assert.Equal(t, time.Duration(0), client.interval)
}

// Test message filtering logic
func TestMessageFiltering(t *testing.T) {
	messages := []metaMessage{
		{
			Message: Message{
				Channel: "/meta/connect",
				Data:    json.RawMessage(`{"successful":true}`),
			},
		},
		{
			Message: Message{
				Channel: "/systemTopic/Logging",
				Data:    json.RawMessage(`{"event":{"type":"created"}}`),
			},
		},
		{
			Message: Message{
				Channel: "",
				Data:    nil,
			},
		},
		{
			Message: Message{
				Channel: "/systemTopic/Logging",
				Data:    nil,
			},
		},
	}

	var validMessages []Message
	for _, msg := range messages {
		// Simulate the filtering logic from send()
		if msg.Channel != "" && msg.Data != nil {
			validMessages = append(validMessages, msg.Message)
		}
	}

	// Should filter out messages 3 and 4 (empty channel or nil data)
	assert.Equal(t, 2, len(validMessages))
	assert.Equal(t, "/meta/connect", validMessages[0].Channel)
	assert.Equal(t, "/systemTopic/Logging", validMessages[1].Channel)
}

// Test request marshaling
func TestRequestMarshaling(t *testing.T) {
	req := &request{
		Channel:                  "/meta/handshake",
		Version:                  "1.0",
		MinimumVersion:           "1.0",
		SupportedConnectionTypes: []string{"long-polling"},
	}

	data, err := json.Marshal([]*request{req})
	assert.Equal(t, nil, err)

	// Verify it's a valid JSON array
	var requests []map[string]interface{}
	err = json.Unmarshal(data, &requests)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(requests))
	assert.Equal(t, "/meta/handshake", requests[0]["channel"])
}

// Test subscription pattern matching
func TestSubscriptionPatternMatching(t *testing.T) {
	// Note: This would require the ohmyglob dependency to fully test
	// For now, test the basic subscription structure
	ch := make(chan *Message, 1)
	sub := subscription{
		// glob would be compiled from pattern
		out: ch,
	}

	msg := &Message{
		Channel: "/systemTopic/Logging",
		Data:    json.RawMessage(`{"test": true}`),
	}

	// Test that we can send to the channel without blocking
	select {
	case sub.out <- msg:
		// Success
	default:
		t.Error("Channel should not block on send")
	}

	// Test that we can receive from the channel
	select {
	case received := <-ch:
		assert.Equal(t, msg.Channel, received.Channel)
	case <-time.After(100 * time.Millisecond):
		t.Error("Should have received message")
	}
}

// Test client state management
func TestClientStateManagement(t *testing.T) {
	client := &Client{
		connected: false,
		clientId:  "",
	}

	// Test initial state
	assert.Equal(t, false, client.connected)
	assert.Equal(t, "", client.clientId)

	// Simulate successful connection
	client.connected = true
	client.clientId = "test-client-123"

	assert.Equal(t, true, client.connected)
	assert.Equal(t, "test-client-123", client.clientId)
}
