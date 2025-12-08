package notifications

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CLIAIMONITOR/internal/events"
)

// mockNotifier is a test implementation of NotificationChannel
type mockNotifier struct {
	name    string
	sent    int32  // atomic counter
	filter  func(events.Event) bool
	sendErr error
	mu      sync.Mutex
	events  []events.Event
}

// newMockNotifier creates a new mock notifier with an optional filter function
func newMockNotifier(name string, filter func(events.Event) bool, sendErr error) *mockNotifier {
	if filter == nil {
		filter = func(events.Event) bool { return true }
	}
	return &mockNotifier{
		name:    name,
		filter:  filter,
		sendErr: sendErr,
		events:  make([]events.Event, 0),
	}
}

// Name returns the notifier name
func (m *mockNotifier) Name() string {
	return m.name
}

// ShouldNotify applies the filter function
func (m *mockNotifier) ShouldNotify(event events.Event) bool {
	return m.filter(event)
}

// Send simulates sending a notification
func (m *mockNotifier) Send(event events.Event) error {
	atomic.AddInt32(&m.sent, 1)

	m.mu.Lock()
	m.events = append(m.events, event)
	m.mu.Unlock()

	return m.sendErr
}

// GetSentCount returns the number of events sent
func (m *mockNotifier) GetSentCount() int {
	return int(atomic.LoadInt32(&m.sent))
}

// GetEvents returns a copy of all received events
func (m *mockNotifier) GetEvents() []events.Event {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]events.Event, len(m.events))
	copy(result, m.events)
	return result
}

func TestRouter_NewRouter(t *testing.T) {
	channels := []NotificationChannel{
		newMockNotifier("test1", nil, nil),
		newMockNotifier("test2", nil, nil),
	}

	router := NewRouter(channels)
	if router == nil {
		t.Fatal("NewRouter returned nil")
	}

	names := router.GetChannels()
	if len(names) != 2 {
		t.Errorf("expected 2 channels, got %d", len(names))
	}
}

func TestRouter_NewRouter_NilChannels(t *testing.T) {
	router := NewRouter(nil)
	if router == nil {
		t.Fatal("NewRouter returned nil")
	}

	names := router.GetChannels()
	if len(names) != 0 {
		t.Errorf("expected 0 channels, got %d", len(names))
	}
}

func TestRouter_AddChannel(t *testing.T) {
	router := NewRouter(nil)

	ch1 := newMockNotifier("ch1", nil, nil)
	router.AddChannel(ch1)

	names := router.GetChannels()
	if len(names) != 1 || names[0] != "ch1" {
		t.Errorf("expected [ch1], got %v", names)
	}

	ch2 := newMockNotifier("ch2", nil, nil)
	router.AddChannel(ch2)

	names = router.GetChannels()
	if len(names) != 2 {
		t.Errorf("expected 2 channels, got %d", len(names))
	}
}

func TestRouter_RemoveChannel(t *testing.T) {
	ch1 := newMockNotifier("ch1", nil, nil)
	ch2 := newMockNotifier("ch2", nil, nil)
	ch3 := newMockNotifier("ch3", nil, nil)

	router := NewRouter([]NotificationChannel{ch1, ch2, ch3})

	router.RemoveChannel("ch2")
	names := router.GetChannels()
	if len(names) != 2 {
		t.Errorf("expected 2 channels after removal, got %d", len(names))
	}

	// Verify ch2 was removed and ch1, ch3 remain
	for _, name := range names {
		if name == "ch2" {
			t.Error("ch2 should have been removed")
		}
	}

	// Remove non-existent channel should not panic
	router.RemoveChannel("nonexistent")
	names = router.GetChannels()
	if len(names) != 2 {
		t.Errorf("expected 2 channels after removing non-existent, got %d", len(names))
	}
}

func TestRouter_Route_AllChannels(t *testing.T) {
	ch1 := newMockNotifier("ch1", nil, nil)
	ch2 := newMockNotifier("ch2", nil, nil)
	ch3 := newMockNotifier("ch3", nil, nil)

	router := NewRouter([]NotificationChannel{ch1, ch2, ch3})

	event := events.NewEvent(
		events.EventAlert,
		"test-source",
		"test-target",
		events.PriorityHigh,
		map[string]interface{}{"msg": "test"},
	)

	router.Route(*event)

	// Wait for goroutines to complete
	time.Sleep(100 * time.Millisecond)

	// Verify all channels received the event
	if ch1.GetSentCount() != 1 {
		t.Errorf("ch1: expected 1 event sent, got %d", ch1.GetSentCount())
	}
	if ch2.GetSentCount() != 1 {
		t.Errorf("ch2: expected 1 event sent, got %d", ch2.GetSentCount())
	}
	if ch3.GetSentCount() != 1 {
		t.Errorf("ch3: expected 1 event sent, got %d", ch3.GetSentCount())
	}
}

func TestRouter_FilteredRoute(t *testing.T) {
	// Channel that only accepts critical priority events
	criticalOnly := newMockNotifier(
		"critical-only",
		func(e events.Event) bool {
			return e.Priority == events.PriorityCritical
		},
		nil,
	)

	// Channel that accepts all events
	allEvents := newMockNotifier("all", nil, nil)

	router := NewRouter([]NotificationChannel{criticalOnly, allEvents})

	// Send a normal priority event
	normalEvent := events.NewEvent(
		events.EventMessage,
		"src",
		"target",
		events.PriorityNormal,
		map[string]interface{}{},
	)
	router.Route(*normalEvent)

	time.Sleep(100 * time.Millisecond)

	if criticalOnly.GetSentCount() != 0 {
		t.Errorf("critical-only: expected 0 events (filtered out), got %d", criticalOnly.GetSentCount())
	}
	if allEvents.GetSentCount() != 1 {
		t.Errorf("all: expected 1 event, got %d", allEvents.GetSentCount())
	}

	// Send a critical priority event
	criticalEvent := events.NewEvent(
		events.EventAlert,
		"src",
		"target",
		events.PriorityCritical,
		map[string]interface{}{},
	)
	router.Route(*criticalEvent)

	time.Sleep(100 * time.Millisecond)

	if criticalOnly.GetSentCount() != 1 {
		t.Errorf("critical-only: expected 1 event, got %d", criticalOnly.GetSentCount())
	}
	if allEvents.GetSentCount() != 2 {
		t.Errorf("all: expected 2 events, got %d", allEvents.GetSentCount())
	}
}

func TestRouter_Route_ErrorHandling(t *testing.T) {
	// Channel that returns an error
	errChannel := newMockNotifier(
		"error-ch",
		nil,
		errors.New("send failed"),
	)

	// Channel that works fine
	okChannel := newMockNotifier("ok-ch", nil, nil)

	router := NewRouter([]NotificationChannel{errChannel, okChannel})

	event := events.NewEvent(
		events.EventMessage,
		"src",
		"target",
		events.PriorityNormal,
		map[string]interface{}{},
	)

	router.Route(*event)

	time.Sleep(100 * time.Millisecond)

	// Both channels should have attempted to send despite error
	if errChannel.GetSentCount() != 1 {
		t.Errorf("error-ch: expected 1 attempt, got %d", errChannel.GetSentCount())
	}
	if okChannel.GetSentCount() != 1 {
		t.Errorf("ok-ch: expected 1 event sent, got %d", okChannel.GetSentCount())
	}
}

func TestRouter_Route_MultipleEvents(t *testing.T) {
	ch := newMockNotifier("ch", nil, nil)
	router := NewRouter([]NotificationChannel{ch})

	// Send multiple events
	for i := 0; i < 5; i++ {
		event := events.NewEvent(
			events.EventMessage,
			"src",
			"target",
			events.PriorityNormal,
			map[string]interface{}{"index": i},
		)
		router.Route(*event)
	}

	time.Sleep(200 * time.Millisecond)

	if ch.GetSentCount() != 5 {
		t.Errorf("expected 5 events sent, got %d", ch.GetSentCount())
	}

	// Verify events were received
	events := ch.GetEvents()
	if len(events) != 5 {
		t.Errorf("expected 5 events in channel, got %d", len(events))
	}
}

func TestRouter_GetChannels(t *testing.T) {
	ch1 := newMockNotifier("alpha", nil, nil)
	ch2 := newMockNotifier("beta", nil, nil)
	ch3 := newMockNotifier("gamma", nil, nil)

	router := NewRouter([]NotificationChannel{ch1, ch2, ch3})

	names := router.GetChannels()
	if len(names) != 3 {
		t.Errorf("expected 3 channels, got %d", len(names))
	}

	// Verify names are present (order doesn't matter)
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	expectedNames := map[string]bool{"alpha": true, "beta": true, "gamma": true}
	for name := range expectedNames {
		if !nameMap[name] {
			t.Errorf("expected channel %s not found", name)
		}
	}
}

func TestRouter_ConcurrentAddRemove(t *testing.T) {
	router := NewRouter(nil)

	// Concurrently add and remove channels
	done := make(chan bool)

	for i := 0; i < 5; i++ {
		go func(id int) {
			ch := newMockNotifier("ch"+string(rune(id)), nil, nil)
			router.AddChannel(ch)
			done <- true
		}(i)
	}

	// Wait for additions
	for i := 0; i < 5; i++ {
		<-done
	}

	for i := 0; i < 3; i++ {
		go func(id int) {
			router.RemoveChannel("ch" + string(rune(id)))
			done <- true
		}(i)
	}

	// Wait for removals
	for i := 0; i < 3; i++ {
		<-done
	}

	names := router.GetChannels()
	if len(names) != 2 {
		t.Errorf("expected 2 channels after concurrent operations, got %d", len(names))
	}
}

func TestRouter_Route_ConcurrentSending(t *testing.T) {
	channels := make([]NotificationChannel, 10)
	for i := 0; i < 10; i++ {
		channels[i] = newMockNotifier("ch"+string(rune(i)), nil, nil)
	}

	router := NewRouter(channels)

	// Send events concurrently
	for i := 0; i < 20; i++ {
		go func(id int) {
			event := events.NewEvent(
				events.EventMessage,
				"src",
				"target",
				events.PriorityNormal,
				map[string]interface{}{"event_id": id},
			)
			router.Route(*event)
		}(i)
	}

	time.Sleep(500 * time.Millisecond)

	// Verify all channels received all events
	for _, ch := range channels {
		mock := ch.(*mockNotifier)
		if mock.GetSentCount() != 20 {
			t.Errorf("channel %s: expected 20 events, got %d", ch.Name(), mock.GetSentCount())
		}
	}
}

func TestRouter_EventPreservation(t *testing.T) {
	ch := newMockNotifier("test", nil, nil)
	router := NewRouter([]NotificationChannel{ch})

	originalEvent := events.NewEvent(
		events.EventAlert,
		"test-source",
		"test-target",
		events.PriorityCritical,
		map[string]interface{}{
			"key1": "value1",
			"key2": 42,
			"key3": true,
		},
	)

	router.Route(*originalEvent)
	time.Sleep(100 * time.Millisecond)

	receivedEvents := ch.GetEvents()
	if len(receivedEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(receivedEvents))
	}

	received := receivedEvents[0]

	// Verify event data is preserved
	if received.Type != originalEvent.Type {
		t.Errorf("event type mismatch: %s != %s", received.Type, originalEvent.Type)
	}
	if received.Source != originalEvent.Source {
		t.Errorf("source mismatch: %s != %s", received.Source, originalEvent.Source)
	}
	if received.Target != originalEvent.Target {
		t.Errorf("target mismatch: %s != %s", received.Target, originalEvent.Target)
	}
	if received.Priority != originalEvent.Priority {
		t.Errorf("priority mismatch: %d != %d", received.Priority, originalEvent.Priority)
	}

	// Verify payload
	for k, v := range originalEvent.Payload {
		if received.Payload[k] != v {
			t.Errorf("payload[%s] mismatch: %v != %v", k, received.Payload[k], v)
		}
	}
}
