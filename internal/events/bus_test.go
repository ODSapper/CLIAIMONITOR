package events

import (
	"testing"
	"time"
)

func TestBus_PublishSubscribe(t *testing.T) {
	bus := NewBus(nil)

	// Subscribe to agent signals for a specific agent
	ch := bus.Subscribe("agent-1", []EventType{EventAgentSignal})

	// Publish an event to that agent
	event := NewEvent(EventAgentSignal, "captain", "agent-1", PriorityNormal, map[string]interface{}{
		"signal": "start",
	})
	bus.Publish(event)

	// Should receive the event
	select {
	case received := <-ch:
		if received.ID != event.ID {
			t.Errorf("Expected event ID %s, got %s", event.ID, received.ID)
		}
		if received.Type != EventAgentSignal {
			t.Errorf("Expected event type %s, got %s", EventAgentSignal, received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Did not receive event within timeout")
	}

	// Cleanup
	bus.Unsubscribe("agent-1", ch)
}

func TestBus_FilterByType(t *testing.T) {
	bus := NewBus(nil)

	// Subscribe only to messages
	ch := bus.Subscribe("agent-1", []EventType{EventMessage})

	// Publish a message event
	msgEvent := NewEvent(EventMessage, "captain", "agent-1", PriorityNormal, map[string]interface{}{
		"content": "Hello",
	})
	bus.Publish(msgEvent)

	// Should receive the message event
	select {
	case received := <-ch:
		if received.Type != EventMessage {
			t.Errorf("Expected event type %s, got %s", EventMessage, received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Did not receive message event")
	}

	// Publish a signal event (should NOT be received)
	signalEvent := NewEvent(EventAgentSignal, "captain", "agent-1", PriorityNormal, map[string]interface{}{
		"signal": "stop",
	})
	bus.Publish(signalEvent)

	// Should NOT receive the signal event
	select {
	case received := <-ch:
		t.Errorf("Should not have received event type %s", received.Type)
	case <-time.After(100 * time.Millisecond):
		// Expected timeout
	}

	// Cleanup
	bus.Unsubscribe("agent-1", ch)
}

func TestBus_BroadcastAll(t *testing.T) {
	bus := NewBus(nil)

	// Subscribe three different agents
	ch1 := bus.Subscribe("agent-1", []EventType{EventMessage})
	ch2 := bus.Subscribe("agent-2", []EventType{EventMessage})
	ch3 := bus.Subscribe("agent-3", []EventType{EventMessage})

	// Publish to "all"
	event := NewEvent(EventMessage, "captain", "all", PriorityNormal, map[string]interface{}{
		"broadcast": true,
	})
	bus.Publish(event)

	// All three should receive it
	agents := []struct {
		name string
		ch   <-chan Event
	}{
		{"agent-1", ch1},
		{"agent-2", ch2},
		{"agent-3", ch3},
	}

	for _, agent := range agents {
		select {
		case received := <-agent.ch:
			if received.ID != event.ID {
				t.Errorf("%s: Expected event ID %s, got %s", agent.name, event.ID, received.ID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("%s: Did not receive broadcast event", agent.name)
		}
	}

	// Cleanup
	bus.Unsubscribe("agent-1", ch1)
	bus.Unsubscribe("agent-2", ch2)
	bus.Unsubscribe("agent-3", ch3)
}

func TestBus_AllSubscriber(t *testing.T) {
	bus := NewBus(nil)

	// Subscribe to "all" - should receive events for any target
	allCh := bus.Subscribe("all", []EventType{EventMessage})

	// Specific agent subscriber
	agent1Ch := bus.Subscribe("agent-1", []EventType{EventMessage})

	// Publish to agent-1
	event := NewEvent(EventMessage, "captain", "agent-1", PriorityNormal, map[string]interface{}{
		"content": "Hello agent-1",
	})
	bus.Publish(event)

	// Both should receive it
	select {
	case received := <-agent1Ch:
		if received.ID != event.ID {
			t.Errorf("agent-1: Expected event ID %s, got %s", event.ID, received.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("agent-1 did not receive event")
	}

	select {
	case received := <-allCh:
		if received.ID != event.ID {
			t.Errorf("all subscriber: Expected event ID %s, got %s", event.ID, received.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("all subscriber did not receive event")
	}

	// Cleanup
	bus.Unsubscribe("all", allCh)
	bus.Unsubscribe("agent-1", agent1Ch)
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := NewBus(nil)

	// Subscribe
	ch := bus.Subscribe("agent-1", []EventType{EventMessage})

	// Publish first event
	event1 := NewEvent(EventMessage, "captain", "agent-1", PriorityNormal, map[string]interface{}{
		"content": "First",
	})
	bus.Publish(event1)

	// Should receive
	select {
	case <-ch:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Did not receive first event")
	}

	// Unsubscribe
	bus.Unsubscribe("agent-1", ch)

	// Publish second event
	event2 := NewEvent(EventMessage, "captain", "agent-1", PriorityNormal, map[string]interface{}{
		"content": "Second",
	})
	bus.Publish(event2)

	// Should NOT receive (channel should be closed)
	select {
	case event, ok := <-ch:
		if ok {
			t.Errorf("Should not have received event after unsubscribe: %+v", event)
		}
		// Channel closed is expected
	case <-time.After(100 * time.Millisecond):
		// Also acceptable - no more events
	}
}

func TestBus_MultipleSubscriptionsSameTarget(t *testing.T) {
	bus := NewBus(nil)

	// Multiple subscriptions for the same target
	ch1 := bus.Subscribe("agent-1", []EventType{EventMessage})
	ch2 := bus.Subscribe("agent-1", []EventType{EventMessage})

	// Publish event
	event := NewEvent(EventMessage, "captain", "agent-1", PriorityNormal, map[string]interface{}{
		"content": "Hello",
	})
	bus.Publish(event)

	// Both should receive
	select {
	case <-ch1:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch1 did not receive event")
	}

	select {
	case <-ch2:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch2 did not receive event")
	}

	// Cleanup
	bus.Unsubscribe("agent-1", ch1)
	bus.Unsubscribe("agent-1", ch2)
}

func TestBus_NoTypeFilter(t *testing.T) {
	bus := NewBus(nil)

	// Subscribe with nil types (should receive all types)
	ch := bus.Subscribe("agent-1", nil)

	// Publish different event types
	msgEvent := NewEvent(EventMessage, "captain", "agent-1", PriorityNormal, map[string]interface{}{})
	bus.Publish(msgEvent)

	signalEvent := NewEvent(EventAgentSignal, "captain", "agent-1", PriorityNormal, map[string]interface{}{})
	bus.Publish(signalEvent)

	alertEvent := NewEvent(EventAlert, "captain", "agent-1", PriorityNormal, map[string]interface{}{})
	bus.Publish(alertEvent)

	// Should receive all three
	receivedTypes := make(map[EventType]bool)
	for i := 0; i < 3; i++ {
		select {
		case event := <-ch:
			receivedTypes[event.Type] = true
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Did not receive all events")
		}
	}

	if !receivedTypes[EventMessage] {
		t.Error("Did not receive message event")
	}
	if !receivedTypes[EventAgentSignal] {
		t.Error("Did not receive signal event")
	}
	if !receivedTypes[EventAlert] {
		t.Error("Did not receive alert event")
	}

	// Cleanup
	bus.Unsubscribe("agent-1", ch)
}

func TestBus_FullChannelNonBlocking(t *testing.T) {
	bus := NewBus(nil)

	// Create subscription with small buffer for testing
	ch := bus.Subscribe("agent-1", []EventType{EventMessage})

	// Fill the channel buffer (100 events)
	for i := 0; i < 100; i++ {
		event := NewEvent(EventMessage, "captain", "agent-1", PriorityNormal, map[string]interface{}{
			"index": i,
		})
		bus.Publish(event)
	}

	// Publish one more event - should not block even if channel is full
	done := make(chan bool)
	go func() {
		event := NewEvent(EventMessage, "captain", "agent-1", PriorityNormal, map[string]interface{}{
			"index": 100,
		})
		bus.Publish(event)
		done <- true
	}()

	// Should complete quickly (non-blocking)
	select {
	case <-done:
		// Expected - publish should not block
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Publish blocked on full channel")
	}

	// Cleanup
	bus.Unsubscribe("agent-1", ch)
}
