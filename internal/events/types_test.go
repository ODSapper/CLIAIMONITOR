package events

import (
	"encoding/json"
	"testing"
	"time"
)

// TestEventType_String verifies event type constants
func TestEventType_String(t *testing.T) {
	tests := []struct {
		name     string
		eventType EventType
		expected string
	}{
		{"Message event", EventMessage, "message"},
		{"Agent signal event", EventAgentSignal, "agent_signal"},
		{"Alert event", EventAlert, "alert"},
		{"Task event", EventTask, "task"},
		{"Recon event", EventRecon, "recon"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.eventType) != tt.expected {
				t.Errorf("EventType = %v, want %v", tt.eventType, tt.expected)
			}
		})
	}
}

// TestPriorityConstants verifies priority level constants
func TestPriorityConstants(t *testing.T) {
	if PriorityCritical != 1 {
		t.Errorf("PriorityCritical = %d, want 1", PriorityCritical)
	}
	if PriorityHigh != 2 {
		t.Errorf("PriorityHigh = %d, want 2", PriorityHigh)
	}
	if PriorityNormal != 3 {
		t.Errorf("PriorityNormal = %d, want 3", PriorityNormal)
	}
	if PriorityLow != 4 {
		t.Errorf("PriorityLow = %d, want 4", PriorityLow)
	}
}

// TestEvent_JSON verifies JSON marshal/unmarshal round-trip
func TestEvent_JSON(t *testing.T) {
	original := &Event{
		ID:       "test-id-123",
		Type:     EventMessage,
		Source:   "captain",
		Target:   "agent-1",
		Priority: PriorityHigh,
		Payload: map[string]interface{}{
			"message": "Hello, agent!",
			"count":   42,
		},
		CreatedAt: time.Date(2025, 12, 8, 10, 0, 0, 0, time.UTC),
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	// Unmarshal back to struct
	var decoded Event
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	// Verify all fields
	if decoded.ID != original.ID {
		t.Errorf("ID = %v, want %v", decoded.ID, original.ID)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type = %v, want %v", decoded.Type, original.Type)
	}
	if decoded.Source != original.Source {
		t.Errorf("Source = %v, want %v", decoded.Source, original.Source)
	}
	if decoded.Target != original.Target {
		t.Errorf("Target = %v, want %v", decoded.Target, original.Target)
	}
	if decoded.Priority != original.Priority {
		t.Errorf("Priority = %v, want %v", decoded.Priority, original.Priority)
	}
	if !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", decoded.CreatedAt, original.CreatedAt)
	}

	// Verify payload
	if decoded.Payload["message"] != "Hello, agent!" {
		t.Errorf("Payload.message = %v, want 'Hello, agent!'", decoded.Payload["message"])
	}
	if int(decoded.Payload["count"].(float64)) != 42 {
		t.Errorf("Payload.count = %v, want 42", decoded.Payload["count"])
	}
}

// TestNewEvent verifies event constructor generates ID and timestamp
func TestNewEvent(t *testing.T) {
	beforeCreate := time.Now()

	event := NewEvent(EventTask, "captain", "agent-1", PriorityNormal, map[string]interface{}{
		"task_id": "task-123",
	})

	afterCreate := time.Now()

	// Verify ID is generated (UUID format)
	if event.ID == "" {
		t.Error("NewEvent did not generate ID")
	}
	if len(event.ID) != 36 { // Standard UUID length with hyphens
		t.Errorf("Generated ID has unexpected length: %d, want 36", len(event.ID))
	}

	// Verify timestamp is set and within reasonable range
	if event.CreatedAt.IsZero() {
		t.Error("NewEvent did not set CreatedAt timestamp")
	}
	if event.CreatedAt.Before(beforeCreate) || event.CreatedAt.After(afterCreate) {
		t.Errorf("CreatedAt timestamp %v is outside expected range [%v, %v]",
			event.CreatedAt, beforeCreate, afterCreate)
	}

	// Verify other fields
	if event.Type != EventTask {
		t.Errorf("Type = %v, want %v", event.Type, EventTask)
	}
	if event.Source != "captain" {
		t.Errorf("Source = %v, want 'captain'", event.Source)
	}
	if event.Target != "agent-1" {
		t.Errorf("Target = %v, want 'agent-1'", event.Target)
	}
	if event.Priority != PriorityNormal {
		t.Errorf("Priority = %v, want %v", event.Priority, PriorityNormal)
	}
	if event.Payload["task_id"] != "task-123" {
		t.Errorf("Payload.task_id = %v, want 'task-123'", event.Payload["task_id"])
	}
}

// TestAllEventTypes verifies the helper function returns all event types
func TestAllEventTypes(t *testing.T) {
	types := AllEventTypes()

	expectedCount := 5
	if len(types) != expectedCount {
		t.Errorf("AllEventTypes returned %d types, want %d", len(types), expectedCount)
	}

	// Verify all expected types are present
	typeMap := make(map[EventType]bool)
	for _, et := range types {
		typeMap[et] = true
	}

	expectedTypes := []EventType{
		EventMessage,
		EventAgentSignal,
		EventAlert,
		EventTask,
		EventRecon,
	}

	for _, expected := range expectedTypes {
		if !typeMap[expected] {
			t.Errorf("AllEventTypes missing event type: %v", expected)
		}
	}
}
