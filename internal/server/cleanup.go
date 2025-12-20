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
	memDB            memory.MemoryDB
	store            persistence.Store
	hub              *Hub
	checkInterval    time.Duration
	staleThreshold   time.Duration
	pendingThreshold time.Duration // Max time for agents to transition from pending to connected
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(memDB memory.MemoryDB, store persistence.Store, hub *Hub) *CleanupService {
	return &CleanupService{
		memDB:            memDB,
		store:            store,
		hub:              hub,
		checkInterval:    30 * time.Second,
		staleThreshold:   120 * time.Second,
		pendingThreshold: 60 * time.Second, // Pending agents have 60s to connect
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
	removedCount := 0

	// 1. Query DB for stale agents (heartbeat > 120s old)
	staleAgents, err := c.memDB.GetStaleAgents(c.staleThreshold)
	if err != nil {
		log.Printf("[CLEANUP] Error querying stale agents: %v", err)
	} else if len(staleAgents) > 0 {
		log.Printf("[CLEANUP] Found %d stale agents", len(staleAgents))
		for _, agent := range staleAgents {
			c.removeStaleAgent(agent)
			removedCount++
		}
	}

	// 2. Clean up "pending" agents that never connected (two-phase registration orphans)
	pendingAgents, err := c.memDB.GetAgentsByStatus("pending")
	if err != nil {
		log.Printf("[CLEANUP] Error querying pending agents: %v", err)
	} else {
		for _, agent := range pendingAgents {
			// Check if agent has been pending for too long
			if time.Since(agent.SpawnedAt) > c.pendingThreshold {
				log.Printf("[CLEANUP] Removing orphan pending agent %s (spawned %v ago, never connected)",
					agent.AgentID, time.Since(agent.SpawnedAt))
				c.removePendingAgent(agent)
				removedCount++
			}
		}
	}

	// Broadcast updated state to dashboard
	if removedCount > 0 {
		c.hub.BroadcastState(c.store.GetState())
	}
}

// removePendingAgent handles cleanup of an agent that never connected
func (c *CleanupService) removePendingAgent(agent *memory.AgentControl) {
	// 1. Kill process if PID exists (process may be stuck)
	if agent.PID != nil && *agent.PID > 0 {
		if err := instance.KillProcess(*agent.PID); err != nil {
			log.Printf("[CLEANUP] Note: Pending process %d may have already exited: %v", *agent.PID, err)
		}
	}

	// 2. Mark as dead in DB (never connected)
	if err := c.memDB.UpdateStatus(agent.AgentID, "dead", "never connected"); err != nil {
		log.Printf("[CLEANUP] Error marking pending agent dead: %v", err)
	}

	// 3. Remove from state.json (dashboard state)
	c.store.RemoveAgent(agent.AgentID)

	log.Printf("[CLEANUP] Successfully removed orphan pending agent %s", agent.AgentID)
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
