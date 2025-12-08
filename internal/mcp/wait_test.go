package mcp

import (
	"testing"
	"time"

	"github.com/CLIAIMONITOR/internal/events"
)

// TestWaitForEvents_ReceivesEvent tests that wait_for_events receives published events
func TestWaitForEvents_ReceivesEvent(t *testing.T) {
	// Create event bus with nil store for testing
	bus := events.NewBus(nil)
	server := NewServer()

	// Register the wait_for_events tool
	RegisterWaitForEventsTool(server, bus)

	// Set up a goroutine to publish an event after a short delay
	agentID := "test-agent-1"
	go func() {
		time.Sleep(100 * time.Millisecond)
		event := events.NewEvent(
			events.EventMessage,
			"test-source",
			agentID,
			events.PriorityNormal,
			map[string]interface{}{
				"text": "Hello from test",
			},
		)
		bus.Publish(event)
	}()

	// Call wait_for_events tool
	params := map[string]interface{}{
		"timeout_seconds": float64(5),
	}

	result, err := server.tools.Execute("wait_for_events", agentID, params)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got: %T", result)
	}

	status, _ := resultMap["status"].(string)
	if status != "event_received" {
		t.Errorf("Expected status 'event_received', got: %s", status)
	}

	// Check that event is present
	if _, hasEvent := resultMap["event"]; !hasEvent {
		t.Error("Expected result to contain 'event' field")
	}
}

// TestWaitForEvents_Timeout tests that wait_for_events times out when no events arrive
func TestWaitForEvents_Timeout(t *testing.T) {
	// Create event bus with nil store for testing
	bus := events.NewBus(nil)
	server := NewServer()

	// Register the wait_for_events tool
	RegisterWaitForEventsTool(server, bus)

	agentID := "test-agent-2"

	// Call wait_for_events with short timeout and no events
	params := map[string]interface{}{
		"timeout_seconds": float64(1), // 1 second timeout
	}

	start := time.Now()
	result, err := server.tools.Execute("wait_for_events", agentID, params)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that it actually waited approximately the timeout duration
	if elapsed < 900*time.Millisecond || elapsed > 1500*time.Millisecond {
		t.Errorf("Expected timeout around 1 second, got: %v", elapsed)
	}

	// Check result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got: %T", result)
	}

	status, _ := resultMap["status"].(string)
	if status != "timeout" {
		t.Errorf("Expected status 'timeout', got: %s", status)
	}
}

// TestWaitForEvents_FilterByType tests that event type filtering works correctly
func TestWaitForEvents_FilterByType(t *testing.T) {
	// Create event bus with nil store for testing
	bus := events.NewBus(nil)
	server := NewServer()

	// Register the wait_for_events tool
	RegisterWaitForEventsTool(server, bus)

	agentID := "test-agent-3"

	// Publish an EventAlert (should be ignored by filter)
	go func() {
		time.Sleep(100 * time.Millisecond)
		alertEvent := events.NewEvent(
			events.EventAlert,
			"test-source",
			agentID,
			events.PriorityCritical,
			map[string]interface{}{
				"message": "This is an alert",
			},
		)
		bus.Publish(alertEvent)

		// Publish an EventTask (should be received)
		time.Sleep(100 * time.Millisecond)
		taskEvent := events.NewEvent(
			events.EventTask,
			"test-source",
			agentID,
			events.PriorityNormal,
			map[string]interface{}{
				"task_id": "task-123",
			},
		)
		bus.Publish(taskEvent)
	}()

	// Call wait_for_events with filter for EventTask only
	params := map[string]interface{}{
		"timeout_seconds": float64(5),
		"event_types":     []interface{}{"task"}, // Filter for task events only
	}

	result, err := server.tools.Execute("wait_for_events", agentID, params)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got: %T", result)
	}

	status, _ := resultMap["status"].(string)
	if status != "event_received" {
		t.Errorf("Expected status 'event_received', got: %s", status)
	}

	// Check that the received event is a task event
	eventData, ok := resultMap["event"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'event' field to be a map")
	}

	eventType, _ := eventData["type"].(string)
	if eventType != "task" {
		t.Errorf("Expected event type 'task', got: %s", eventType)
	}
}

// TestWaitForEvents_TimeoutClamping tests that timeout values are clamped correctly
func TestWaitForEvents_TimeoutClamping(t *testing.T) {
	bus := events.NewBus(nil)
	server := NewServer()
	RegisterWaitForEventsTool(server, bus)

	tests := []struct {
		name           string
		inputTimeout   float64
		expectedMin    time.Duration
		expectedMax    time.Duration
	}{
		{
			name:         "Zero timeout clamped to 1 second",
			inputTimeout: 0,
			expectedMin:  900 * time.Millisecond,
			expectedMax:  1500 * time.Millisecond,
		},
		{
			name:         "Negative timeout clamped to 1 second",
			inputTimeout: -5,
			expectedMin:  900 * time.Millisecond,
			expectedMax:  1500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]interface{}{
				"timeout_seconds": tt.inputTimeout,
			}

			start := time.Now()
			_, err := server.tools.Execute("wait_for_events", "test-agent", params)
			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if elapsed < tt.expectedMin || elapsed > tt.expectedMax {
				t.Errorf("Expected timeout between %v and %v, got: %v", tt.expectedMin, tt.expectedMax, elapsed)
			}
		})
	}
}
