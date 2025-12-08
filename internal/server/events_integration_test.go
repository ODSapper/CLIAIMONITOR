// internal/server/events_integration_test.go
package server

import (
	"testing"
	"time"

	"github.com/CLIAIMONITOR/internal/events"
)

func TestEventBus_EndToEnd(t *testing.T) {
	// Create event bus without store (in-memory only for test)
	bus := events.NewBus(nil)

	// Simulate Captain subscribing
	captainSub := bus.Subscribe("Captain", nil)
	defer bus.Unsubscribe("Captain", captainSub)

	// Simulate dashboard sending message
	event := events.NewEvent(
		events.EventMessage,
		"human",
		"Captain",
		events.PriorityNormal,
		map[string]interface{}{"text": "Hello Captain!"},
	)
	bus.Publish(event)

	// Captain should receive immediately
	select {
	case received := <-captainSub:
		if received.ID != event.ID {
			t.Errorf("got event %s, want %s", received.ID, event.ID)
		}
		t.Logf("Captain received event: type=%s source=%s", received.Type, received.Source)
	case <-time.After(1 * time.Second):
		t.Error("Captain did not receive event within timeout")
	}
}

func TestEventBus_AgentSignal(t *testing.T) {
	bus := events.NewBus(nil)

	// Captain subscribes to all events
	captainSub := bus.Subscribe("Captain", nil)
	defer bus.Unsubscribe("Captain", captainSub)

	// Agent sends blocked signal
	event := events.NewEvent(
		events.EventAgentSignal,
		"agent-001",
		"Captain",
		events.PriorityHigh,
		map[string]interface{}{
			"signal": "blocked",
			"task":   "waiting for guidance",
		},
	)
	bus.Publish(event)

	// Captain should receive
	select {
	case received := <-captainSub:
		if received.Type != events.EventAgentSignal {
			t.Errorf("got type %s, want agent_signal", received.Type)
		}
		if received.Priority != events.PriorityHigh {
			t.Errorf("got priority %d, want %d (high)", received.Priority, events.PriorityHigh)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for agent signal")
	}
}

func TestEventBus_FilteredSubscription(t *testing.T) {
	bus := events.NewBus(nil)

	// Subscribe only to alerts
	alertSub := bus.Subscribe("monitor", []events.EventType{events.EventAlert})
	defer bus.Unsubscribe("monitor", alertSub)

	// Send message (should be filtered)
	bus.Publish(events.NewEvent(events.EventMessage, "human", "monitor", events.PriorityNormal, nil))

	// Send alert (should pass through)
	alert := events.NewEvent(events.EventAlert, "system", "monitor", events.PriorityCritical, nil)
	bus.Publish(alert)

	select {
	case received := <-alertSub:
		if received.Type != events.EventAlert {
			t.Errorf("expected alert, got %s", received.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("did not receive alert event")
	}
}
