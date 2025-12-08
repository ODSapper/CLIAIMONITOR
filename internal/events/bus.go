package events

import (
	"sync"
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

// Bus manages event subscriptions and publishing
type Bus struct {
	subscribers map[string][]*Subscription // target -> subscriptions
	store       EventStore                 // Optional persistent store
	mu          sync.RWMutex               // Protects subscribers map
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
		_ = b.store.Save(event) // Ignore errors for now
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
			// Non-blocking send
			select {
			case sub.Ch <- *event:
				// Event sent successfully
			default:
				// Channel full, drop event to avoid blocking
			}
		}
	}
}

// GetPendingEvents retrieves pending events from the store for a specific target
func (b *Bus) GetPendingEvents(target string, types []EventType) ([]*Event, error) {
	if b.store == nil {
		return nil, nil
	}

	return b.store.GetPending(target, types)
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
