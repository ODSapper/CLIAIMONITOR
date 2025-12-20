package server

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// PresenceTracker monitors agent online/offline status via NATS presence messages
type PresenceTracker struct {
	nc     *nats.Conn
	server *Server

	// Map of agent IDs to last seen timestamps
	mu       sync.RWMutex
	lastSeen map[string]time.Time

	// Subscriptions
	onlineSub  *nats.Subscription
	offlineSub *nats.Subscription

	// Control
	stopChan chan struct{}
}

// NewPresenceTracker creates a new presence tracker
func NewPresenceTracker(nc *nats.Conn, server *Server) *PresenceTracker {
	return &PresenceTracker{
		nc:       nc,
		server:   server,
		lastSeen: make(map[string]time.Time),
		stopChan: make(chan struct{}),
	}
}

// Start begins monitoring presence messages
func (p *PresenceTracker) Start() error {
	// Subscribe to presence.online.* messages
	onlineSub, err := p.nc.Subscribe("presence.online.*", func(msg *nats.Msg) {
		agentID := extractAgentID(msg.Subject)
		if agentID == "" {
			log.Printf("[PRESENCE] Warning: Could not extract agent ID from subject: %s", msg.Subject)
			return
		}

		log.Printf("[PRESENCE] Agent %s is online", agentID)

		// Update last seen timestamp
		p.mu.Lock()
		p.lastSeen[agentID] = time.Now()
		p.mu.Unlock()

		// Mark agent as connected
		if err := p.server.atomicAgentUpdate(agentID, "connected", ""); err != nil {
			log.Printf("[PRESENCE] ERROR: Failed to mark agent %s as connected: %v", agentID, err)
		}

		p.server.broadcastState()
	})
	if err != nil {
		return err
	}
	p.onlineSub = onlineSub
	log.Printf("[PRESENCE] Subscribed to presence.online.*")

	// Subscribe to presence.offline.* messages
	offlineSub, err := p.nc.Subscribe("presence.offline.*", func(msg *nats.Msg) {
		agentID := extractAgentID(msg.Subject)
		if agentID == "" {
			log.Printf("[PRESENCE] Warning: Could not extract agent ID from subject: %s", msg.Subject)
			return
		}

		log.Printf("[PRESENCE] Agent %s is offline", agentID)

		// Remove from last seen map
		p.mu.Lock()
		delete(p.lastSeen, agentID)
		p.mu.Unlock()

		// Mark agent as disconnected
		if err := p.server.atomicAgentUpdate(agentID, "disconnected", ""); err != nil {
			log.Printf("[PRESENCE] ERROR: Failed to mark agent %s as disconnected: %v", agentID, err)
		}

		p.server.broadcastState()
	})
	if err != nil {
		// Clean up online subscription
		p.onlineSub.Unsubscribe()
		return err
	}
	p.offlineSub = offlineSub
	log.Printf("[PRESENCE] Subscribed to presence.offline.*")

	// Start background goroutine to check for stale presence
	go p.monitorStaleAgents()

	log.Printf("[PRESENCE] Presence tracker started")
	return nil
}

// Stop stops the presence tracker
func (p *PresenceTracker) Stop() {
	close(p.stopChan)

	// Unsubscribe from NATS
	if p.onlineSub != nil {
		p.onlineSub.Unsubscribe()
	}
	if p.offlineSub != nil {
		p.offlineSub.Unsubscribe()
	}

	log.Printf("[PRESENCE] Presence tracker stopped")
}

// monitorStaleAgents runs in the background and checks for agents that haven't been seen in 2 minutes
// NOTE: Only marks agents as disconnected if they are in an idle state (connected, idle).
// Agents that are actively working (working, blocked, etc.) are NOT marked stale.
func (p *PresenceTracker) monitorStaleAgents() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	const staleThreshold = 2 * time.Minute

	// Active states that should NOT be marked as disconnected due to stale presence
	activeStates := map[string]bool{
		"working":     true,
		"blocked":     true,
		"in_progress": true,
		"starting":    true,
	}

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.mu.Lock()
			now := time.Now()
			for agentID, lastSeen := range p.lastSeen {
				if now.Sub(lastSeen) > staleThreshold {
					// Check current agent status before marking as disconnected
					state := p.server.store.GetState()
					if agent, ok := state.Agents[agentID]; ok {
						if activeStates[string(agent.Status)] {
							// Agent is actively working, don't mark as disconnected
							log.Printf("[PRESENCE] Agent %s stale but status is '%s' - keeping active",
								agentID, agent.Status)
							// Update lastSeen to prevent repeated logs
							p.lastSeen[agentID] = now
							continue
						}
					}

					log.Printf("[PRESENCE] Agent %s is stale (last seen: %v ago), marking as disconnected",
						agentID, now.Sub(lastSeen))

					// Mark as disconnected
					if err := p.server.atomicAgentUpdate(agentID, "disconnected", ""); err != nil {
						log.Printf("[PRESENCE] ERROR: Failed to mark stale agent %s as disconnected: %v", agentID, err)
					}

					// Remove from map
					delete(p.lastSeen, agentID)

					p.server.broadcastState()
				}
			}
			p.mu.Unlock()
		}
	}
}

// extractAgentID parses agent ID from a presence subject like "presence.online.team-sntgreen001"
// Returns empty string if parsing fails
func extractAgentID(subject string) string {
	// Subject format: presence.online.{agentID} or presence.offline.{agentID}
	parts := strings.Split(subject, ".")
	if len(parts) != 3 {
		return ""
	}
	return parts[2]
}
