package mcp

import (
	"log"
	"sync"
	"time"
)

// SSEPresenceTracker monitors agent presence via SSE connections
type SSEPresenceTracker struct {
	connections sync.Map // map[string]*SSEConnection (agentID -> connection)
	lastSeen    sync.Map // map[string]time.Time (agentID -> last seen timestamp)

	onOnline  func(agentID string)
	onOffline func(agentID string)

	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewSSEPresenceTracker creates a new SSE-based presence tracker
func NewSSEPresenceTracker(onOnline, onOffline func(agentID string)) *SSEPresenceTracker {
	return &SSEPresenceTracker{
		onOnline:  onOnline,
		onOffline: onOffline,
		stopChan:  make(chan struct{}),
	}
}

// OnConnect handles agent connection events
// Called when an agent establishes an SSE connection
func (t *SSEPresenceTracker) OnConnect(agentID string, conn *SSEConnection) {
	log.Printf("[SSE-PRESENCE] Agent %s connected (session: %s)", agentID, conn.SessionID)

	// Store connection and update last seen
	t.connections.Store(agentID, conn)
	t.lastSeen.Store(agentID, time.Now())

	// Notify online callback
	if t.onOnline != nil {
		t.onOnline(agentID)
	}
}

// OnDisconnect handles agent disconnection events
// Called when an agent's SSE connection is closed
func (t *SSEPresenceTracker) OnDisconnect(agentID string) {
	log.Printf("[SSE-PRESENCE] Agent %s disconnected", agentID)

	// Remove connection and last seen timestamp
	t.connections.Delete(agentID)
	t.lastSeen.Delete(agentID)

	// Notify offline callback
	if t.onOffline != nil {
		t.onOffline(agentID)
	}
}

// UpdateLastSeen updates the last seen timestamp for an agent
// Called by report_status MCP tool handler to keep agent marked as active
func (t *SSEPresenceTracker) UpdateLastSeen(agentID string) {
	t.lastSeen.Store(agentID, time.Now())
	log.Printf("[SSE-PRESENCE] Updated last seen for agent %s", agentID)
}

// StartStaleMonitor starts a background goroutine that checks for stale connections
// Agents that haven't reported status in 2 minutes are marked offline
func (t *SSEPresenceTracker) StartStaleMonitor() {
	t.wg.Add(1)
	go t.monitorStaleConnections()
	log.Printf("[SSE-PRESENCE] Stale monitor started (threshold: 2 minutes)")
}

// Stop stops the stale monitor goroutine
func (t *SSEPresenceTracker) Stop() {
	close(t.stopChan)
	t.wg.Wait()
	log.Printf("[SSE-PRESENCE] Stale monitor stopped")
}

// monitorStaleConnections runs in the background and marks stale agents as offline
func (t *SSEPresenceTracker) monitorStaleConnections() {
	defer t.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	const staleThreshold = 2 * time.Minute

	for {
		select {
		case <-t.stopChan:
			return
		case <-ticker.C:
			now := time.Now()

			// Check all connected agents for staleness
			t.lastSeen.Range(func(key, value interface{}) bool {
				agentID := key.(string)
				lastSeen := value.(time.Time)

				if now.Sub(lastSeen) > staleThreshold {
					log.Printf("[SSE-PRESENCE] Agent %s is stale (last seen: %v ago), marking as offline",
						agentID, now.Sub(lastSeen))

					// Mark as disconnected
					t.OnDisconnect(agentID)
				}

				return true // continue iteration
			})
		}
	}
}

// GetConnectedAgents returns a list of currently connected agent IDs
func (t *SSEPresenceTracker) GetConnectedAgents() []string {
	var agents []string

	t.connections.Range(func(key, value interface{}) bool {
		agents = append(agents, key.(string))
		return true
	})

	return agents
}

// IsConnected checks if an agent is currently connected
func (t *SSEPresenceTracker) IsConnected(agentID string) bool {
	_, ok := t.connections.Load(agentID)
	return ok
}

// GetLastSeen returns the last seen timestamp for an agent
func (t *SSEPresenceTracker) GetLastSeen(agentID string) (time.Time, bool) {
	if val, ok := t.lastSeen.Load(agentID); ok {
		return val.(time.Time), true
	}
	return time.Time{}, false
}
