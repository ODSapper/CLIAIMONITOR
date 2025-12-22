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
	connectionLimiter        *ConnectionLimiter
	isAgentShutdownRequested func(agentID string) bool
	onToolCall               func(agentID string, toolName string)
}

// NewServer creates a new MCP server
func NewServer() *Server {
	return &Server{
		connections:       NewConnectionManager(),
		tools:             NewToolRegistry(),
		connectionLimiter: NewConnectionLimiter(MaxConnectionsPerAgent, MaxTotalConnections),
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

// ServeStreamableHTTP handles the new MCP Streamable HTTP transport (2025-03-26 spec).
// This is the recommended transport, replacing the deprecated SSE transport.
// Single endpoint handles both GET (SSE stream) and POST (JSON-RPC requests).
// Uses Mcp-Session-Id header for session management.
func (s *Server) ServeStreamableHTTP(w http.ResponseWriter, r *http.Request) {
	// Get agent ID from header (required for Streamable HTTP)
	agentID := r.Header.Get("X-Agent-ID")
	if agentID == "" {
		agentID = r.URL.Query().Get("agent_id")
	}
	if agentID == "" {
		http.Error(w, "X-Agent-ID header or agent_id query param required", http.StatusBadRequest)
		return
	}

	sessionID := r.Header.Get("Mcp-Session-Id")

	switch r.Method {
	case http.MethodPost:
		s.handleStreamableHTTPPost(w, r, agentID, sessionID)
	case http.MethodGet:
		s.handleStreamableHTTPGet(w, r, agentID, sessionID)
	case http.MethodDelete:
		s.handleStreamableHTTPDelete(w, r, agentID, sessionID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStreamableHTTPPost handles POST requests for Streamable HTTP transport
func (s *Server) handleStreamableHTTPPost(w http.ResponseWriter, r *http.Request, agentID, sessionID string) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Parse JSON-RPC request
	var req types.MCPRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.sendJSONError(w, nil, -32700, "Parse error")
		return
	}

	// Handle initialize specially - assign session ID
	if req.Method == "initialize" {
		resp := s.handleInitialize(&req)

		// Generate new session ID for initialization
		newSessionID := fmt.Sprintf("%d", time.Now().UnixNano())

		// Set session ID header
		w.Header().Set("Mcp-Session-Id", newSessionID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// For non-initialize requests, session ID is optional but useful for tracking
	// Handle request
	resp := s.handleRequest(agentID, &req)

	// Check Accept header for response type preference
	accept := r.Header.Get("Accept")

	// If request only contains notifications (no response needed)
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Send JSON response (simpler than SSE for single request/response)
	if accept == "" || accept == "application/json" || accept == "*/*" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// If client prefers SSE, check for active connection
	if accept == "text/event-stream" {
		conn := s.connections.Get(agentID)
		if conn != nil {
			if err := conn.SendResponse(resp); err != nil {
				http.Error(w, "failed to send response", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusAccepted)
			return
		}
	}

	// Default to JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// handleStreamableHTTPGet handles GET requests (SSE stream) for Streamable HTTP transport
func (s *Server) handleStreamableHTTPGet(w http.ResponseWriter, r *http.Request, agentID, sessionID string) {
	// Check connection limits before accepting new connection
	if !s.connectionLimiter.TryAcquire(agentID) {
		s.connectionLimiter.HandleLimitExceeded(w, agentID)
		return
	}

	// This is for establishing an SSE stream for server->client notifications
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create connection
	conn, err := NewSSEConnection(agentID, w)
	if err != nil {
		// Release the connection slot on error
		s.connectionLimiter.Release(agentID)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If no session ID provided, generate one
	if sessionID == "" {
		sessionID = conn.SessionID
	}

	// Set session ID in response header
	w.Header().Set("Mcp-Session-Id", sessionID)

	// Register connection
	s.connections.Add(agentID, conn)
	defer func() {
		s.connections.Remove(agentID)
		s.connectionLimiter.Release(agentID)
	}()

	// Mark connection as active after registration
	conn.SetActive()

	// Keep connection alive with periodic pings
	// Use 15s interval - some proxies/firewalls drop connections after 30s idle
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Ensure goroutine cleanup on exit
	done := make(chan struct{})
	defer close(done)

	for {
		select {
		case <-conn.Done:
			return
		case <-r.Context().Done():
			// Client disconnected - ensure connection is closed
			conn.Close()
			return
		case <-done:
			return
		case <-ticker.C:
			// Check if connection is still alive before sending
			if conn.IsClosed() {
				return
			}
			if err := conn.Send("ping", map[string]int64{"time": time.Now().Unix()}); err != nil {
				conn.Close()
				return
			}
		}
	}
}

// handleStreamableHTTPDelete handles DELETE requests (session termination) for Streamable HTTP transport
func (s *Server) handleStreamableHTTPDelete(w http.ResponseWriter, r *http.Request, agentID, sessionID string) {
	if sessionID == "" {
		http.Error(w, "Mcp-Session-Id required for session termination", http.StatusBadRequest)
		return
	}

	// Remove connection if exists
	conn := s.connections.GetBySession(sessionID)
	if conn != nil {
		s.connections.Remove(conn.AgentID)
	}

	w.WriteHeader(http.StatusOK)
}

// sendJSONError sends a JSON-RPC error response
func (s *Server) sendJSONError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := types.MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &types.MCPError{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
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
	// Check connection limits before accepting new connection
	if !s.connectionLimiter.TryAcquire(agentID) {
		s.connectionLimiter.HandleLimitExceeded(w, agentID)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create connection
	conn, err := NewSSEConnection(agentID, w)
	if err != nil {
		// Release the connection slot on error
		s.connectionLimiter.Release(agentID)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Register connection
	s.connections.Add(agentID, conn)
	defer func() {
		s.connections.Remove(agentID)
		s.connectionLimiter.Release(agentID)
	}()

	// Mark connection as active after registration
	conn.SetActive()

	// Send initial endpoint message (MCP SSE protocol)
	endpointURL := fmt.Sprintf("/mcp/messages/?session_id=%s", conn.SessionID)
	if err := conn.SendPlainData("endpoint", endpointURL); err != nil {
		conn.Close()
		return
	}

	// Keep connection alive with periodic pings
	// Use 15s interval - some proxies/firewalls drop connections after 30s idle
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Ensure goroutine cleanup on exit
	done := make(chan struct{})
	defer close(done)

	for {
		select {
		case <-conn.Done:
			return
		case <-r.Context().Done():
			// Client disconnected - ensure connection is closed
			conn.Close()
			return
		case <-done:
			return
		case <-ticker.C:
			// Check if connection is still alive before sending
			if conn.IsClosed() {
				return
			}
			// Send keepalive ping
			if err := conn.Send("ping", map[string]int64{"time": time.Now().Unix()}); err != nil {
				conn.Close()
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
