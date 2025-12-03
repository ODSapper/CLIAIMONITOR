# Push-Based Heartbeat & Auto-Respawn Design

**Date:** 2025-12-03
**Status:** Draft
**Authors:** Captain + Human

## Problem Statement

1. Captain cannot reliably tell if agents are alive or dead
2. Dashboard shows "disconnected" even when agents are working fine
3. Captain mistakenly respawned working agents, causing duplicates
4. No automatic recovery when agents actually crash

## Solution: Push-Based Heartbeat with Auto-Respawn

Each agent's companion script pings Captain every 30 seconds via HTTP POST. Captain tracks last-seen times and auto-respawns dead agents with incomplete tasks.

## Architecture

```
┌─────────────────┐                                   ┌─────────────────┐
│  Agent Process  │                                   │    Captain      │
│  (Claude CLI)   │                                   │    Server       │
└────────┬────────┘                                   └────────┬────────┘
         │                                                     │
         │ spawned together                                    │ tracks
         │                                                     │
┌────────▼────────┐                                   ┌────────▼────────┐
│ Heartbeat Script│───── curl POST every 30s ────────►│ agentHeartbeats │
│ (PowerShell)    │      {agent_id, task, status}    │ map[string]time │
└─────────────────┘                                   └─────────────────┘
```

**When heartbeat stops for 120+ seconds:**
1. Captain marks agent as dead
2. Checks if task was completed (via stop_request)
3. If incomplete → auto-respawn with same task
4. If complete → just cleanup

---

## Components

### New HTTP Endpoint

```
POST /api/heartbeat
Body: {
  "agent_id": "team-sntgreen001",
  "status": "working",
  "current_task": "SEC-MSS-002 - Generate HMAC key..."
}
Response: { "ok": true }
```

### Captain Server Changes

- Add `agentHeartbeats map[string]*HeartbeatInfo` to Server struct
- `HeartbeatInfo` struct: `AgentID`, `LastSeen`, `Status`, `CurrentTask`, `Config`
- Background goroutine checks for stale heartbeats every 60 seconds
- Stale threshold: 120 seconds (4 missed heartbeats)

### Heartbeat Script Changes

- Replace sqlite3/dbctl calls with simple `curl` POST
- Keep the 30-second interval
- Include agent_id, status, and current task in payload

### Agent Launcher Changes

- Spawn heartbeat script as background job alongside Claude
- Pass agent ID to heartbeat script
- Heartbeat script dies when terminal closes (natural cleanup)

---

## Auto-Respawn Logic

When Captain detects a stale agent (no heartbeat for 120s):

```
1. Check: Was there an approved stop_request for this agent?
   └─ YES → Clean removal, don't respawn
   └─ NO  → Continue to step 2

2. Check: Is the process actually dead? (PID check as safety)
   └─ NO (still running) → Reset heartbeat timer, log warning
   └─ YES → Continue to step 3

3. Respawn agent with same config:
   - Same config_name (SNTGreen, SNTRed, etc.)
   - Same project_path
   - Same task (from last known current_task)

4. Log: "Agent {id} died, respawned as {new_id} to continue task"
```

**Safety measures:**
- PID check prevents respawning agents that are actually alive
- Stop request check prevents respawning agents that finished gracefully
- New agent gets a new ID (team-sntgreen002 → team-sntgreen003)

---

## Implementation Plan

### Wave 1: Parallel Foundation (2 agents)

#### Task 1A: Heartbeat Endpoint
**Agent:** SNTGreen
**File:** `internal/server/handlers.go`, `internal/server/server.go`
**Dependencies:** None

1. Add `HeartbeatInfo` struct to `internal/server/types.go` or handlers.go:
   ```go
   type HeartbeatInfo struct {
       AgentID     string    `json:"agent_id"`
       ConfigName  string    `json:"config_name"`
       ProjectPath string    `json:"project_path"`
       Status      string    `json:"status"`
       CurrentTask string    `json:"current_task"`
       LastSeen    time.Time `json:"last_seen"`
   }
   ```

2. Add to Server struct in `server.go`:
   ```go
   agentHeartbeats map[string]*HeartbeatInfo
   heartbeatMu     sync.RWMutex
   ```

3. Initialize map in `NewServer()`

4. Add handler `handleHeartbeat` in `handlers.go`:
   ```go
   func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
       var req struct {
           AgentID     string `json:"agent_id"`
           Status      string `json:"status"`
           CurrentTask string `json:"current_task"`
       }
       // Decode, update map with timestamp, respond ok
   }
   ```

5. Register route: `POST /api/heartbeat`

6. Add `GET /api/heartbeats` to see all heartbeat status

**Verification:**
```bash
go build ./cmd/cliaimonitor
curl -X POST http://localhost:3000/api/heartbeat -d '{"agent_id":"test","status":"working","current_task":"test task"}'
curl http://localhost:3000/api/heartbeats
```

---

#### Task 1B: Update Heartbeat Script
**Agent:** SNTGreen
**File:** `scripts/agent-heartbeat.ps1`
**Dependencies:** None

1. Replace the dbctl-based heartbeat with curl POST:
   ```powershell
   param(
       [Parameter(Mandatory=$true)]
       [string]$AgentID,

       [string]$ServerURL = "http://localhost:3000",

       [int]$IntervalSeconds = 30
   )

   Write-Host "[HEARTBEAT] Started for agent: $AgentID" -ForegroundColor Cyan

   while ($true) {
       try {
           $body = @{
               agent_id = $AgentID
               status = "working"
               current_task = ""
           } | ConvertTo-Json

           $response = Invoke-RestMethod -Uri "$ServerURL/api/heartbeat" `
               -Method POST `
               -Body $body `
               -ContentType "application/json" `
               -ErrorAction SilentlyContinue

           Write-Host "[HEARTBEAT] OK - $(Get-Date -Format 'HH:mm:ss')" -ForegroundColor DarkGreen
       } catch {
           Write-Host "[HEARTBEAT] Failed: $_" -ForegroundColor Yellow
       }

       Start-Sleep -Seconds $IntervalSeconds
   }
   ```

2. Remove all sqlite3/dbctl references

**Verification:**
```powershell
# Start server, then:
.\scripts\agent-heartbeat.ps1 -AgentID "test-agent"
# Check http://localhost:3000/api/heartbeats shows the agent
```

---

### Wave 2: Parallel Integration (2 agents, after Wave 1)

#### Task 2A: Stale Agent Checker & Auto-Respawn
**Agent:** SNTGreen
**File:** `internal/server/heartbeat.go` (new file)
**Dependencies:** Task 1A

1. Create `internal/server/heartbeat.go`:
   ```go
   package server

   import (
       "context"
       "log"
       "time"
   )

   const (
       HeartbeatCheckInterval = 60 * time.Second
       StaleThreshold         = 120 * time.Second
   )

   func (s *Server) StartHeartbeatChecker(ctx context.Context) {
       ticker := time.NewTicker(HeartbeatCheckInterval)
       defer ticker.Stop()

       for {
           select {
           case <-ctx.Done():
               return
           case <-ticker.C:
               s.checkStaleAgents()
           }
       }
   }

   func (s *Server) checkStaleAgents() {
       s.heartbeatMu.RLock()
       defer s.heartbeatMu.RUnlock()

       now := time.Now()
       for agentID, info := range s.agentHeartbeats {
           if now.Sub(info.LastSeen) > StaleThreshold {
               go s.handleStaleAgent(agentID, info)
           }
       }
   }

   func (s *Server) handleStaleAgent(agentID string, info *HeartbeatInfo) {
       // 1. Check if stop_request was approved
       // 2. Check if PID is actually dead
       // 3. If dead and no stop_request, respawn
       // 4. Clean up old heartbeat entry
   }
   ```

2. Implement `handleStaleAgent` with:
   - Stop request check via `s.store.GetStopRequests()`
   - PID check via `os.FindProcess()` or PowerShell
   - Respawn via `s.spawner.SpawnAgent()`
   - Logging

3. Start checker in `Server.Start()`:
   ```go
   go s.StartHeartbeatChecker(ctx)
   ```

**Verification:**
```bash
go test ./internal/server/... -v -run Heartbeat
```

---

#### Task 2B: Update Agent Launcher
**Agent:** SNTGreen
**File:** `scripts/agent-launcher.ps1`
**Dependencies:** Task 1B

1. After launching Claude, spawn heartbeat script as background job:
   ```powershell
   # After Start-Process for Claude/Windows Terminal...

   # Spawn heartbeat script in background
   $heartbeatScript = Join-Path $PSScriptRoot "agent-heartbeat.ps1"
   $heartbeatJob = Start-Job -ScriptBlock {
       param($script, $agentId)
       & $script -AgentID $agentId
   } -ArgumentList $heartbeatScript, $AgentID

   Write-Host "Heartbeat started as job: $($heartbeatJob.Id)" -ForegroundColor DarkGray
   ```

2. Alternative: spawn as separate process (survives terminal close):
   ```powershell
   Start-Process powershell.exe -ArgumentList @(
       "-WindowStyle", "Hidden",
       "-File", $heartbeatScript,
       "-AgentID", $AgentID
   ) -PassThru
   ```

3. Store heartbeat PID alongside agent PID for cleanup

**Verification:**
- Spawn agent via API
- Check heartbeat appears in `GET /api/heartbeats`
- Kill agent terminal, verify heartbeat stops

---

### Wave 3: Wiring (After Wave 2)

#### Task 3A: Server Integration
**Agent:** Captain or SNTGreen
**File:** `internal/server/server.go`, `cmd/cliaimonitor/main.go`
**Dependencies:** Task 2A, Task 2B

1. Ensure `StartHeartbeatChecker` is called in `Server.Start()`

2. Add spawner reference to Server for auto-respawn:
   ```go
   type Server struct {
       // ...
       spawner *agents.Spawner
   }
   ```

3. Wire up respawn in `handleStaleAgent`:
   ```go
   // Respawn with same config
   newAgent, err := s.spawner.SpawnAgent(info.ConfigName, info.ProjectPath, info.CurrentTask)
   ```

4. Update dashboard to show heartbeat status:
   - Add heartbeat info to `/api/state` response
   - Or create dedicated `/api/heartbeats` endpoint

**Verification:**
```bash
go build ./cmd/cliaimonitor
# Full integration test:
# 1. Start server
# 2. Spawn agent
# 3. Verify heartbeat appears
# 4. Kill agent process manually
# 5. Wait 120s, verify auto-respawn
```

---

## Parallel Execution Diagram

```
Time ──────────────────────────────────────────────────────►

Wave 1:  ┌──────────────────┐  ┌──────────────────┐
         │ Task 1A:         │  │ Task 1B:         │
         │ Heartbeat        │  │ Update Script    │
         │ Endpoint         │  │ (curl POST)      │
         │ (Agent A)        │  │ (Agent B)        │
         └────────┬─────────┘  └────────┬─────────┘
                  │                     │
                  ▼                     ▼
Wave 2:  ┌──────────────────┐  ┌──────────────────┐
         │ Task 2A:         │  │ Task 2B:         │
         │ Stale Checker    │  │ Update Launcher  │
         │ + Auto-Respawn   │  │ (spawn script)   │
         │ (Agent A)        │  │ (Agent B)        │
         └────────┬─────────┘  └────────┬─────────┘
                  │                     │
                  └──────────┬──────────┘
                             ▼
Wave 3:           ┌──────────────────┐
                  │ Task 3A:         │
                  │ Server Wiring    │
                  │ (Captain)        │
                  └──────────────────┘
```

**Agents needed:** 2 concurrent max
**Estimated time:** 3 waves

---

## Testing Checklist

- [ ] `POST /api/heartbeat` accepts heartbeat and updates map
- [ ] `GET /api/heartbeats` returns all heartbeat info
- [ ] Heartbeat script sends POST every 30s
- [ ] Agent launcher spawns heartbeat script alongside agent
- [ ] Stale checker runs every 60s
- [ ] Agent with no heartbeat for 120s triggers respawn logic
- [ ] Approved stop_request prevents respawn
- [ ] PID check prevents respawning alive agents
- [ ] Auto-respawned agent continues same task

---

## Rollback Plan

If issues arise:
1. Comment out `StartHeartbeatChecker` call
2. Agents still work, just no auto-respawn
3. Manual monitoring via dashboard as before
