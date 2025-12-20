package nats

import (
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

// StreamManager manages JetStream streams for the application
type StreamManager struct {
	js nats.JetStreamContext
}

// NewStreamManager creates a new StreamManager with JetStream context
func NewStreamManager(nc *nats.Conn) (*StreamManager, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}

	return &StreamManager{
		js: js,
	}, nil
}

// SetupStreams creates or updates all required JetStream streams
func (sm *StreamManager) SetupStreams() error {
	// Define stream configurations
	streams := []nats.StreamConfig{
		{
			Name:        "CHAT",
			Description: "Chat messages between Captain and agents",
			Subjects:    []string{"chat.>"},
			Storage:     nats.FileStorage,
			MaxAge:      24 * time.Hour,
			Retention:   nats.LimitsPolicy,
		},
		{
			Name:        "PRESENCE",
			Description: "Agent presence and heartbeat messages",
			Subjects:    []string{"presence.>"},
			Storage:     nats.MemoryStorage,
			MaxAge:      5 * time.Minute,
			Retention:   nats.LimitsPolicy,
		},
		{
			Name:        "COMMANDS",
			Description: "Command messages to agents",
			Subjects:    []string{"cmd.>"},
			Storage:     nats.MemoryStorage,
			MaxAge:      1 * time.Hour,
			Retention:   nats.LimitsPolicy,
		},
	}

	// Create or update each stream
	for _, streamCfg := range streams {
		if err := sm.createOrUpdateStream(streamCfg); err != nil {
			return err
		}
	}

	log.Println("[NATS-STREAMS] All streams configured successfully")
	return nil
}

// createOrUpdateStream creates a new stream or updates an existing one
func (sm *StreamManager) createOrUpdateStream(cfg nats.StreamConfig) error {
	// Try to get existing stream info
	info, err := sm.js.StreamInfo(cfg.Name)

	if err != nil {
		// Stream doesn't exist, create it
		if err == nats.ErrStreamNotFound {
			log.Printf("[NATS-STREAMS] Creating stream %s with subjects %v", cfg.Name, cfg.Subjects)
			_, err := sm.js.AddStream(&cfg)
			if err != nil {
				log.Printf("[NATS-STREAMS] Error creating stream %s: %v", cfg.Name, err)
				return err
			}
			log.Printf("[NATS-STREAMS] Stream %s created successfully", cfg.Name)
			return nil
		}

		// Other error occurred
		log.Printf("[NATS-STREAMS] Error getting stream info for %s: %v", cfg.Name, err)
		return err
	}

	// Stream exists, update it if needed
	log.Printf("[NATS-STREAMS] Stream %s already exists, updating configuration", cfg.Name)
	_, err = sm.js.UpdateStream(&cfg)
	if err != nil {
		log.Printf("[NATS-STREAMS] Error updating stream %s: %v", cfg.Name, err)
		return err
	}

	log.Printf("[NATS-STREAMS] Stream %s updated successfully (messages: %d)", cfg.Name, info.State.Msgs)
	return nil
}

// DeleteStream deletes a stream by name (useful for cleanup/testing)
func (sm *StreamManager) DeleteStream(name string) error {
	log.Printf("[NATS-STREAMS] Deleting stream %s", name)
	err := sm.js.DeleteStream(name)
	if err != nil {
		log.Printf("[NATS-STREAMS] Error deleting stream %s: %v", name, err)
		return err
	}
	log.Printf("[NATS-STREAMS] Stream %s deleted successfully", name)
	return nil
}

// GetStreamInfo returns information about a specific stream
func (sm *StreamManager) GetStreamInfo(name string) (*nats.StreamInfo, error) {
	return sm.js.StreamInfo(name)
}
