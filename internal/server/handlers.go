package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

// handleSpawnAgent spawns a new agent
func (s *Server) handleSpawnAgent(w http.ResponseWriter, r *http.Request) {
	// Check supervisor is connected
	if !s.store.IsSupervisorConnected() {
		s.respondError(w, http.StatusServiceUnavailable, "Supervisor not connected")
		return
	}

	var req struct {
		ConfigName  string `json:"config_name"`
		ProjectPath string `json:"project_path"`
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

	// Spawn agent
	pid, err := s.spawner.SpawnAgent(*agentConfig, agentID, projectPath)
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

// handleCheckin records human check-in
func (s *Server) handleCheckin(w http.ResponseWriter, r *http.Request) {
	s.store.RecordHumanCheckin()
	s.broadcastState()

	s.respondJSON(w, map[string]bool{"success": true})
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
