package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/CLIAIMONITOR/internal/types"
)

// Server implements MCP over HTTP (POST-only JSON-RPC)
type Server struct {
	tools      *ToolRegistry
	onToolCall func(agentID string, toolName string)
}

// NewServer creates a new MCP server
func NewServer() *Server {
	return &Server{
		tools: NewToolRegistry(),
	}
}

// SetToolCallCallback sets callback for when a tool is called (for metrics)
func (s *Server) SetToolCallCallback(callback func(agentID string, toolName string)) {
	s.onToolCall = callback
}

// RegisterTool adds a tool to the server
func (s *Server) RegisterTool(tool ToolDefinition) {
	s.tools.Register(tool)
}

// ServeHTTP handles MCP requests (POST-only JSON-RPC)
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get agent ID from header (required)
	agentID := r.Header.Get("X-Agent-ID")
	if agentID == "" {
		agentID = r.URL.Query().Get("agent_id")
	}
	if agentID == "" {
		http.Error(w, "X-Agent-ID header or agent_id query param required", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
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
		s.sendJSONError(w, nil, -32700, "Parse error")
		return
	}

	// Handle request
	resp := s.handleRequest(agentID, &req)

	// If request is a notification (no ID), return 202 Accepted
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
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

	return types.MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": resultText,
				},
			},
		},
	}
}
