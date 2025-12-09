package events

import (
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of event
type EventType string

// Event type constants
const (
	EventMessage      EventType = "message"
	EventAgentSignal  EventType = "agent_signal"
	EventAlert        EventType = "alert"
	EventTask         EventType = "task"
	EventRecon        EventType = "recon"
	EventStopApproval EventType = "stop_approval" // Response to stop approval request
)

// Priority constants for events
const (
	PriorityCritical = 1
	PriorityHigh     = 2
	PriorityNormal   = 3
	PriorityLow      = 4
)

// Event represents a system event that can be published and subscribed to
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Source    string                 `json:"source"`
	Target    string                 `json:"target"`
	Priority  int                    `json:"priority"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt time.Time              `json:"created_at"`
}

// NewEvent creates a new event with auto-generated ID and timestamp
func NewEvent(eventType EventType, source, target string, priority int, payload map[string]interface{}) *Event {
	return &Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Source:    source,
		Target:    target,
		Priority:  priority,
		Payload:   payload,
		CreatedAt: time.Now(),
	}
}

// AllEventTypes returns all defined event types
func AllEventTypes() []EventType {
	return []EventType{
		EventMessage,
		EventAgentSignal,
		EventAlert,
		EventTask,
		EventRecon,
		EventStopApproval,
	}
}
