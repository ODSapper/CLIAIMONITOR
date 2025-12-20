package notifications

import (
	"log"
	"sync"

	"github.com/CLIAIMONITOR/internal/events"
)

// NotificationChannel represents a channel that can send notifications
type NotificationChannel interface {
	// Name returns the channel name
	Name() string

	// ShouldNotify checks if an event should trigger a notification on this channel
	ShouldNotify(event events.Event) bool

	// Send sends a notification to the channel
	Send(event events.Event) error
}

// Router dispatches events to multiple notification channels
type Router struct {
	channels []NotificationChannel
	mu       sync.RWMutex
}

// NewRouter creates a new notification router with the provided channels
func NewRouter(channels []NotificationChannel) *Router {
	if channels == nil {
		channels = []NotificationChannel{}
	}
	return &Router{
		channels: channels,
	}
}

// AddChannel adds a notification channel to the router
func (r *Router) AddChannel(channel NotificationChannel) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.channels = append(r.channels, channel)
}

// RemoveChannel removes a notification channel by name
func (r *Router) RemoveChannel(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	filtered := make([]NotificationChannel, 0, len(r.channels))
	for _, ch := range r.channels {
		if ch.Name() != name {
			filtered = append(filtered, ch)
		}
	}
	r.channels = filtered
}

// Route sends an event to all matching notification channels asynchronously
// It uses a goroutine per channel and logs failures without returning errors (fire-and-forget)
func (r *Router) Route(event events.Event) {
	// Copy channels slice under read lock
	r.mu.RLock()
	channels := make([]NotificationChannel, len(r.channels))
	copy(channels, r.channels)
	r.mu.RUnlock()

	// Send to each channel in a separate goroutine
	for _, ch := range channels {
		go func(channel NotificationChannel) {
			// Check if the channel should handle this event
			if !channel.ShouldNotify(event) {
				return
			}

			// Send the event to the channel
			if err := channel.Send(event); err != nil {
				log.Printf("[NOTIFY-ROUTER] failed to send event %s to channel %s: %v", event.ID, channel.Name(), err)
			}
		}(ch)
	}
}

// RouteWithWait routes an event and waits for all channels to complete
// Unlike Route, this method blocks until all notification channels have finished processing
func (r *Router) RouteWithWait(event events.Event) {
	// Copy channels slice under read lock
	r.mu.RLock()
	channels := make([]NotificationChannel, len(r.channels))
	copy(channels, r.channels)
	r.mu.RUnlock()

	// Send to each channel in a separate goroutine with WaitGroup tracking
	var wg sync.WaitGroup
	for _, ch := range channels {
		wg.Add(1)
		go func(channel NotificationChannel) {
			defer wg.Done()

			// Check if the channel should handle this event
			if !channel.ShouldNotify(event) {
				return
			}

			// Send the event to the channel
			if err := channel.Send(event); err != nil {
				log.Printf("[NOTIFY-ROUTER] failed to send event %s to channel %s: %v", event.ID, channel.Name(), err)
			}
		}(ch)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

// GetChannels returns a list of all registered channel names
func (r *Router) GetChannels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, len(r.channels))
	for i, ch := range r.channels {
		names[i] = ch.Name()
	}
	return names
}
