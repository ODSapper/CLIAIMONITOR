# Captain Orchestrator Design

**Date:** 2025-12-02
**Status:** Approved

## Overview

Restructure CLIAIMONITOR so the Captain (Go code) is the central orchestrator, replacing the auto-spawned Supervisor terminal agent. The Captain handles task intake, recon dispatch, decision-making, agent spawning, monitoring, and escalation.

## Architecture

```
Human / Dashboard / Planner API
              │
              ▼
        ┌───────────┐
        │  CAPTAIN  │  ← Go code - the brain (runs in main process)
        └───────────┘
              │
       ┌──────┴──────┐
       │             │
       ▼             ▼
   ┌───────┐   ┌─────────────────┐
   │ SNAKE │   │  FULL AGENTS    │
   │(recon)│   │ (implementation)│
   └───────┘   └─────────────────┘
    subagent      terminal agents
```

**Captain (Go code) handles:**
- Receives tasks from dashboard/Planner API
- Spawns Snake subagents for reconnaissance
- Parses recon output using DecisionEngine
- Spawns full terminal agents for implementation
- Monitors agent health via state/MCP
- Handles escalations (queues for human input)

**Removed:**
- Auto-spawn supervisor on startup
- Supervisor as separate concept
- `supervisor/` dispatcher and executor logic absorbed into Captain

## Startup Flow

**Current (to be replaced):**
```go
// main.go line 217-241
if !*noSupervisor {
    pid, err := spawner.SpawnSupervisor(config.Supervisor)
    // ... spawns Claude terminal agent
}
```

**New:**
```go
// main.go
captain := captain.NewCaptain(basePath, spawner, memoryDB, configs)

// Start Captain's main loop in background
go captain.Run(ctx)
```

## Captain Main Loop

```go
func (c *Captain) Run(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return

        case <-ticker.C:
            // 1. Check for pending tasks (from queue/API)
            tasks := c.checkPendingTasks()

            // 2. For tasks needing recon, spawn Snake
            for _, task := range tasks {
                if task.NeedsRecon {
                    report := c.runSnakeRecon(ctx, task)
                    task.ReconReport = report
                }
            }

            // 3. Analyze and decide (using DecisionEngine logic)
            for _, task := range tasks {
                plan := c.analyzeAndPlan(task)
                c.executeAgentSpawns(plan)
            }

            // 4. Health check running agents
            c.checkAgentHealth()

            // 5. Process escalations
            c.processEscalations()
        }
    }
}
```

**Key behaviors:**
- Polls every 30 seconds (configurable)
- Snake recon runs as subagent (`claude --print`)
- DecisionEngine logic (already in Go) makes spawn decisions
- Full agents spawn in Windows Terminal with MCP
- Health checks use existing state/metrics

## File Changes

### Files to Modify

1. **`cmd/cliaimonitor/main.go`**
   - Remove supervisor auto-spawn (lines 217-241)
   - Add Captain initialization and `go captain.Run(ctx)`

2. **`internal/captain/captain.go`**
   - Add `Run(ctx)` main loop method
   - Add `checkPendingTasks()` - read from `data/pending_tasks.json` or HTTP queue
   - Add `runSnakeRecon()` - already have `executeSubagent()` logic
   - Add `checkAgentHealth()` - query state store for stale agents
   - Add `processEscalations()` - check for human-required items

3. **`internal/supervisor/decision.go`**
   - Keep as-is, Captain imports and uses `DecisionEngine`

### Files to Remove/Deprecate

- `internal/supervisor/dispatcher.go` - absorbed into Captain
- `internal/supervisor/executor.go` - absorbed into Captain
- `internal/handlers/supervisor.go` - HTTP handlers move to captain handlers

### Files to Expand

- `internal/handlers/captain.go` - add endpoints:
  - `POST /api/captain/task` - submit task to Captain
  - `GET /api/captain/status` - Captain loop status
  - `POST /api/captain/recon` - trigger manual recon

## Data Flow

### Task Lifecycle

```
1. INTAKE
   Dashboard POST /api/captain/task
   OR Planner API webhook
   OR pending_tasks.json
        │
        ▼
2. RECON (if needed)
   Captain spawns Snake subagent
   Snake returns YAML report
   Captain parses into ReconReport
        │
        ▼
3. DECISION
   DecisionEngine.AnalyzeReport()
   Returns ActionPlan with agent recommendations
        │
        ▼
4. EXECUTION
   Captain spawns terminal agents via ProcessSpawner
   Agents connect back via MCP
   State tracked in store
        │
        ▼
5. MONITORING
   Captain polls agent status every 30s
   Detects: stalls, failures, completions
   Triggers escalation if needed
        │
        ▼
6. COMPLETION
   Agent reports task_complete via MCP
   Captain marks task done
   Updates Planner API if connected
```

### Escalation Queue

- Stored in `data/escalations.json` or memory DB
- Dashboard shows pending escalations
- Human responds via `POST /api/captain/escalation/{id}/respond`
- Captain resumes blocked work

## Agent Types

| Agent | Mode | Purpose |
|-------|------|---------|
| Snake | Subagent (`claude --print`) | Reconnaissance, scanning |
| SNTGreen | Terminal | Go development |
| SNTPurple | Terminal | Code review/audit |
| SNTRed | Terminal | Security fixes |
| OpusGreen | Terminal | Architecture work |
| OpusRed | Terminal | Critical security |

## Implementation Order

1. Add `Run()` loop to Captain
2. Wire Captain into main.go, remove supervisor spawn
3. Implement `checkPendingTasks()`
4. Implement `checkAgentHealth()`
5. Implement `processEscalations()`
6. Add HTTP endpoints to captain handlers
7. Remove deprecated supervisor files
8. Test end-to-end flow
