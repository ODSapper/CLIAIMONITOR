package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/CLIAIMONITOR/internal/agents"
	"github.com/CLIAIMONITOR/internal/types"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// handleWebSocket upgrades to WebSocket and manages connection
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &Client{
		hub:  s.hub,
		conn: conn,
		send: make(chan []byte, 256),
	}

	s.hub.Register(client)

	// Send current state immediately
	state := s.store.GetState()
	data, _ := json.Marshal(types.WSMessage{
		Type: types.WSTypeStateUpdate,
		Data: state,
	})
	client.send <- data

	go client.readPump()
	go client.writePump()
}

// handleGetState returns current dashboard state
func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	state := s.store.GetState()
	s.respondJSON(w, state)
}

// handleGetProjects returns available projects for spawning agents
func (s *Server) handleGetProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := agents.GetAllProjects(s.projectsConfig)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "Failed to load projects")
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"projects": projects,
	})
}

// handleSpawnAgent spawns a new agent
func (s *Server) handleSpawnAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConfigName  string `json:"config_name"`
		ProjectPath string `json:"project_path"`
		Task        string `json:"task"` // Optional initial task for agent
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Find agent config
	agentConfig := s.getAgentConfig(req.ConfigName)
	if agentConfig == nil {
		s.respondError(w, http.StatusBadRequest, "Unknown agent type")
		return
	}

	// Generate agent ID
	num := s.store.GetNextAgentNumber(req.ConfigName)
	agentID := req.ConfigName + formatAgentNumber(num)

	// Default project path
	projectPath := req.ProjectPath
	if projectPath == "" {
		projectPath = s.basePath
	}

	// Build initial prompt if task provided (single line to avoid PowerShell issues)
	// Include explicit instructions about MCP tools and autonomy
	initialPrompt := ""
	if req.Task != "" {
		initialPrompt = fmt.Sprintf(
			"CRITICAL: You have MCP tools from the 'cliaimonitor' server. "+
				"Use these MCP tools for communication - NOT PowerShell scripts. "+
				"Available MCP tools: mcp__cliaimonitor__register_agent, mcp__cliaimonitor__report_status, mcp__cliaimonitor__request_human_input, mcp__cliaimonitor__request_stop_approval. "+
				"FIRST: Call mcp__cliaimonitor__register_agent with agent_id='%s' and role='Go Developer'. "+
				"THEN: Work on your task: %s. "+
				"Work autonomously. Do NOT ask clarifying questions.",
			agentID, req.Task)
	} else {
		initialPrompt = fmt.Sprintf(
			"CRITICAL: You have MCP tools from the 'cliaimonitor' server. "+
				"Use these MCP tools for communication - NOT PowerShell scripts. "+
				"FIRST: Call mcp__cliaimonitor__register_agent with agent_id='%s' and role='Go Developer'. "+
				"THEN: Wait for instructions. Work autonomously.",
			agentID)
	}

	// Spawn agent with initial prompt
	pid, err := s.spawner.SpawnAgent(*agentConfig, agentID, projectPath, initialPrompt)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Create agent record
	agent := &types.Agent{
		ID:          agentID,
		ConfigName:  req.ConfigName,
		Role:        agentConfig.Role,
		Model:       agentConfig.Model,
		Color:       agentConfig.Color,
		Status:      types.StatusStarting,
		PID:         pid,
		ProjectPath: projectPath,
		SpawnedAt:   time.Now(),
		LastSeen:    time.Now(),
	}

	s.store.AddAgent(agent)
	s.broadcastState()

	s.respondJSON(w, agent)
}

// handleStopAgent stops an agent
func (s *Server) handleStopAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	if err := s.spawner.StopAgent(agentID); err != nil {
		// Still remove from store even if process kill fails
	}

	// Cleanup generated config and prompt files
	s.spawner.CleanupAgentFiles(agentID)

	s.store.RemoveAgent(agentID)
	s.metrics.RemoveAgent(agentID)
	s.broadcastState()

	s.respondJSON(w, map[string]bool{"success": true})
}

// handleGracefulStopAgent requests graceful shutdown of an agent
func (s *Server) handleGracefulStopAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	// Mark agent for shutdown
	now := time.Now()
	s.store.RequestAgentShutdown(agentID, now)
	s.broadcastState()

	// Start a goroutine to force-kill after timeout
	go func() {
		time.Sleep(60 * time.Second)

		// Check if agent is still running
		state := s.store.GetState()
		if agent, ok := state.Agents[agentID]; ok && agent.ShutdownRequested {
			// Agent didn't stop gracefully, force kill
			s.spawner.StopAgent(agentID)
			s.spawner.CleanupAgentFiles(agentID)
			s.store.RemoveAgent(agentID)
			s.metrics.RemoveAgent(agentID)
			s.broadcastState()
		}
	}()

	s.respondJSON(w, map[string]interface{}{
		"success": true,
		"message": "Graceful shutdown requested. Agent will be force-stopped in 60 seconds if it doesn't exit.",
	})
}

// handleAnswerHumanInput answers a human input request
func (s *Server) handleAnswerHumanInput(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID := vars["id"]

	var req struct {
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate answer
	if len(req.Answer) == 0 {
		s.respondError(w, http.StatusBadRequest, "Answer cannot be empty")
		return
	}
	if len(req.Answer) > 10000 {
		s.respondError(w, http.StatusBadRequest, "Answer exceeds maximum length of 10000 characters")
		return
	}

	// Verify request exists and is pending
	state := s.store.GetState()
	humanReq := state.HumanRequests[requestID]
	if humanReq == nil {
		s.respondError(w, http.StatusNotFound, "Request not found")
		return
	}
	if humanReq.Answered {
		s.respondError(w, http.StatusConflict, "Request already answered")
		return
	}

	// Update store
	s.store.AnswerHumanRequest(requestID, req.Answer)

	// Notify agent via MCP (humanReq already validated above)
	if humanReq.AgentID != "" {
		s.mcp.NotifyAgent(humanReq.AgentID, "human_input_response", map[string]string{
			"request_id": requestID,
			"answer":     req.Answer,
		})
	}

	s.broadcastState()
	s.respondJSON(w, map[string]bool{"success": true})
}

// handleAcknowledgeAlert acknowledges an alert
func (s *Server) handleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	alertID := vars["id"]

	s.store.AcknowledgeAlert(alertID)
	s.broadcastState()

	s.respondJSON(w, map[string]bool{"success": true})
}

// handleClearAllAlerts clears all alerts
func (s *Server) handleClearAllAlerts(w http.ResponseWriter, r *http.Request) {
	s.store.ClearAllAlerts()
	s.broadcastState()

	s.respondJSON(w, map[string]bool{"success": true})
}

// handleUpdateThresholds updates alert thresholds
func (s *Server) handleUpdateThresholds(w http.ResponseWriter, r *http.Request) {
	var thresholds types.AlertThresholds
	if err := json.NewDecoder(r.Body).Decode(&thresholds); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate threshold values
	if err := thresholds.Validate(); err != nil {
		s.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.store.SetThresholds(thresholds)
	s.alerts.SetThresholds(thresholds)
	s.broadcastState()

	s.respondJSON(w, map[string]bool{"success": true})
}

// handleResetMetrics resets metrics history
func (s *Server) handleResetMetrics(w http.ResponseWriter, r *http.Request) {
	s.store.ResetMetricsHistory()
	s.metrics.ResetHistory()
	s.broadcastState()

	s.respondJSON(w, map[string]bool{"success": true})
}

// handleHealthCheck returns health status of the instance
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	state := s.store.GetState()

	// Count connected agents
	connectedAgents := 0
	for _, agent := range state.Agents {
		if agent.Status == types.StatusConnected || agent.Status == types.StatusWorking {
			connectedAgents++
		}
	}

	// Count active alerts
	activeAlerts := 0
	for _, alert := range state.Alerts {
		if !alert.Acknowledged {
			activeAlerts++
		}
	}

	health := map[string]interface{}{
		"status":         "ok",
		"uptime_seconds": int(time.Since(s.startTime).Seconds()),
		"version":        "1.0.0",
		"pid":            os.Getpid(),
		"port":           s.port,
		"agents": map[string]int{
			"total":     len(state.Agents),
			"connected": connectedAgents,
		},
		"alerts": map[string]int{
			"total":  len(state.Alerts),
			"active": activeAlerts,
		},
	}

	s.respondJSON(w, health)
}

// handleShutdown initiates a graceful shutdown of the server
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	// Only allow from localhost
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if host != "127.0.0.1" && host != "::1" && host != "[::1]" {
		s.respondError(w, http.StatusForbidden, "Shutdown can only be requested from localhost")
		return
	}

	s.respondJSON(w, map[string]string{
		"status":  "shutting_down",
		"message": "Graceful shutdown initiated",
	})

	// Trigger shutdown in goroutine to allow response to be sent
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
}

// Notification Handlers

func (s *Server) handleGetBanner(w http.ResponseWriter, r *http.Request) {
	bannerState := s.notifications.GetBannerState()
	s.respondJSON(w, bannerState)
}

func (s *Server) handleClearBanner(w http.ResponseWriter, r *http.Request) {
	if err := s.notifications.ClearAlert(); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to clear banner: %v", err))
		return
	}

	s.respondJSON(w, map[string]string{
		"status": "cleared",
	})
}

// Stop Request Handlers

// handleGetStopRequests returns pending stop approval requests
func (s *Server) handleGetStopRequests(w http.ResponseWriter, r *http.Request) {
	pending := s.store.GetPendingStopRequests()
	s.respondJSON(w, map[string]interface{}{
		"stop_requests": pending,
	})
}

// handleRespondStopRequest responds to a stop approval request
func (s *Server) handleRespondStopRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID := vars["id"]

	var req struct {
		Approved bool   `json:"approved"`
		Response string `json:"response"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Verify request exists and is pending
	state := s.store.GetState()
	stopReq := state.StopRequests[requestID]
	if stopReq == nil {
		s.respondError(w, http.StatusNotFound, "Stop request not found")
		return
	}
	if stopReq.Reviewed {
		s.respondError(w, http.StatusConflict, "Stop request already reviewed")
		return
	}

	// Update store
	s.store.RespondStopRequest(requestID, req.Approved, req.Response, "human")

	// Notify agent via MCP
	if stopReq.AgentID != "" {
		s.mcp.NotifyAgent(stopReq.AgentID, "stop_approval_response", map[string]interface{}{
			"request_id": requestID,
			"approved":   req.Approved,
			"response":   req.Response,
		})
	}

	s.broadcastState()
	s.respondJSON(w, map[string]interface{}{
		"success":  true,
		"approved": req.Approved,
	})
}

// Stats Handler

// handleGetStats returns session statistics
func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	state := s.store.GetState()
	s.respondJSON(w, state.SessionStats)
}

// Agent Cleanup Handlers

// handleCleanupAgents removes stale disconnected agents and kills their processes
func (s *Server) handleCleanupAgents(w http.ResponseWriter, r *http.Request) {
	// Get current state to find disconnected agents
	state := s.store.GetState()
	removedCount := 0

	for agentID, agent := range state.Agents {
		// Only clean up disconnected agents
		if agent.Status == types.StatusDisconnected {
			// Kill the process if it's still running
			if err := s.spawner.StopAgent(agentID); err != nil {
				// Log but continue - process may already be dead
			}

			// Clean up config and prompt files
			s.spawner.CleanupAgentFiles(agentID)

			// Remove from store and metrics
			s.store.RemoveAgent(agentID)
			s.metrics.RemoveAgent(agentID)
			removedCount++
		}
	}

	s.broadcastState()

	s.respondJSON(w, map[string]interface{}{
		"success": true,
		"removed": removedCount,
		"message": fmt.Sprintf("Removed %d stale agent(s)", removedCount),
	})
}

// Helper functions
func (s *Server) respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func formatAgentNumber(n int) string {
	return fmt.Sprintf("%03d", n)
}
