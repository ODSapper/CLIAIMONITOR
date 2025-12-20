package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/CLIAIMONITOR/internal/types"
)

// Server implements MCP over SSE
type Server struct {
	connections              *ConnectionManager
	tools                    *ToolRegistry
	isAgentShutdownRequested func(agentID string) bool
	onToolCall               func(agentID string, toolName string)
}

// NewServer creates a new MCP server
func NewServer() *Server {
	return &Server{
		connections: NewConnectionManager(),
		tools:       NewToolRegistry(),
	}
}

// SetConnectionCallbacks sets connect/disconnect callbacks
func (s *Server) SetConnectionCallbacks(onConnect, onDisconnect func(agentID string)) {
	s.connections.SetCallbacks(onConnect, onDisconnect)
}

// SetShutdownChecker sets callback to check if agent should shutdown
func (s *Server) SetShutdownChecker(checker func(agentID string) bool) {
	s.isAgentShutdownRequested = checker
}

// SetToolCallCallback sets callback for when a tool is called (for metrics)
func (s *Server) SetToolCallCallback(callback func(agentID string, toolName string)) {
	s.onToolCall = callback
}

// RegisterTool adds a tool to the server
func (s *Server) RegisterTool(tool ToolDefinition) {
	s.tools.Register(tool)
}

// GetConnectedAgents returns connected agent IDs
func (s *Server) GetConnectedAgents() []string {
	return s.connections.GetConnectedAgentIDs()
}

// SendToAgent sends a message to a specific agent
func (s *Server) SendToAgent(agentID string, resp types.MCPResponse) error {
	conn := s.connections.Get(agentID)
	if conn == nil {
		return fmt.Errorf("agent %s not connected", agentID)
	}
	return conn.SendResponse(resp)
}

// NotifyAgent sends a notification to a specific agent
func (s *Server) NotifyAgent(agentID string, method string, params interface{}) error {
	conn := s.connections.Get(agentID)
	if conn == nil {
		return fmt.Errorf("agent %s not connected", agentID)
	}
	return conn.SendNotification(method, params)
}

// Broadcast sends a notification to all agents
func (s *Server) Broadcast(method string, params interface{}) {
	s.connections.Broadcast(method, params)
}

// ServeSSE handles SSE connections from agents (GET) and JSON-RPC messages (POST)
func (s *Server) ServeSSE(w http.ResponseWriter, r *http.Request) {
	// Get agent ID from header or query param
	agentID := r.Header.Get("X-Agent-ID")
	if agentID == "" {
		agentID = r.URL.Query().Get("agent_id")
	}
	if agentID == "" {
		http.Error(w, "X-Agent-ID header or agent_id query param required", http.StatusBadRequest)
		return
	}

	// Handle POST - JSON-RPC message (Claude MCP client sends POST to same endpoint)
	if r.Method == http.MethodPost {
		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		// Parse JSON-RPC request
		var req types.MCPRequest
		if err := json.Unmarshal(body, &req); err != nil {
			s.sendError(w, nil, -32700, "Parse error")
			return
		}

		// Handle request using agent ID from header
		resp := s.handleRequest(agentID, &req)

		// Check if we have an active SSE connection
		conn := s.connections.Get(agentID)
		if conn != nil {
			// Send response via SSE stream (MCP SSE protocol)
			if err := conn.SendResponse(resp); err != nil {
				http.Error(w, "failed to send response", http.StatusInternalServerError)
				return
			}
			// Return 202 Accepted to acknowledge receipt
			w.WriteHeader(http.StatusAccepted)
		} else {
			// No active SSE connection - send response directly as JSON (fallback mode)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		}
		return
	}

	// Handle GET - establish SSE stream
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create connection
	conn, err := NewSSEConnection(agentID, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Register connection
	s.connections.Add(agentID, conn)
	defer s.connections.Remove(agentID)

	// Send initial endpoint message (MCP SSE protocol)
	endpointURL := fmt.Sprintf("/mcp/messages/?session_id=%s", conn.SessionID)
	if err := conn.SendPlainData("endpoint", endpointURL); err != nil {
		return
	}

	// Keep connection alive with periodic pings
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-conn.Done:
			return
		case <-r.Context().Done():
			return
		case <-ticker.C:
			// Send keepalive ping
			if err := conn.Send("ping", map[string]int64{"time": time.Now().Unix()}); err != nil {
				return
			}
		}
	}
}

// ServeMessage handles POST messages from agents
func (s *Server) ServeMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	// Get session ID from query param (per MCP SSE protocol)
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	// Look up connection by session
	conn := s.connections.GetBySession(sessionID)
	if conn == nil {
		http.Error(w, "invalid session", http.StatusUnauthorized)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Parse JSON-RPC request
	var req types.MCPRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.sendErrorToSSE(conn, nil, -32700, "Parse error")
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Handle request - get agentID from connection
	resp := s.handleRequest(conn.AgentID, &req)

	// Send response via SSE stream (MCP SSE protocol)
	if err := conn.SendResponse(resp); err != nil {
		http.Error(w, "failed to send response", http.StatusInternalServerError)
		return
	}

	// Return 202 Accepted to acknowledge receipt
	w.WriteHeader(http.StatusAccepted)
}

// handleRequest processes an MCP request
func (s *Server) handleRequest(agentID string, req *types.MCPRequest) types.MCPResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(agentID, req)
	default:
		return types.MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &types.MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

// handleInitialize processes initialize request
func (s *Server) handleInitialize(req *types.MCPRequest) types.MCPResponse {
	return types.MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    "CLIAIMONITOR",
				"version": "1.0.0",
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]bool{
					"listChanged": false,
				},
			},
		},
	}
}

// handleToolsList returns available tools
func (s *Server) handleToolsList(req *types.MCPRequest) types.MCPResponse {
	return types.MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": s.tools.List(),
		},
	}
}

// handleToolsCall executes a tool
func (s *Server) handleToolsCall(agentID string, req *types.MCPRequest) types.MCPResponse {
	// Parse params
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return types.MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &types.MCPError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	toolName, _ := params["name"].(string)
	toolArgs, _ := params["arguments"].(map[string]interface{})

	// Notify callback for metrics tracking
	if s.onToolCall != nil && toolName != "" {
		s.onToolCall(agentID, toolName)
	}

	if toolName == "" {
		return types.MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &types.MCPError{
				Code:    -32602,
				Message: "Tool name required",
			},
		}
	}

	// Execute tool
	result, err := s.tools.Execute(toolName, agentID, toolArgs)
	if err != nil {
		return types.MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &types.MCPError{
				Code:    -32000,
				Message: err.Error(),
			},
		}
	}

	// Format result as text content
	resultText := fmt.Sprintf("%v", result)
	if jsonBytes, err := json.Marshal(result); err == nil {
		resultText = string(jsonBytes)
	}

	resultMap := map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": resultText,
			},
		},
	}

	// Add shutdown flag if applicable
	if s.isAgentShutdownRequested != nil && s.isAgentShutdownRequested(agentID) {
		resultMap["_shutdown_requested"] = true
	}

	return types.MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultMap,
	}
}

// sendError sends an error response
func (s *Server) sendError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := types.MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &types.MCPError{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// sendErrorToSSE sends error via SSE stream
func (s *Server) sendErrorToSSE(conn *SSEConnection, id interface{}, code int, message string) {
	resp := types.MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &types.MCPError{
			Code:    code,
			Message: message,
		},
	}
	conn.SendResponse(resp)
}
