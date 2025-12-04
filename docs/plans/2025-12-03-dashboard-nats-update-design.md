# Dashboard NATS Update Design

**Date:** 2025-12-03
**Status:** Ready for Implementation
**Parallelization:** Tasks marked with same phase can run concurrently

---

## Overview

Update CLIAIMONITOR dashboard to work with NATS-based agent communication, removing legacy heartbeat system and supervisor concept. Captain becomes the orchestrator with full dashboard control available for human override.

### Key Principles

- **SQLite** = Persistence only (survives restarts, history, knowledge)
- **NATS** = All real-time messaging (bot-to-bot, bot-to-server)
- **Dashboard is authoritative** - Commands execute immediately, Captain adapts reactively
- **Escalations broadcast** - Both agent and Captain see human responses

---

## Phase 1: Code Removal (Parallel - 3 Agents)

### Task 1A: Remove HTTP Heartbeat System [Haiku]

**Files to modify:**

1. `internal/server/server.go`
   - Remove `agentHeartbeats map[string]*HeartbeatInfo` field
   - Remove `heartbeatMu sync.RWMutex` field
   - Remove `HeartbeatInfo` struct definition

2. `internal/server/handlers.go`
   - Remove `handleHeartbeat()` function (~115 lines)
   - Remove `handleGetHeartbeats()` function
   - Remove `handleDeleteHeartbeat()` function

3. `internal/server/server.go` (setupRoutes)
   - Remove route: `api.HandleFunc("/api/heartbeat", ...)`
   - Remove route: `api.HandleFunc("/api/heartbeats", ...)`
   - Remove route: `api.HandleFunc("/api/heartbeats/{id}", ...)`

4. `internal/memory/agent_control.go`
   - Remove `UpdateHeartbeat()` method - no longer needed for liveness

**Verification:** `go build ./...` succeeds, no references to removed functions

---

### Task 1B: Remove Supervisor Connected System [Haiku]

**Files to modify:**

1. `internal/types/types.go`
   - Remove `SupervisorConnected bool` from `DashboardState` struct

2. `internal/persistence/store.go`
   - Remove from interface: `SetSupervisorConnected(connected bool)`
   - Remove from interface: `IsSupervisorConnected() bool`
   - Remove implementations of both methods

3. `internal/server/server.go`
   - Remove all `s.store.SetSupervisorConnected()` calls (lines ~265, ~808, ~987)
   - Remove all `s.hub.BroadcastSupervisorStatus()` calls

4. `internal/server/hub.go`
   - Remove `BroadcastSupervisorStatus()` method

**Verification:** `go build ./...` succeeds, grep for "SupervisorConnected" returns nothing

---

### Task 1C: Update Test Files [Haiku]

**Files to modify:**

1. `internal/types/types_test.go`
   - Remove assertions checking `SupervisorConnected`

2. `internal/persistence/store_test.go`
   - Remove tests for `SetSupervisorConnected`, `IsSupervisorConnected`
   - Update mock implementations

3. `internal/server/hub_test.go`
   - Remove `BroadcastSupervisorStatus` test cases

4. `internal/server/cleanup_test.go`
   - Update mock store to remove supervisor methods

5. `internal/handlers/test.json`
   - Remove `"supervisor_connected": false` field

**Verification:** `go test ./...` passes

---

## Phase 2: NATS Infrastructure (Parallel - 2 Agents)

### Task 2A: Add Client ID Convention & Connection Tracking [Sonnet]

**Client ID Convention:**

| Component | Client ID Pattern | Example |
|-----------|------------------|---------|
| Captain | `captain` | `captain` |
| Server | `server` | `server` |
| Agents | `agent-{configName}-{seq}` | `agent-coder-1` |

**Files to modify:**

1. `internal/nats/client.go`
   - Add `ClientID` field to client config
   - Pass client ID to NATS connection options: `nats.Name(clientID)`
   - Add `GetClientID()` method

2. `internal/nats/server.go`
   - Add connection event callbacks to track connected clients
   - Maintain `connectedClients map[string]bool`
   - Add methods: `GetConnectedClients()`, `IsClientConnected(clientID string) bool`

3. `internal/server/server.go`
   - Pass `server` as client ID when creating NATS client
   - Subscribe to connection events from embedded server

4. `internal/agents/spawner.go`
   - Generate client ID using convention when spawning agents
   - Pass client ID to agent process via environment or flag

**New types in `internal/nats/messages.go`:**

```go
// ClientInfo represents a connected NATS client
type ClientInfo struct {
    ClientID    string    `json:"client_id"`
    ConnectedAt time.Time `json:"connected_at"`
}
```

**Verification:** Server logs show client IDs on connect/disconnect

---

### Task 2B: Add New State Fields & NATS Status [Sonnet]

**Files to modify:**

1. `internal/types/types.go`
   - Add to `DashboardState`:
     ```go
     NATSConnected    bool   `json:"nats_connected"`
     CaptainConnected bool   `json:"captain_connected"`
     CaptainStatus    string `json:"captain_status"` // idle, busy, error
     ```

2. `internal/persistence/store.go`
   - Add interface methods:
     ```go
     SetNATSConnected(connected bool)
     SetCaptainConnected(connected bool)
     SetCaptainStatus(status string)
     ```
   - Add implementations

3. `internal/server/server.go`
   - Initialize `NATSConnected` based on embedded server state
   - Update `CaptainConnected` when client `captain` connects/disconnects
   - Add periodic NATS health check (or rely on connection events)

4. `internal/server/nats_bridge.go`
   - Simplify `handleHeartbeat()` - just update store, no heartbeat map
   - Add subscription to `captain.status` subject
   - Update `CaptainStatus` field on captain status messages

**Verification:** `/api/state` returns new fields with correct values

---

## Phase 3: NATS Message Subjects (Sequential - Sonnet)

### Task 3: Define & Implement Message Subjects

**Subject Structure:**

| Subject | Publisher | Subscriber | Purpose |
|---------|-----------|------------|---------|
| `captain.status` | Captain | Server | Captain state (idle, busy, error) |
| `captain.commands` | Server | Captain | Dashboard commands to Captain |
| `agent.{id}.status` | Agent | Server, Captain | Agent state changes |
| `agent.{id}.commands` | Captain, Server | Agent | Commands to specific agent |
| `system.broadcast` | Server | All | System-wide announcements |
| `escalation.create` | Agent | Captain, Server | Agent raises question |
| `escalation.forward` | Captain | Server | Captain forwards to human |
| `escalation.response.{id}` | Server | Agent, Captain | Human's answer |

**Files to modify:**

1. `internal/nats/messages.go`
   - Add new message types:
     ```go
     type CaptainStatusMessage struct {
         Status    string    `json:"status"` // idle, busy, error
         CurrentOp string    `json:"current_op,omitempty"`
         QueueSize int       `json:"queue_size"`
         Timestamp time.Time `json:"timestamp"`
     }

     type CaptainCommandMessage struct {
         Type    string                 `json:"type"` // spawn_agent, kill_agent, submit_task, pause, resume
         Payload map[string]interface{} `json:"payload"`
         From    string                 `json:"from"` // client ID of sender
     }

     type EscalationCreateMessage struct {
         ID        string                 `json:"id"`
         AgentID   string                 `json:"agent_id"`
         Question  string                 `json:"question"`
         Context   map[string]interface{} `json:"context,omitempty"`
         Timestamp time.Time              `json:"timestamp"`
     }

     type EscalationForwardMessage struct {
         ID                string                 `json:"id"`
         AgentID           string                 `json:"agent_id"`
         Question          string                 `json:"question"`
         CaptainContext    string                 `json:"captain_context,omitempty"`
         CaptainRecommends string                 `json:"captain_recommends,omitempty"`
         Timestamp         time.Time              `json:"timestamp"`
     }

     type EscalationResponseMessage struct {
         ID        string    `json:"id"`
         Response  string    `json:"response"`
         From      string    `json:"from"` // "human" or client ID
         Timestamp time.Time `json:"timestamp"`
     }

     type SystemBroadcastMessage struct {
         Type    string                 `json:"type"` // shutdown, agent_killed, config_change
         Message string                 `json:"message"`
         Data    map[string]interface{} `json:"data,omitempty"`
         Timestamp time.Time            `json:"timestamp"`
     }
     ```

2. `internal/nats/handler.go`
   - Add subscriptions for new subjects
   - Add callbacks for captain.status, escalation.forward

3. `internal/server/nats_bridge.go`
   - Add handlers for new message types
   - Wire escalation.forward to dashboard alerts/UI

**Verification:** Can publish/subscribe to all subjects, messages route correctly

---

## Phase 4: Dashboard UI Updates (Parallel - 2 Agents)

### Task 4A: Update Status Indicators [Haiku]

**Files to modify:**

1. `web/index.html`
   - Replace supervisor status indicator with:
     ```html
     <div class="status-bar">
       <span class="status-indicator" id="nats-status">NATS: <span class="dot"></span></span>
       <span class="status-indicator" id="captain-status">Captain: <span class="dot"></span> <span class="status-text">--</span></span>
       <span class="status-indicator" id="agent-count">Agents: <span class="count">0</span></span>
     </div>
     ```

2. `web/style.css`
   - Add styles for new status bar
   - Status dot colors: green (connected), red (disconnected), yellow (degraded)
   - Remove `.heartbeat-ok`, `.heartbeat-caution`, `.heartbeat-warning` classes

3. `web/app.js`
   - Add `updateNATSStatus(connected)` method
   - Add `updateCaptainStatus(connected, status)` method
   - Add `updateAgentCount(count)` method
   - Update `render()` to call new status methods
   - Remove heartbeat age calculation and display logic

**Verification:** Status bar shows correct NATS/Captain/Agent status

---

### Task 4B: Update Agent Cards & Escalation Panel [Haiku]

**Files to modify:**

1. `web/app.js` (renderAgents method)
   - Remove heartbeat age display
   - Remove heartbeat status classes
   - Add NATS connection indicator (simple connected/disconnected)
   - Keep: status badge, current task, metrics

2. `web/index.html`
   - Update escalation panel to show:
     - Source agent ID
     - Question text
     - Captain's context/recommendation (if any)
     - Response input field
     - Submit button

3. `web/app.js`
   - Add `renderEscalations()` method
   - Handle `escalation_forward` WebSocket messages
   - Add `submitEscalationResponse(id, response)` method
   - Wire to API endpoint

4. `web/style.css`
   - Style escalation cards
   - Highlight pending escalations

**Verification:** Agent cards display without heartbeat, escalations show and can be responded to

---

## Phase 5: API Endpoints (Sequential - Sonnet)

### Task 5: Add Escalation & Captain Control Endpoints

**Files to modify:**

1. `internal/server/handlers.go`
   - Add `handleSubmitEscalationResponse()`:
     ```go
     // POST /api/escalation/{id}/respond
     // Body: { "response": "string" }
     // Publishes to NATS escalation.response.{id}
     ```
   - Add `handleSendCaptainCommand()`:
     ```go
     // POST /api/captain/command
     // Body: { "type": "spawn_agent|kill_agent|pause|resume", "payload": {...} }
     // Publishes to NATS captain.commands
     ```
   - Add `handleGetNATSStatus()`:
     ```go
     // GET /api/nats/status
     // Returns: { "connected": bool, "clients": [...] }
     ```

2. `internal/server/server.go` (setupRoutes)
   - Add route: `api.HandleFunc("/api/escalation/{id}/respond", ...)`
   - Add route: `api.HandleFunc("/api/captain/command", ...)`
   - Add route: `api.HandleFunc("/api/nats/status", ...)`

3. `internal/server/hub.go`
   - Add WebSocket message type for escalations: `WSTypeEscalation = "escalation"`
   - Add `BroadcastEscalation()` method

**Verification:** API endpoints work, escalation responses reach agents via NATS

---

## Phase 6: Integration & Testing (Sequential - Sonnet)

### Task 6: Integration Testing

1. **Start server** - Verify NATS embedded server starts, status shows connected

2. **Simulate Captain connect** - Use NATS client with ID `captain`, verify dashboard shows connected

3. **Simulate agent connect** - Use NATS client with ID `agent-coder-1`, verify agent count updates

4. **Test escalation flow:**
   - Publish to `escalation.create` from agent
   - Publish to `escalation.forward` from captain
   - Verify dashboard shows escalation
   - Submit response via API
   - Verify both agent and captain receive response on `escalation.response.{id}`

5. **Test captain commands:**
   - Send command via `/api/captain/command`
   - Verify captain receives on `captain.commands`

6. **Test override scenario:**
   - Dashboard kills agent directly
   - Verify `system.broadcast` sent
   - Verify Captain observes state change

**Verification:** All flows work end-to-end

---

## Task Assignment Summary

| Phase | Task | Model | Can Parallelize With |
|-------|------|-------|---------------------|
| 1 | 1A: Remove HTTP Heartbeat | Haiku | 1B, 1C |
| 1 | 1B: Remove Supervisor Connected | Haiku | 1A, 1C |
| 1 | 1C: Update Test Files | Haiku | 1A, 1B |
| 2 | 2A: Client ID & Connection Tracking | Sonnet | 2B |
| 2 | 2B: New State Fields & NATS Status | Sonnet | 2A |
| 3 | 3: Message Subjects | Sonnet | - |
| 4 | 4A: Status Indicators | Haiku | 4B |
| 4 | 4B: Agent Cards & Escalation Panel | Haiku | 4A |
| 5 | 5: API Endpoints | Sonnet | - |
| 6 | 6: Integration Testing | Sonnet | - |

**Optimal execution:**
- Phase 1: 3 Haiku agents in parallel
- Phase 2: 2 Sonnet agents in parallel
- Phase 3: 1 Sonnet agent (depends on Phase 2)
- Phase 4: 2 Haiku agents in parallel
- Phase 5: 1 Sonnet agent (depends on Phase 3, 4)
- Phase 6: 1 Sonnet agent (depends on all)

---

## Success Criteria

1. No references to heartbeat system in codebase
2. No references to supervisor_connected in codebase
3. Dashboard shows NATS, Captain, and Agent connection status
4. Escalations flow from agent → captain → dashboard → response broadcast
5. Dashboard can send commands to Captain via NATS
6. All existing tests pass
7. `go build ./...` succeeds
8. `go test ./...` succeeds
