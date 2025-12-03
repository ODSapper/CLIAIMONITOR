# DB-Based Agent Control & Auto-Cleanup Design

**Date:** 2025-12-02
**Status:** Approved
**Authors:** Captain + Human

## Problem Statement

1. Dashboard shows stale disconnected agents that require manual cleanup
2. Agent-to-DB communication is inefficient (MCP overhead, high token cost)
3. Captain lacks direct control over agent lifecycle via durable state

## Solution: Hybrid DB + MCP Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                      SQLite DB (data/memory.db)                      │
│                        SOURCE OF TRUTH                               │
├─────────────────────────────────────────────────────────────────────┤
│  agent_control:  agent_id, heartbeat, shutdown_flag, status         │
│  task_queue:     id, assigned_to, status, priority, description     │
└─────────────────────────────────────────────────────────────────────┘
         ▲                              ▲                    │
         │ sqlite3                      │ sqlite3            │ monitor
         │ (background script)          │ (background script)│ (30s interval)
         │                              │                    ▼
    ┌────┴────┐                   ┌─────┴─────┐       ┌──────────────┐
    │ Agent 1 │                   │  Agent 2  │       │   Captain    │
    └────┬────┘                   └─────┬─────┘       │   Server     │
         │                              │             └──────┬───────┘
         │         MCP (urgent only)    │                    │
         └──────────────────────────────┴────────────────────┘
```

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Agent DB access | Direct sqlite3 | Most token efficient |
| Heartbeat method | Background PowerShell script | Zero agent token cost |
| Polling interval | 30 seconds | Balanced responsiveness |
| Stale threshold | 120 seconds | Conservative, fewer false positives |
| MCP usage | Urgent notifications only | Shutdown, high-priority tasks, agent signals |

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS agent_control (
    agent_id TEXT PRIMARY KEY,
    config_name TEXT NOT NULL,
    role TEXT,
    project_path TEXT,
    pid INTEGER,

    -- Heartbeat & Status
    status TEXT DEFAULT 'starting',
    heartbeat_at DATETIME,
    current_task TEXT,

    -- Control Flags
    shutdown_flag INTEGER DEFAULT 0,
    shutdown_reason TEXT,
    priority_override INTEGER,

    -- Lifecycle
    spawned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    stopped_at DATETIME,
    stop_reason TEXT,

    -- Metadata
    model TEXT,
    color TEXT
);

CREATE INDEX idx_agent_heartbeat ON agent_control(heartbeat_at);
CREATE INDEX idx_agent_status ON agent_control(status);
```

---

# Implementation Plan (Parallelized)

## Wave 1: Foundation (Parallel - 2 agents)

### Task 1A: Database Layer
**Agent:** SNTGreen (DB specialist)
**Estimated tokens:** ~5000
**Dependencies:** None

1. Create migration file `internal/memory/migrations/003_agent_control.sql`
2. Create `internal/memory/agent_control.go` with interface:
   ```go
   type AgentControlRepository interface {
       // Write operations
       RegisterAgent(agent *AgentControl) error
       UpdateHeartbeat(agentID string) error
       UpdateStatus(agentID, status, currentTask string) error
       SetShutdownFlag(agentID string, reason string) error
       ClearShutdownFlag(agentID string) error
       MarkStopped(agentID, reason string) error
       RemoveAgent(agentID string) error

       // Read operations
       GetAgent(agentID string) (*AgentControl, error)
       GetAllAgents() ([]*AgentControl, error)
       GetStaleAgents(threshold time.Duration) ([]*AgentControl, error)
       GetAgentsByStatus(status string) ([]*AgentControl, error)
       CheckShutdownFlag(agentID string) (bool, string, error)
   }
   ```
3. Implement all methods on `SQLiteMemoryDB`
4. Update `internal/memory/db.go` to run migration
5. Write unit tests in `internal/memory/agent_control_test.go`

**Verification:**
```bash
go test ./internal/memory/... -v -run AgentControl
```

---

### Task 1B: Heartbeat Script
**Agent:** SNTGreen (PowerShell)
**Estimated tokens:** ~2000
**Dependencies:** None

1. Create `scripts/agent-heartbeat.ps1`:
   ```powershell
   param(
       [Parameter(Mandatory=$true)][string]$AgentID,
       [string]$DBPath = "data/memory.db",
       [int]$IntervalSeconds = 30
   )

   $ErrorActionPreference = "SilentlyContinue"

   while ($true) {
       # Write heartbeat
       sqlite3 $DBPath "UPDATE agent_control SET heartbeat_at = datetime('now') WHERE agent_id = '$AgentID'"

       # Check shutdown flag
       $shutdown = sqlite3 $DBPath "SELECT shutdown_flag FROM agent_control WHERE agent_id = '$AgentID'"

       if ($shutdown -eq "1") {
           $reason = sqlite3 $DBPath "SELECT shutdown_reason FROM agent_control WHERE agent_id = '$AgentID'"
           Set-Content -Path "data/shutdown-$AgentID.flag" -Value $reason
           exit 0
       }

       Start-Sleep -Seconds $IntervalSeconds
   }
   ```

2. Create test script `scripts/test-heartbeat.ps1` to verify functionality

**Verification:**
```powershell
# Manual test
.\scripts\agent-heartbeat.ps1 -AgentID "test-agent" -IntervalSeconds 5
# Check DB
sqlite3 data/memory.db "SELECT * FROM agent_control WHERE agent_id='test-agent'"
```

---

## Wave 2: Integration (Parallel - 3 agents, after Wave 1)

### Task 2A: Auto-Cleanup Service
**Agent:** SNTGreen
**Estimated tokens:** ~3000
**Dependencies:** Task 1A (agent_control.go)

1. Create `internal/server/cleanup.go`:
   ```go
   type CleanupService struct {
       memDB    memory.MemoryDB
       store    persistence.Store
       hub      *Hub
       interval time.Duration
       staleThreshold time.Duration
   }

   func NewCleanupService(memDB memory.MemoryDB, store persistence.Store, hub *Hub) *CleanupService
   func (c *CleanupService) Start(ctx context.Context)
   func (c *CleanupService) cleanupStaleAgents()
   func (c *CleanupService) RunOnce() int  // For manual trigger
   ```

2. Cleanup logic:
   - Query agents with heartbeat > 120 seconds old
   - Mark as "dead" in DB
   - Kill PID if still running
   - Remove from state.json
   - Broadcast dashboard update

3. Write tests in `internal/server/cleanup_test.go`

**Verification:**
```bash
go test ./internal/server/... -v -run Cleanup
```

---

### Task 2B: Spawner Integration
**Agent:** SNTGreen
**Estimated tokens:** ~2500
**Dependencies:** Task 1A, Task 1B

1. Modify `internal/agents/spawner.go`:
   - Add `memDB memory.MemoryDB` field to Spawner struct
   - Update `NewSpawner()` to accept memDB parameter
   - In `SpawnAgent()`:
     - Register agent in DB via `memDB.RegisterAgent()`
     - Spawn heartbeat script alongside Claude terminal
     - Track heartbeat script PID for cleanup
   - Add `StopAgent()` method:
     - Set shutdown flag in DB
     - Wait for graceful stop or force kill
     - Kill heartbeat script
     - Mark stopped in DB

2. Update spawner tests

**Verification:**
```bash
go test ./internal/agents/... -v
```

---

### Task 2C: MCP Signal Tool
**Agent:** SNTGreen
**Estimated tokens:** ~2000
**Dependencies:** Task 1A

1. Modify `internal/mcp/handlers.go`:
   - Add `signal_captain` tool:
     ```go
     s.RegisterTool(ToolDefinition{
         Name:        "signal_captain",
         Description: "Signal Captain that you need attention",
         Parameters: map[string]ParameterDef{
             "signal":  {Type: "string", Description: "stopped|blocked|completed|error", Required: true},
             "context": {Type: "string", Description: "Brief explanation", Required: true},
         },
         Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
             // Update DB + notify Captain
         },
     })
     ```
   - Add callback `OnSignalCaptain` to ToolCallbacks interface

2. Add notification mechanism to alert Captain of agent signals

**Verification:**
```bash
go test ./internal/mcp/... -v -run Signal
```

---

## Wave 3: Wiring (Sequential - after Wave 2)

### Task 3A: Server & Main Integration
**Agent:** SNTGreen or Captain
**Estimated tokens:** ~2000
**Dependencies:** All Wave 2 tasks

1. Modify `internal/server/server.go`:
   - Add CleanupService field
   - Create cleanup service in NewServer()
   - Start cleanup in Start() method
   - Wire OnSignalCaptain callback
   - Add `GET /api/agents/control` endpoint (reads from DB)
   - Add `POST /api/agents/{id}/shutdown` endpoint (sets DB flag)

2. Modify `cmd/cliaimonitor/main.go`:
   - Pass memDB to spawner
   - Ensure cleanup stops on shutdown

3. Update server tests

**Verification:**
```bash
go build ./cmd/cliaimonitor && go test ./... -v
```

---

## Wave 4: Dashboard Update (After Wave 3)

### Task 4A: Dashboard Integration
**Agent:** SNTGreen (frontend)
**Estimated tokens:** ~1500
**Dependencies:** Wave 3

1. Modify `web/app.js`:
   - Fetch agent status from `/api/agents/control` (DB-backed)
   - Show heartbeat age in agent cards
   - Add visual indicator for agents approaching stale threshold
   - Remove manual cleanup button (auto-cleanup handles it)

2. Add CSS for heartbeat status indicators

**Verification:**
- Manual testing via browser
- Check agents auto-disappear after 120s of no heartbeat

---

## Parallel Execution Summary

```
Time ─────────────────────────────────────────────────────────►

Wave 1:  ┌─────────────┐  ┌─────────────┐
         │ Task 1A:    │  │ Task 1B:    │
         │ DB Layer    │  │ Heartbeat   │
         │ (Agent A)   │  │ Script      │
         └─────────────┘  │ (Agent B)   │
                          └─────────────┘
              │                  │
              ▼                  ▼
Wave 2:  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
         │ Task 2A:    │  │ Task 2B:    │  │ Task 2C:    │
         │ Cleanup     │  │ Spawner     │  │ MCP Signal  │
         │ (Agent A)   │  │ (Agent B)   │  │ (Agent C)   │
         └─────────────┘  └─────────────┘  └─────────────┘
              │                  │                │
              └──────────────────┴────────────────┘
                                 │
                                 ▼
Wave 3:                   ┌─────────────┐
                          │ Task 3A:    │
                          │ Wiring      │
                          │ (Captain)   │
                          └─────────────┘
                                 │
                                 ▼
Wave 4:                   ┌─────────────┐
                          │ Task 4A:    │
                          │ Dashboard   │
                          │ (Agent)     │
                          └─────────────┘
```

**Total agents needed:** 3 concurrent (Wave 2)
**Estimated total tokens:** ~18,000
**Estimated time:** 4 waves, ~30-45 min with parallel execution

---

## Testing Checklist

- [ ] Unit tests pass: `go test ./...`
- [ ] Build succeeds: `go build ./cmd/cliaimonitor`
- [ ] Spawn agent → appears in DB
- [ ] Heartbeat updates every 30s
- [ ] Kill agent process → auto-cleaned after 120s
- [ ] Shutdown flag → agent stops gracefully
- [ ] Dashboard shows live heartbeat status
- [ ] MCP signal_captain tool works

---

## Rollback Plan

If issues arise:
1. State.json still works as fallback
2. Remove migration, revert to state.json-only
3. Kill heartbeat scripts manually: `Get-Process powershell | Where-Object {$_.CommandLine -like "*agent-heartbeat*"} | Stop-Process`
