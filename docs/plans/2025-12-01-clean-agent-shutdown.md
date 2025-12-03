# Clean Agent Shutdown Feature

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow graceful agent shutdown that gives agents time to wrap up and commit before exiting

**Architecture:** Add shutdown_requested flag to agent state. Agents check this flag on each MCP call. Dashboard shows "Stopping..." status. If agent doesn't stop within timeout, force kill.

**Tech Stack:** Go backend, JavaScript frontend

---

## Problem

Current "Stop" button kills agent process immediately. This can cause:
- Lost uncommitted work
- Corrupted files
- No summary of what was accomplished

## Solution Design

### Flow
1. User clicks "Graceful Stop" button
2. Backend sets `shutdown_requested: true` for agent
3. Dashboard shows "Stopping..." with countdown timer
4. Agent's next MCP call includes `shutdown_requested: true` in response
5. Agent wraps up, commits, calls `request_stop_approval`
6. If agent doesn't stop in 60 seconds, force kill

### Why This Works
Since agents call MCP tools regularly (status updates, human input checks), they'll see the shutdown flag within seconds. No persistent connection needed.

---

### Task 1: Add ShutdownRequested Field to Agent Type

**Files:**
- Modify: `internal/types/types.go`

**Step 1: Add field to Agent struct**

Find the `Agent` struct and add:

```go
type Agent struct {
    ID              string    `json:"id"`
    ConfigName      string    `json:"config_name"`
    Role            string    `json:"role"`
    Model           string    `json:"model"`
    Color           string    `json:"color"`
    Status          string    `json:"status"`
    PID             int       `json:"pid"`
    ProjectPath     string    `json:"project_path"`
    SpawnedAt       time.Time `json:"spawned_at"`
    LastSeen        time.Time `json:"last_seen"`
    CurrentTask     string    `json:"current_task,omitempty"`
    ShutdownRequested bool    `json:"shutdown_requested"`           // NEW
    ShutdownRequestedAt *time.Time `json:"shutdown_requested_at,omitempty"` // NEW
}
```

**Step 2: Verify build**

Run: `go build -o cliaimonitor.exe ./cmd/cliaimonitor/main.go`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/types/types.go
git commit -m "feat: add ShutdownRequested field to Agent type"
```

---

### Task 2: Add API Endpoint for Graceful Stop

**Files:**
- Modify: `internal/server/handlers.go`

**Step 1: Add handleGracefulStop function**

Add this new handler after `handleStopAgent`:

```go
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
```

**Step 2: Verify build**

Run: `go build -o cliaimonitor.exe ./cmd/cliaimonitor/main.go`
Expected: Build fails (RequestAgentShutdown doesn't exist yet)

---

### Task 3: Add Store Method for Shutdown Request

**Files:**
- Modify: `internal/store/store.go`

**Step 1: Add RequestAgentShutdown method**

Add this method to the Store:

```go
// RequestAgentShutdown marks an agent for graceful shutdown
func (s *Store) RequestAgentShutdown(agentID string, requestTime time.Time) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if agent, ok := s.state.Agents[agentID]; ok {
        agent.ShutdownRequested = true
        agent.ShutdownRequestedAt = &requestTime
        agent.Status = "stopping"
        s.state.Agents[agentID] = agent
        s.persist()
    }
}
```

**Step 2: Verify build**

Run: `go build -o cliaimonitor.exe ./cmd/cliaimonitor/main.go`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/store/store.go internal/server/handlers.go
git commit -m "feat: add graceful shutdown endpoint and store method"
```

---

### Task 4: Register Route

**Files:**
- Modify: `internal/server/server.go` or `internal/server/routes.go`

**Step 1: Add route for graceful stop**

Find where routes are registered and add:

```go
router.HandleFunc("/api/agents/{id}/graceful-stop", s.handleGracefulStopAgent).Methods("POST")
```

**Step 2: Verify build**

Run: `go build -o cliaimonitor.exe ./cmd/cliaimonitor/main.go`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/server/server.go
git commit -m "feat: register graceful-stop route"
```

---

### Task 5: Include Shutdown Flag in MCP Responses

**Files:**
- Modify: `internal/mcp/server.go`

**Step 1: Update handleRequest to include shutdown flag**

Modify `handleRequest` to return shutdown status in results:

```go
func (s *Server) handleRequest(agentID string, req *types.MCPRequest) types.MCPResponse {
    // ... existing switch/case logic ...

    resp := // existing response handling

    // Add shutdown flag to result if applicable
    if result, ok := resp.Result.(map[string]interface{}); ok {
        // Check if agent is marked for shutdown
        if s.isAgentShutdownRequested != nil && s.isAgentShutdownRequested(agentID) {
            result["_shutdown_requested"] = true
        }
    }

    return resp
}
```

**Step 2: Add callback for checking shutdown status**

Add to Server struct:

```go
type Server struct {
    connections              *ConnectionManager
    tools                    *ToolRegistry
    isAgentShutdownRequested func(agentID string) bool // NEW
}
```

And add setter:

```go
func (s *Server) SetShutdownChecker(checker func(agentID string) bool) {
    s.isAgentShutdownRequested = checker
}
```

**Step 3: Wire up in main server setup**

In server initialization, add:

```go
mcpServer.SetShutdownChecker(func(agentID string) bool {
    state := store.GetState()
    if agent, ok := state.Agents[agentID]; ok {
        return agent.ShutdownRequested
    }
    return false
})
```

**Step 4: Commit**

```bash
git add internal/mcp/server.go internal/server/server.go
git commit -m "feat: include shutdown flag in MCP responses"
```

---

### Task 6: Update Dashboard UI

**Files:**
- Modify: `web/app.js`
- Modify: `web/style.css`

**Step 1: Add graceful stop button**

In `renderAgents`, update the agent-actions section:

```javascript
<div class="agent-actions">
    ${agent.shutdown_requested ? `
        <span class="shutdown-countdown" data-started="${agent.shutdown_requested_at}">
            Stopping... <span class="countdown">60s</span>
        </span>
        <button class="btn btn-danger" onclick="dashboard.stopAgent('${agent.id}')">Force Stop</button>
    ` : `
        <button class="btn btn-warning" onclick="dashboard.gracefulStopAgent('${agent.id}')" title="Request graceful shutdown">
            Stop
        </button>
        <button class="btn btn-danger btn-small" onclick="dashboard.stopAgent('${agent.id}')" title="Force kill immediately">
            Kill
        </button>
    `}
</div>
```

**Step 2: Add gracefulStopAgent method**

```javascript
async gracefulStopAgent(agentId) {
    try {
        await fetch(`/api/agents/${agentId}/graceful-stop`, { method: 'POST' });
    } catch (error) {
        console.error('Failed to request graceful stop:', error);
    }
}
```

**Step 3: Add CSS for stopping state**

```css
.agent-status.stopping {
    background: rgba(236, 201, 75, 0.3);
    color: var(--accent-yellow);
    animation: pulse 1s infinite;
}

.shutdown-countdown {
    font-size: 0.8rem;
    color: var(--accent-yellow);
    margin-right: 0.5rem;
}
```

**Step 4: Rebuild and verify**

Run: `go build -o cliaimonitor.exe ./cmd/cliaimonitor/main.go`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add web/app.js web/style.css
git commit -m "feat: add graceful stop UI with countdown"
```

---

### Task 7: Update Agent System Prompt

**Files:**
- Modify: Agent system prompts to mention shutdown handling

The agent's system prompt should include instructions like:

```
## Shutdown Protocol
When you receive `_shutdown_requested: true` in any MCP response:
1. Finish your current atomic operation (don't leave broken state)
2. Commit any pending changes with a summary message
3. Call `request_stop_approval` with work summary
4. Exit gracefully

You have ~60 seconds before force termination.
```

---

## Summary

| Component | Change |
|-----------|--------|
| `types.go` | Add `ShutdownRequested`, `ShutdownRequestedAt` fields |
| `store.go` | Add `RequestAgentShutdown()` method |
| `handlers.go` | Add `handleGracefulStopAgent()` with timeout |
| `server.go` | Register route, wire up shutdown checker |
| `mcp/server.go` | Include `_shutdown_requested` in responses |
| `app.js` | Add graceful stop button, countdown display |
| `style.css` | Add stopping state styles |

## Testing

1. Spawn an agent
2. Click "Stop" (should be graceful by default)
3. Verify agent receives shutdown flag on next MCP call
4. Verify countdown shows in UI
5. If agent exits cleanly, it's removed from dashboard
6. If timeout expires, agent is force-killed
