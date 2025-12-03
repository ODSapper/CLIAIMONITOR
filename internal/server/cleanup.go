package server

import (
	"context"
	"log"
	"time"

	"github.com/CLIAIMONITOR/internal/instance"
	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/persistence"
)

// CleanupService monitors and removes stale agents
type CleanupService struct {
	memDB          memory.MemoryDB
	store          persistence.Store
	hub            *Hub
	checkInterval  time.Duration
	staleThreshold time.Duration
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(memDB memory.MemoryDB, store persistence.Store, hub *Hub) *CleanupService {
	return &CleanupService{
		memDB:          memDB,
		store:          store,
		hub:            hub,
		checkInterval:  30 * time.Second,
		staleThreshold: 120 * time.Second,
	}
}

// Start begins the cleanup monitoring loop
func (c *CleanupService) Start(ctx context.Context) {
	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	log.Println("[CLEANUP] Auto-cleanup service started")

	for {
		select {
		case <-ctx.Done():
			log.Println("[CLEANUP] Auto-cleanup service stopped")
			return
		case <-ticker.C:
			c.cleanupStaleAgents()
		}
	}
}

// cleanupStaleAgents finds and removes agents with stale heartbeats
func (c *CleanupService) cleanupStaleAgents() {
	// 1. Query DB for stale agents (heartbeat > 120s old)
	staleAgents, err := c.memDB.GetStaleAgents(c.staleThreshold)
	if err != nil {
		log.Printf("[CLEANUP] Error querying stale agents: %v", err)
		return
	}

	if len(staleAgents) == 0 {
		return
	}

	log.Printf("[CLEANUP] Found %d stale agents", len(staleAgents))

	for _, agent := range staleAgents {
		c.removeStaleAgent(agent)
	}

	// Broadcast updated state to dashboard
	if len(staleAgents) > 0 {
		c.hub.BroadcastState(c.store.GetState())
	}
}

// removeStaleAgent handles cleanup of a single stale agent
func (c *CleanupService) removeStaleAgent(agent *memory.AgentControl) {
	log.Printf("[CLEANUP] Removing stale agent %s (last heartbeat: %v)",
		agent.AgentID, agent.HeartbeatAt)

	// 1. Kill process if PID exists
	if agent.PID != nil && *agent.PID > 0 {
		if err := instance.KillProcess(*agent.PID); err != nil {
			log.Printf("[CLEANUP] Note: Process %d may have already exited: %v", *agent.PID, err)
		}
	}

	// 2. Mark as dead in DB
	if err := c.memDB.UpdateStatus(agent.AgentID, "dead", ""); err != nil {
		log.Printf("[CLEANUP] Error marking agent dead: %v", err)
	}

	// 3. Remove from state.json (dashboard state)
	c.store.RemoveAgent(agent.AgentID)

	log.Printf("[CLEANUP] Successfully removed agent %s", agent.AgentID)
}

// RunOnce performs a single cleanup cycle (for manual trigger via API)
func (c *CleanupService) RunOnce() int {
	staleAgents, err := c.memDB.GetStaleAgents(c.staleThreshold)
	if err != nil {
		return 0
	}

	for _, agent := range staleAgents {
		c.removeStaleAgent(agent)
	}

	if len(staleAgents) > 0 {
		c.hub.BroadcastState(c.store.GetState())
	}

	return len(staleAgents)
}

// SetIntervals allows configuring check and stale thresholds
func (c *CleanupService) SetIntervals(check, stale time.Duration) {
	c.checkInterval = check
	c.staleThreshold = stale
}
