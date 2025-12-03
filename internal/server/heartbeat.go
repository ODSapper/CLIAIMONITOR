package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

const (
	HeartbeatCheckInterval = 60 * time.Second
	StaleThreshold         = 120 * time.Second
)

// StartHeartbeatChecker runs a background goroutine that checks for stale agents
// and handles auto-respawn logic when agents die unexpectedly.
func (s *Server) StartHeartbeatChecker(ctx context.Context) {
	ticker := time.NewTicker(HeartbeatCheckInterval)
	defer ticker.Stop()

	log.Printf("[HEARTBEAT] Starting heartbeat checker (interval: %v, stale threshold: %v)", HeartbeatCheckInterval, StaleThreshold)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[HEARTBEAT] Heartbeat checker stopping")
			return
		case <-ticker.C:
			s.checkStaleAgents()
		}
	}
}

// checkStaleAgents scans all registered heartbeats for stale entries
func (s *Server) checkStaleAgents() {
	s.heartbeatMu.RLock()
	now := time.Now()

	// Collect stale agents (don't mutate map while iterating with read lock)
	var staleAgents []struct {
		agentID string
		info    *HeartbeatInfo
	}

	for agentID, info := range s.agentHeartbeats {
		if now.Sub(info.LastSeen) > StaleThreshold {
			// Copy the info to avoid race conditions
			infoCopy := *info
			staleAgents = append(staleAgents, struct {
				agentID string
				info    *HeartbeatInfo
			}{agentID, &infoCopy})
		}
	}
	s.heartbeatMu.RUnlock()

	// Handle each stale agent in separate goroutines
	for _, stale := range staleAgents {
		go s.handleStaleAgent(stale.agentID, stale.info)
	}
}

// handleStaleAgent processes a single stale agent with auto-respawn logic
func (s *Server) handleStaleAgent(agentID string, info *HeartbeatInfo) {
	log.Printf("[HEARTBEAT] Agent %s is stale (last seen: %v ago)", agentID, time.Since(info.LastSeen))

	// Step 1: Check if there was an approved stop_request for this agent
	stopRequests := s.store.GetPendingStopRequests()
	var approvedStop bool
	for _, req := range stopRequests {
		if req.AgentID == agentID && req.Reviewed && req.Approved {
			approvedStop = true
			log.Printf("[HEARTBEAT] Agent %s had approved stop request, cleaning up without respawn", agentID)
			break
		}
	}

	if approvedStop {
		// Clean removal - agent stopped gracefully
		s.cleanupStaleHeartbeat(agentID)
		return
	}

	// Step 2: Check if the process is actually dead (PID check as safety)
	agent := s.store.GetAgent(agentID)
	if agent != nil && agent.PID > 0 {
		// Try to find the process
		process, err := os.FindProcess(agent.PID)
		if err == nil {
			// On Windows, FindProcess always succeeds, so we need to test signal
			// Signal 0 doesn't kill but checks if process exists
			err = process.Signal(os.Signal(nil))
			if err == nil {
				// Process is still running - false alarm, reset heartbeat timer
				log.Printf("[HEARTBEAT] Agent %s appears stale but PID %d is still running, resetting timer", agentID, agent.PID)
				s.heartbeatMu.Lock()
				if hb, exists := s.agentHeartbeats[agentID]; exists {
					hb.LastSeen = time.Now()
				}
				s.heartbeatMu.Unlock()
				return
			}
		}

		// Process is dead - proceed to respawn
		log.Printf("[HEARTBEAT] Agent %s PID %d is confirmed dead", agentID, agent.PID)
	}

	// Step 3: Respawn agent with same config
	if info.ConfigName == "" || info.ProjectPath == "" {
		log.Printf("[HEARTBEAT] Cannot respawn agent %s - missing config_name or project_path", agentID)
		s.cleanupStaleHeartbeat(agentID)
		return
	}

	// Find agent config
	agentConfig := s.getAgentConfig(info.ConfigName)
	if agentConfig == nil {
		log.Printf("[HEARTBEAT] Cannot respawn agent %s - config %s not found", agentID, info.ConfigName)
		s.cleanupStaleHeartbeat(agentID)
		return
	}

	// Generate new agent ID for respawned agent
	newAgentID := s.spawner.GenerateAgentID(info.ConfigName)

	// Build respawn task
	respawnTask := info.CurrentTask
	if respawnTask == "" {
		respawnTask = "Continue previous work (respawned after crash)"
	} else {
		respawnTask = fmt.Sprintf("RESPAWNED: %s", respawnTask)
	}

	log.Printf("[HEARTBEAT] Respawning agent %s as %s to continue task: %s", agentID, newAgentID, respawnTask)

	// Spawn new agent with same config
	pid, err := s.spawner.SpawnAgent(*agentConfig, newAgentID, info.ProjectPath, respawnTask)
	if err != nil {
		log.Printf("[HEARTBEAT] Failed to respawn agent %s: %v", agentID, err)
		s.cleanupStaleHeartbeat(agentID)
		return
	}

	log.Printf("[HEARTBEAT] Successfully respawned agent %s as %s (PID: %d)", agentID, newAgentID, pid)

	// Clean up old heartbeat entry
	s.cleanupStaleHeartbeat(agentID)
}

// cleanupStaleHeartbeat removes a stale heartbeat entry from the map
func (s *Server) cleanupStaleHeartbeat(agentID string) {
	s.heartbeatMu.Lock()
	delete(s.agentHeartbeats, agentID)
	s.heartbeatMu.Unlock()
	log.Printf("[HEARTBEAT] Cleaned up heartbeat entry for agent %s", agentID)
}
