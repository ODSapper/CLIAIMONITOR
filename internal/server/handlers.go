package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/CLIAIMONITOR/internal/agents"
	"github.com/CLIAIMONITOR/internal/types"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// MaxPayloadSize defines the maximum size for request payloads (1MB)
// This prevents DoS attacks via large payloads
const MaxPayloadSize = 1 * 1024 * 1024 // 1MB

// Agent shutdown timeout constants
const (
	// GracefulStopTimeout is the duration to wait for graceful agent shutdown before force-killing
	GracefulStopTimeout = 60 * time.Second
)

// AllowedOrigins contains the list of allowed WebSocket origins
// Default: localhost only. Can be configured via CLIAIMONITOR_ALLOWED_ORIGINS env var
// Example: CLIAIMONITOR_ALLOWED_ORIGINS=http://myhost.local:3000,https://dashboard.example.com
var allowedOrigins = initAllowedOrigins()

func initAllowedOrigins() []string {
	// Always allow localhost on common ports
	defaults := []string{
		"http://localhost:3000",
		"http://localhost:8080",
		"http://127.0.0.1:3000",
		"http://127.0.0.1:8080",
	}

	// Add origins from environment variable
	envOrigins := os.Getenv("CLIAIMONITOR_ALLOWED_ORIGINS")
	if envOrigins != "" {
		for _, origin := range strings.Split(envOrigins, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				defaults = append(defaults, origin)
			}
		}
	}

	return defaults
}

// checkWebSocketOrigin validates the Origin header for WebSocket connections
// to prevent CSRF attacks. Allows localhost origins and configured domains.
func checkWebSocketOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")

	// No origin header means same-origin request (browser doesn't send Origin
	// for same-origin requests in some cases)
	if origin == "" {
		return true
	}

	// Parse the origin URL
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}

	// Allow all localhost origins (any port)
	host := originURL.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}

	// Check against configured allowed origins
	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}

		// Parse allowed origin for more flexible matching
		allowedURL, err := url.Parse(allowed)
		if err != nil {
			continue
		}

		// Match host (without port requirement if port not specified in allowed)
		if originURL.Hostname() == allowedURL.Hostname() {
			// If allowed origin has a port, require exact match
			if allowedURL.Port() != "" {
				if originURL.Port() == allowedURL.Port() && originURL.Scheme == allowedURL.Scheme {
					return true
				}
			} else {
				// No port in allowed origin, just match host and scheme
				if originURL.Scheme == allowedURL.Scheme {
					return true
				}
			}
		}
	}

	return false
}

// limitRequestSize limits the request body size to prevent DoS via large payloads
// Returns the limited body reader to use for decoding
func limitRequestSize(r *http.Request, maxSize int64) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxSize)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: checkWebSocketOrigin,
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
		send: make(chan []byte, WebSocketBufferSize),
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
	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	var req struct {
		ConfigName  string `json:"config_name"`
		ProjectPath string `json:"project_path"`
		Task        string `json:"task"` // Optional initial task for agent
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate ConfigName
	if req.ConfigName == "" {
		s.respondError(w, http.StatusBadRequest, "ConfigName is required")
		return
	}

	// Validate ConfigName length (prevent arbitrarily long inputs)
	if len(req.ConfigName) > 50 {
		s.respondError(w, http.StatusBadRequest, "ConfigName too long (max 50 characters)")
		return
	}

	// Validate ProjectPath (optional, but if provided, validate it)
	if req.ProjectPath != "" {
		cleanPath := filepath.Clean(req.ProjectPath)
		// Reject path traversal attempts in relative paths
		if !filepath.IsAbs(cleanPath) && strings.Contains(cleanPath, "..") {
			s.respondError(w, http.StatusBadRequest, "Invalid project path: path traversal not allowed")
			return
		}
		// Verify the path exists and is a directory
		info, err := os.Stat(cleanPath)
		if err != nil || !info.IsDir() {
			s.respondError(w, http.StatusBadRequest, "Invalid project path: directory does not exist")
			return
		}
	}

	// Validate Task length (if provided)
	if len(req.Task) > 5000 {
		s.respondError(w, http.StatusBadRequest, "Task description too long (max 5000 characters)")
		return
	}

	// Find agent config
	agentConfig := s.getAgentConfig(req.ConfigName)
	if agentConfig == nil {
		s.respondError(w, http.StatusBadRequest, "Unknown agent type")
		return
	}

	// Generate team-compatible agent ID using spawner's method
	// This ensures consistent ID format: team-{type}{seq:03d}
	agentID := s.spawner.GenerateAgentID(req.ConfigName)

	// Default project path
	projectPath := req.ProjectPath
	if projectPath == "" {
		projectPath = s.basePath
	}

	// Build initial prompt - agent should use MCP tools and start working
	// Use the MCP server name that was configured for this agent
	mcpServerName := fmt.Sprintf("cliaimonitor-%s", agentID)
	mcpInstructions := fmt.Sprintf(
		"You are agent '%s' with role '%s'. You have an MCP server '%s' connected. "+
			"Use the available MCP tools to communicate and coordinate. ",
		agentID, agentConfig.Role, mcpServerName)

	initialPrompt := ""
	if req.Task != "" {
		initialPrompt = mcpInstructions +
			fmt.Sprintf("TASK: %s. Work autonomously. Do NOT ask clarifying questions.", req.Task)
	} else {
		initialPrompt = mcpInstructions +
			"Use wait_for_events tool to wait for instructions."
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

	// Agent will register itself via MCP when it connects
	log.Printf("[SPAWN] Agent %s created, awaiting MCP registration", agentID)

	s.broadcastState()

	s.respondJSON(w, agent)
}

// handleStopAgent stops an agent
func (s *Server) handleStopAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	// Validate agent ID
	if !isValidAgentID(agentID) {
		s.respondError(w, http.StatusBadRequest, "Invalid agent ID")
		return
	}

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

	// Validate agent ID
	if !isValidAgentID(agentID) {
		s.respondError(w, http.StatusBadRequest, "Invalid agent ID")
		return
	}

	// Mark agent for shutdown
	now := time.Now()
	s.store.RequestAgentShutdown(agentID, now)
	s.broadcastState()

	// Start a goroutine to force-kill after timeout
	go func() {
		timer := time.NewTimer(GracefulStopTimeout)
		defer timer.Stop()

		select {
		case <-s.stopChan:
			// Server shutting down, skip force-kill
			return
		case <-timer.C:
			// Timeout reached, check if agent is still running
			state := s.store.GetState()
			if agent, ok := state.Agents[agentID]; ok && agent.ShutdownRequested {
				// Agent didn't stop gracefully, force kill
				s.spawner.StopAgent(agentID)
				s.spawner.CleanupAgentFiles(agentID)
				s.store.RemoveAgent(agentID)
				s.metrics.RemoveAgent(agentID)
				s.broadcastState()
			}
		}
	}()

	s.respondJSON(w, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Graceful shutdown requested. Agent will be force-stopped in %d seconds if it doesn't exit.", int(GracefulStopTimeout.Seconds())),
	})
}

// handleAnswerHumanInput answers a human input request
func (s *Server) handleAnswerHumanInput(w http.ResponseWriter, r *http.Request) {
	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	vars := mux.Vars(r)
	requestID := vars["id"]

	// Validate requestID (prevent potential NoSQL/SQL/path injection)
	if requestID == "" || len(requestID) > 100 {
		s.respondError(w, http.StatusBadRequest, "Invalid request ID")
		return
	}

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

	// Optional: Basic sanitization or content validation
	if hasUnsafeContent(req.Answer) {
		s.respondError(w, http.StatusBadRequest, "Answer contains unsafe content")
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

// handleGetAlerts returns active alerts
func (s *Server) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	alerts := s.store.GetActiveAlerts()
	s.respondJSON(w, map[string]interface{}{
		"count":  len(alerts),
		"alerts": alerts,
	})
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
	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

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

	// Check memory database health
	memoryHealth := map[string]interface{}{
		"connected": false,
	}
	if s.memDB != nil {
		if health, err := s.memDB.Health(); err == nil {
			memoryHealth = map[string]interface{}{
				"connected":         health.Connected,
				"schema_version":    health.SchemaVersion,
				"agent_count":       health.AgentCount,
				"task_count":        health.TaskCount,
				"learning_count":    health.LearningCount,
				"context_count":     health.ContextCount,
				"last_context_save": health.LastContextSave,
				"db_size_bytes":     health.DBSizeBytes,
			}
		}
	}

	health := map[string]interface{}{
		"status":         "ok",
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
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
		"captain_connected": state.CaptainConnected,
		"memory_db":         memoryHealth,
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

	// Signal shutdown via channel (allows main.go to do proper cleanup)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		select {
		case <-ctx.Done():
			select {
			case <-s.ShutdownChan:
				// Already closed
			default:
				close(s.ShutdownChan)
			}
		case <-s.stopChan:
			// Server already shutting down
			return
		}
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
	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

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

// handleGetAgentsFromDB returns agent data from the in-memory store
func (s *Server) handleGetAgentsFromDB(w http.ResponseWriter, r *http.Request) {
	// NOTE: Agent control database layer has been removed
	// Return agents from in-memory JSONStore instead
	state := s.store.GetState()

	// Convert state.Agents map to slice for consistent API response
	agents := make([]interface{}, 0, len(state.Agents))
	for _, agent := range state.Agents {
		agents = append(agents, agent)
	}

	s.respondJSON(w, map[string]interface{}{
		"agents": agents,
	})
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
	w.Header().Set("X-Error-Type", "validation")
	w.WriteHeader(status)

	// Log error for server-side tracking (optional)
	log.Printf("[HTTP_ERROR] Status %d: %s", status, message)

	// More detailed error response
	errorResp := map[string]interface{}{
		"error":      message,
		"error_code": fmt.Sprintf("ERR_%d", status),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	json.NewEncoder(w).Encode(errorResp)
}

func formatAgentNumber(n int) string {
	return fmt.Sprintf("%03d", n)
}

// hasUnsafeContent performs basic sanitization check
func hasUnsafeContent(s string) bool {
	// Check for potential script tags or HTML injection
	unsafePatterns := []string{"<script", "javascript:", "onerror", "onload", "eval("}

	lowerStr := strings.ToLower(s)
	for _, pattern := range unsafePatterns {
		if strings.Contains(lowerStr, pattern) {
			return true
		}
	}
	return false
}

// isValidAgentID validates an agent ID format
func isValidAgentID(id string) bool {
	// Example validation: must be non-empty, shorter than 100 chars
	// Optionally can add more specific regex validation based on your ID format
	return id != "" && len(id) <= 100 &&
		// Optional: strict validation for team-{type}{seq:03d} format
		// Uncomment and modify as needed for your specific format
		// regexp.MustCompile(`^team-[a-z]+\d{3}$`).MatchString(id)
		true
}

// Escalation & Captain Control Handlers

// handleSubmitEscalationResponse handles POST /api/escalation/{id}/respond
func (s *Server) handleSubmitEscalationResponse(w http.ResponseWriter, r *http.Request) {
	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	vars := mux.Vars(r)
	escalationID := vars["id"]

	// Validate escalation ID
	if escalationID == "" || len(escalationID) > 100 {
		s.respondError(w, http.StatusBadRequest, "Invalid escalation ID")
		return
	}

	var req struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate response
	if req.Response == "" {
		s.respondError(w, http.StatusBadRequest, "Response cannot be empty")
		return
	}

	// Validate response length
	if len(req.Response) > 5000 {
		s.respondError(w, http.StatusBadRequest, "Response is too long (max 5000 characters)")
		return
	}

	// Optional: Content safety check
	if hasUnsafeContent(req.Response) {
		s.respondError(w, http.StatusBadRequest, "Response contains unsafe content")
		return
	}

	// Log escalation response
	log.Printf("[ESCALATION] Response for %s: %s", escalationID, req.Response)

	s.respondJSON(w, map[string]interface{}{
		"success": true,
		"message": "Escalation response recorded via MCP",
	})
}

// handleSendCaptainCommand handles POST /api/captain/command
// Broadcasts command via MCP event bus
func (s *Server) handleSendCaptainCommand(w http.ResponseWriter, r *http.Request) {
	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	var req struct {
		Type    string                 `json:"type"`
		Payload map[string]interface{} `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate command type
	validTypes := map[string]bool{
		"spawn_agent": true,
		"kill_agent":  true,
		"pause":       true,
		"resume":      true,
		"message":     true, // Allow human messages to Captain
	}
	if !validTypes[req.Type] {
		s.respondError(w, http.StatusBadRequest, "Invalid command type (must be spawn_agent, kill_agent, pause, resume, or message)")
		return
	}

	// Log Captain command
	log.Printf("[CAPTAIN] Command received: %s, payload: %v", req.Type, req.Payload)

	// Handle message type - log as activity
	if req.Type == "message" {
		if text, ok := req.Payload["text"].(string); ok {
			s.store.AddActivity(&types.ActivityLog{
				ID:        fmt.Sprintf("activity-%d", time.Now().UnixNano()),
				AgentID:   "Human",
				Action:    "message_to_captain",
				Details:   text,
				Timestamp: time.Now(),
			})
		}
	}

	s.respondJSON(w, map[string]interface{}{
		"success": true,
		"message": "Command published to Captain",
		"type":    req.Type,
	})
}


// Captain Terminal Supervisor Handlers

// handleCaptainTerminalStatus returns the current status of the Captain terminal process
func (s *Server) handleCaptainTerminalStatus(w http.ResponseWriter, r *http.Request) {
	if s.captainSupervisor == nil {
		s.respondJSON(w, map[string]interface{}{
			"status":      "not_configured",
			"can_restart": false,
			"message":     "Captain supervisor not configured",
		})
		return
	}

	info := s.captainSupervisor.GetInfo()
	s.respondJSON(w, info)
}

// handleCaptainTerminalRestart manually restarts the Captain terminal
func (s *Server) handleCaptainTerminalRestart(w http.ResponseWriter, r *http.Request) {
	if s.captainSupervisor == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Captain supervisor not configured")
		return
	}

	info := s.captainSupervisor.GetInfo()
	if !info.CanRestart {
		s.respondError(w, http.StatusConflict, fmt.Sprintf("Cannot restart Captain (status: %s)", info.Status))
		return
	}

	if err := s.captainSupervisor.Restart(); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to restart Captain: %v", err))
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"success": true,
		"message": "Captain restart initiated",
	})
}

// Captain Context Handlers

// handleGetCaptainContext returns all Captain context entries
func (s *Server) handleGetCaptainContext(w http.ResponseWriter, r *http.Request) {
	if s.memDB == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Memory database not available")
		return
	}

	contexts, err := s.memDB.GetAllContext()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get context: %v", err))
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"contexts": contexts,
		"count":    len(contexts),
	})
}

// handleSetCaptainContext sets a context entry
func (s *Server) handleSetCaptainContext(w http.ResponseWriter, r *http.Request) {
	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	if s.memDB == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Memory database not available")
		return
	}

	var req struct {
		Key         string `json:"key"`
		Value       string `json:"value"`
		Priority    int    `json:"priority"`
		MaxAgeHours int    `json:"max_age_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Key == "" {
		s.respondError(w, http.StatusBadRequest, "Key is required")
		return
	}

	// Default priority to 5 if not specified
	if req.Priority == 0 {
		req.Priority = 5
	}

	if err := s.memDB.SetContext(req.Key, req.Value, req.Priority, req.MaxAgeHours); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to set context: %v", err))
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"success": true,
		"key":     req.Key,
	})
}

// handleDeleteCaptainContext deletes a context entry
func (s *Server) handleDeleteCaptainContext(w http.ResponseWriter, r *http.Request) {
	if s.memDB == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Memory database not available")
		return
	}

	vars := mux.Vars(r)
	key := vars["key"]

	if err := s.memDB.DeleteContext(key); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete context: %v", err))
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"success": true,
		"key":     key,
	})
}

// handleGetCaptainContextSummary returns formatted context for Captain startup
func (s *Server) handleGetCaptainContextSummary(w http.ResponseWriter, r *http.Request) {
	if s.memDB == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Memory database not available")
		return
	}

	// Clean expired context first
	cleaned, _ := s.memDB.CleanExpiredContext()

	contexts, err := s.memDB.GetAllContext()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get context: %v", err))
		return
	}

	// Build formatted summary
	summary := ""
	if len(contexts) > 0 {
		summary = "=== CAPTAIN CONTEXT (from memory.db) ===\n\n"
		for _, ctx := range contexts {
			summary += fmt.Sprintf("[%s] (priority: %d)\n%s\n\n", ctx.Key, ctx.Priority, ctx.Value)
		}
	}

	s.respondJSON(w, map[string]interface{}{
		"summary":         summary,
		"context_count":   len(contexts),
		"expired_cleaned": cleaned,
	})
}

// handleGetMetricsByModel returns aggregated metrics per model
func (s *Server) handleGetMetricsByModel(w http.ResponseWriter, r *http.Request) {
	if s.memDB == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Memory database not available")
		return
	}

	modelFilter := r.URL.Query().Get("model")

	metrics, err := s.memDB.GetMetricsByModel(modelFilter)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get metrics: %v", err))
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"metrics": metrics,
		"count":   len(metrics),
	})
}

// handleGetMetricsByAgentType returns aggregated metrics by agent type (captain, sgt, spawned_window, subagent)
func (s *Server) handleGetMetricsByAgentType(w http.ResponseWriter, r *http.Request) {
	if s.memDB == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Memory database not available")
		return
	}

	metrics, err := s.memDB.GetMetricsByAgentType()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get metrics: %v", err))
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"metrics": metrics,
		"count":   len(metrics),
	})
}

// handleGetMetricsByAgent returns per-agent metrics breakdown
func (s *Server) handleGetMetricsByAgent(w http.ResponseWriter, r *http.Request) {
	if s.memDB == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Memory database not available")
		return
	}

	metrics, err := s.memDB.GetMetricsByAgent()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get metrics: %v", err))
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"metrics": metrics,
		"count":   len(metrics),
	})
}

// handleCaptainHealth returns Captain health status
func (s *Server) handleCaptainHealth(w http.ResponseWriter, r *http.Request) {
	// Check memory database health
	memoryConnected := false
	memorySchemaVersion := 0
	if s.memDB != nil {
		if health, err := s.memDB.Health(); err == nil {
			memoryConnected = health.Connected
			memorySchemaVersion = health.SchemaVersion
		}
	}

	response := map[string]interface{}{
		"captain_connected":     s.store.GetState().CaptainConnected,
		"status":                s.store.GetState().CaptainStatus,
		"memory_db_connected":   memoryConnected,
		"memory_schema_version": memorySchemaVersion,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetLeaderboard returns agent quality scores for the leaderboard
func (s *Server) handleGetLeaderboard(w http.ResponseWriter, r *http.Request) {
	if s.memDB == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Memory database not available")
		return
	}

	// Get role filter from query param
	role := r.URL.Query().Get("role")

	// Get limit (default 20)
	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && l > 0 {
			if limit > 100 {
				limit = 100
			}
		}
	}

	scores, err := s.memDB.GetAgentLeaderboard(role, limit)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get leaderboard: %v", err))
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"leaderboard": scores,
		"count":       len(scores),
		"role_filter": role,
	})
}

// handleGetReviewBoards returns active review boards
func (s *Server) handleGetReviewBoards(w http.ResponseWriter, r *http.Request) {
	if s.memDB == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Memory database not available")
		return
	}

	// For now, return empty list since GetActiveReviewBoards may not exist
	// This endpoint is for the dashboard to show active reviews
	s.respondJSON(w, map[string]interface{}{
		"review_boards": []interface{}{},
		"count":         0,
	})
}

// handleGetDefectCategories returns valid defect categories
func (s *Server) handleGetDefectCategories(w http.ResponseWriter, r *http.Request) {
	if s.memDB == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Memory database not available")
		return
	}

	categories, err := s.memDB.GetDefectCategories()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get categories: %v", err))
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"categories": categories,
		"count":      len(categories),
	})
}

// handleDebugWezterm handles GET /api/debug/wezterm
// Tests wezterm cli from server's context
func (s *Server) handleDebugWezterm(w http.ResponseWriter, r *http.Request) {
	result := map[string]interface{}{}

	// Check if wezterm.exe is in PATH
	weztermPath, lookErr := exec.LookPath("wezterm.exe")
	result["wezterm_path"] = weztermPath
	result["lookup_error"] = ""
	if lookErr != nil {
		result["lookup_error"] = lookErr.Error()
	}

	// Try to list panes
	cmd := exec.Command("wezterm.exe", "cli", "list")
	output, err := cmd.CombinedOutput()
	result["list_output"] = string(output)
	result["list_error"] = ""
	if err != nil {
		result["list_error"] = err.Error()
	}

	// Check environment
	result["wezterm_socket"] = os.Getenv("WEZTERM_UNIX_SOCKET")
	result["wezterm_pane"] = os.Getenv("WEZTERM_PANE")

	s.respondJSON(w, result)
}
