package events

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Subscription represents a subscription to events
type Subscription struct {
	Ch     chan Event   // Channel to receive events
	Types  []EventType  // Event types to filter (nil/empty = all types)
	Target string       // Target identifier
}

// EventStore defines the interface for persisting events
type EventStore interface {
	Save(event *Event) error
	GetPending(target string, types []EventType) ([]*Event, error)
	MarkDelivered(eventID string) error
}

// Backpressure configuration constants
const (
	// MaxBackpressureRetries is the number of times to retry sending before dropping
	MaxBackpressureRetries = 3
	// BackpressureRetryDelay is the delay between retry attempts
	BackpressureRetryDelay = 10 * time.Millisecond
)

// Bus manages event subscriptions and publishing
type Bus struct {
	subscribers   map[string][]*Subscription // target -> subscriptions
	store         EventStore                 // Optional persistent store
	mu            sync.RWMutex               // Protects subscribers map
	droppedEvents uint64                     // Counter for dropped events (atomic)
}

// NewBus creates a new event bus
func NewBus(store EventStore) *Bus {
	return &Bus{
		subscribers: make(map[string][]*Subscription),
		store:       store,
	}
}

// Subscribe creates a new subscription for the given target and event types.
// Returns a channel that will receive matching events.
// If types is nil or empty, all event types will be received.
func (b *Bus) Subscribe(target string, types []EventType) <-chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub := &Subscription{
		Ch:     make(chan Event, 100), // Buffered channel
		Types:  types,
		Target: target,
	}

	b.subscribers[target] = append(b.subscribers[target], sub)

	return sub.Ch
}

// Unsubscribe removes a subscription and closes its channel
func (b *Bus) Unsubscribe(target string, ch <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs, exists := b.subscribers[target]
	if !exists {
		return
	}

	// Find and remove the subscription
	for i, sub := range subs {
		if sub.Ch == ch {
			// Close the channel
			close(sub.Ch)

			// Remove from slice
			b.subscribers[target] = append(subs[:i], subs[i+1:]...)

			// Clean up empty target entries
			if len(b.subscribers[target]) == 0 {
				delete(b.subscribers, target)
			}

			return
		}
	}
}

// Publish sends an event to all matching subscribers.
// Events are sent to:
// 1. Subscribers for the specific target
// 2. Subscribers for "all" (if target is not "all")
// 3. All subscribers (if target is "all")
func (b *Bus) Publish(event *Event) {
	// Persist to store if available
	if b.store != nil {
		if err := b.store.Save(event); err != nil {
			log.Printf("[EVENTS] ERROR: Failed to persist event to store: type=%s, target=%s, id=%s, error=%v",
				event.Type, event.Target, event.ID, err)
		}
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	// Collect all matching subscriptions
	var targetSubs []*Subscription

	if event.Target == "all" {
		// Broadcast to everyone
		for _, subs := range b.subscribers {
			targetSubs = append(targetSubs, subs...)
		}
	} else {
		// Send to specific target
		if subs, exists := b.subscribers[event.Target]; exists {
			targetSubs = append(targetSubs, subs...)
		}

		// Also send to "all" subscribers
		if subs, exists := b.subscribers["all"]; exists {
			targetSubs = append(targetSubs, subs...)
		}
	}

	// Send to all matching subscriptions
	for _, sub := range targetSubs {
		if b.matchesTypes(event.Type, sub.Types) {
			b.sendWithBackpressure(sub, event)
		}
	}
}

// sendWithBackpressure attempts to send an event to a subscriber with retries.
// If the channel is full, it retries a few times before logging and dropping the event.
// The event is still persisted to the store (if available) and can be retrieved later.
func (b *Bus) sendWithBackpressure(sub *Subscription, event *Event) {
	// First attempt - non-blocking
	select {
	case sub.Ch <- *event:
		return // Success on first try
	default:
		// Channel full, apply backpressure with retries
	}

	// Retry with brief delays to allow channel to drain
	for retry := 1; retry <= MaxBackpressureRetries; retry++ {
		time.Sleep(BackpressureRetryDelay)
		select {
		case sub.Ch <- *event:
			log.Printf("[EVENTS] Event delivered after %d retry(ies): type=%s, target=%s, id=%s",
				retry, event.Type, event.Target, event.ID)
			return
		default:
			// Still full, continue retrying
		}
	}

	// All retries exhausted, drop the event
	dropped := atomic.AddUint64(&b.droppedEvents, 1)
	log.Printf("[EVENTS] WARNING: Dropped event after %d retries (channel full): type=%s, target=%s, source=%s, id=%s (total dropped: %d)",
		MaxBackpressureRetries, event.Type, event.Target, event.Source, event.ID, dropped)
}

// GetPendingEvents retrieves pending events from the store for a specific target
func (b *Bus) GetPendingEvents(target string, types []EventType) ([]*Event, error) {
	if b.store == nil {
		return nil, nil
	}

	return b.store.GetPending(target, types)
}

// MarkDelivered marks an event as delivered so it won't be returned by GetPendingEvents
func (b *Bus) MarkDelivered(eventID string) error {
	if b.store == nil {
		return nil
	}

	return b.store.MarkDelivered(eventID)
}

// DroppedEventCount returns the total number of events that were dropped
// due to full subscriber channels
func (b *Bus) DroppedEventCount() uint64 {
	return atomic.LoadUint64(&b.droppedEvents)
}

// matchesTypes checks if an event type matches the subscription filter
func (b *Bus) matchesTypes(eventType EventType, types []EventType) bool {
	// Nil or empty types means accept all
	if len(types) == 0 {
		return true
	}

	// Check if event type is in the filter list
	for _, t := range types {
		if t == eventType {
			return true
		}
	}

	return false
}
