# MCP SSE Protocol Fix Plan

**Date**: 2025-12-01
**Priority**: Critical - Blocking agent coordination
**Status**: Ready for execution

## Problem Statement

Claude Code agents cannot discover or use MCP tools because our SSE implementation doesn't match the expected protocol format. The agent reports: "The MCP tool mcp__cliaimonitor__register_agent is not available in my current tool set."

## Root Cause Analysis

Based on research of the MCP SSE specification (protocol version `2024-11-05`), our implementation has several protocol violations:

### Issue 1: Endpoint Event Format (Critical)

**Current** (internal/mcp/server.go:95-97):
```go
conn.Send("endpoint", map[string]string{
    "url": fmt.Sprintf("/mcp/message?agent_id=%s", agentID),
})
```

This sends:
```
event: endpoint
data: {"url":"/mcp/message?agent_id=test"}
```

**Required** (per MCP SSE spec):
```
event: endpoint
data: /mcp/messages/?session_id=<UUID>
```

The endpoint event data must be a **plain string URL**, not a JSON object.

### Issue 2: Session ID vs Agent ID

- MCP protocol uses `session_id` (UUID generated per SSE connection)
- We're using `agent_id` from HTTP headers
- The session_id should be generated server-side and passed to the client

### Issue 3: Messages Endpoint Path

- Current: `/mcp/message`
- Expected: `/mcp/messages/` (plural, with trailing slash)

### Issue 4: Response Routing via SSE

- Current: Responses go via HTTP response body in `ServeMessage`
- Expected: Responses should go through SSE stream with `event: message`

## Implementation Tasks

### Task 1: Add Session ID Generation

**File**: `internal/mcp/connections.go`

Add `SessionID` field to SSEConnection:
```go
type SSEConnection struct {
    AgentID   string
    SessionID string  // NEW: UUID for MCP protocol
    Writer    http.ResponseWriter
    // ... rest unchanged
}
```

Update `NewSSEConnection`:
```go
func NewSSEConnection(agentID string, w http.ResponseWriter) (*SSEConnection, error) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        return nil, fmt.Errorf("streaming not supported")
    }

    return &SSEConnection{
        AgentID:   agentID,
        SessionID: uuid.New().String(),  // NEW
        Writer:    w,
        Flusher:   flusher,
        Done:      make(chan struct{}),
        CreatedAt: time.Now(),
        LastPing:  time.Now(),
    }, nil
}
```

### Task 2: Fix Endpoint Event Format

**File**: `internal/mcp/connections.go`

Add new method `SendPlainData` that sends plain string (not JSON):
```go
// SendPlainData writes an SSE message with plain string data (not JSON)
func (c *SSEConnection) SendPlainData(event string, data string) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    // SSE format: event: <event>\ndata: <data>\n\n
    _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
    if err != nil {
        return err
    }

    c.Flusher.Flush()
    c.LastPing = time.Now()
    return nil
}
```

### Task 3: Update ServeSSE to Use Correct Protocol

**File**: `internal/mcp/server.go`

Update `ServeSSE` function (lines 66-116):
```go
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

    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Access-Control-Allow-Origin", "*")

    // Create connection (generates SessionID internally)
    conn, err := NewSSEConnection(agentID, w)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Register connection
    s.connections.Add(agentID, conn)
    defer s.connections.Remove(agentID)

    // Send endpoint message - PLAIN STRING, NOT JSON
    // Format: /mcp/messages/?session_id=<UUID>
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
            // Send keepalive ping (can remain as JSON)
            if err := conn.Send("ping", map[string]int64{"time": time.Now().Unix()}); err != nil {
                return
            }
        }
    }
}
```

### Task 4: Add Session-to-Agent Mapping

**File**: `internal/mcp/connections.go`

Add session lookup to ConnectionManager:
```go
type ConnectionManager struct {
    mu           sync.RWMutex
    connections  map[string]*SSEConnection  // by agentID
    sessions     map[string]*SSEConnection  // by sessionID - NEW
    onConnect    func(agentID string)
    onDisconnect func(agentID string)
}

func NewConnectionManager() *ConnectionManager {
    return &ConnectionManager{
        connections: make(map[string]*SSEConnection),
        sessions:    make(map[string]*SSEConnection),  // NEW
    }
}

func (m *ConnectionManager) Add(agentID string, conn *SSEConnection) {
    m.mu.Lock()
    if existing, ok := m.connections[agentID]; ok {
        delete(m.sessions, existing.SessionID)  // Clean up old session
        existing.Close()
    }
    m.connections[agentID] = conn
    m.sessions[conn.SessionID] = conn  // NEW
    m.mu.Unlock()

    if m.onConnect != nil {
        m.onConnect(agentID)
    }
}

func (m *ConnectionManager) Remove(agentID string) {
    m.mu.Lock()
    if conn, ok := m.connections[agentID]; ok {
        delete(m.sessions, conn.SessionID)  // Clean up session
        conn.Close()
        delete(m.connections, agentID)
    }
    m.mu.Unlock()

    if m.onDisconnect != nil {
        m.onDisconnect(agentID)
    }
}

// GetBySession looks up connection by session ID - NEW
func (m *ConnectionManager) GetBySession(sessionID string) *SSEConnection {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.sessions[sessionID]
}
```

### Task 5: Update Message Handler for Session ID

**File**: `internal/mcp/server.go`

Update `ServeMessage` to use session_id and route responses via SSE:
```go
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
        w.WriteHeader(http.StatusAccepted)  // Acknowledge receipt
        return
    }

    // Handle request - get agentID from connection
    resp := s.handleRequest(conn.AgentID, &req)

    // Send response via SSE stream (not HTTP response)
    conn.SendResponse(resp)

    // HTTP response just acknowledges receipt
    w.WriteHeader(http.StatusAccepted)
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
```

### Task 6: Update Route Registration

**File**: `internal/server/routes.go` (or wherever routes are defined)

Change route from `/mcp/message` to `/mcp/messages/`:
```go
// Old:
// router.HandleFunc("/mcp/message", s.mcp.ServeMessage)

// New:
router.HandleFunc("/mcp/messages/", s.mcp.ServeMessage)
```

## Testing Plan

1. **Build and start server**:
   ```bash
   go build -o cliaimonitor.exe ./cmd/main.go
   ./cliaimonitor.exe --no-supervisor
   ```

2. **Test SSE endpoint format**:
   ```bash
   curl -N -H "X-Agent-ID: test" http://localhost:3000/mcp/sse
   ```
   Expected: `event: endpoint\ndata: /mcp/messages/?session_id=<uuid>\n\n`

3. **Test message endpoint**:
   ```bash
   # Get session_id from SSE connection first
   curl -X POST "http://localhost:3000/mcp/messages/?session_id=<uuid>" \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
   ```

4. **Spawn test agent and verify MCP tool discovery**

## Rollback Plan

If issues occur, revert the three files:
- `internal/mcp/connections.go`
- `internal/mcp/server.go`
- Route registration file

## Success Criteria

- [ ] SSE endpoint event sends plain string URL
- [ ] Session ID generated per connection
- [ ] Messages routed via SSE stream
- [ ] Agents can discover tools via `tools/list`
- [ ] Agents can call `register_agent` successfully
