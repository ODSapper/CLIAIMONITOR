# Agent Heartbeat Scripts

This directory contains scripts for the DB-based agent heartbeat and control system implemented in Task 1B of the [DB-Based Agent Control Design](../docs/plans/2025-12-02-db-agent-control-design.md).

## Overview

The heartbeat system allows Captain to monitor and control agent processes through a SQLite database, with zero token cost for agents.

```
┌─────────────┐                    ┌──────────────┐
│   Agent     │                    │  Heartbeat   │
│  (Claude)   │                    │   Script     │
│             │                    │ (PowerShell) │
└─────────────┘                    └──────┬───────┘
                                          │
                                          │ Every 30s:
                                          │ 1. Write heartbeat
                                          │ 2. Check shutdown flag
                                          │ 3. Create marker if shutdown
                                          │
                                          ▼
                                   ┌──────────────┐
                                   │   SQLite DB  │
                                   │ (memory.db)  │
                                   │              │
                                   │ agent_control│
                                   └──────────────┘
```

## Core Components

### 1. dbctl.exe - Database Control Utility

**Location:** `cmd/dbctl/main.go`
**Build:** `go build -o bin/dbctl.exe ./cmd/dbctl`

A lightweight CLI for database operations:

```bash
# Update heartbeat
dbctl.exe -db data/memory.db -action heartbeat -agent agent-001

# Check shutdown flag
dbctl.exe -db data/memory.db -action check-shutdown -agent agent-001

# Get agent info (JSON)
dbctl.exe -db data/memory.db -action get-agent -agent agent-001
```

**Why dbctl?**
- sqlite3 CLI not available on all Windows environments
- Consistent interface across all systems
- Proper error handling and JSON output
- Built with same SQLite driver as main app

### 2. agent-heartbeat.ps1 - Heartbeat Monitor

**Location:** `scripts/agent-heartbeat.ps1`

Background script spawned alongside each agent terminal.

**Parameters:**
- `AgentID` (required) - Unique agent identifier
- `DBPath` (optional) - Path to memory.db (default: data/memory.db)
- `IntervalSeconds` (optional) - Heartbeat interval (default: 30)
- `DBCtlPath` (optional) - Path to dbctl (default: bin/dbctl.exe)

**Usage:**
```powershell
.\scripts\agent-heartbeat.ps1 `
    -AgentID "agent-snt-green-001" `
    -IntervalSeconds 30
```

**Behavior:**
1. Writes heartbeat to DB every N seconds
2. Updates agent status to "active"
3. Checks for shutdown_flag in DB
4. If shutdown detected:
   - Creates `data/shutdown-{AgentID}.flag` file
   - Writes shutdown reason to file
   - Exits gracefully
5. Logs all activity to console

**Integration:**
- Spawned by `internal/agents/spawner.go`
- Runs in separate PowerShell process
- PID tracked for cleanup

## Testing Scripts

### test-heartbeat-simple.ps1

Manual test for heartbeat functionality.

```powershell
.\scripts\test-heartbeat-simple.ps1 -TestDuration 20
```

**What it does:**
1. Builds dbctl if needed
2. Creates test agent in DB
3. Starts heartbeat script in new window
4. Monitors heartbeats for N seconds
5. Sets shutdown flag
6. Verifies marker file creation
7. Cleans up

**Expected output:**
```
Test Agent ID: manual-test-123456
Created test agent in database...
Starting heartbeat monitor in background...
[1/4] Status: active | Last heartbeat: 2025-12-03T04:39:28Z
[2/4] Status: active | Last heartbeat: 2025-12-03T04:39:33Z
Testing shutdown signal...
SUCCESS: Shutdown marker file detected!
Reason: Manual test shutdown
```

### test-heartbeat.ps1

Comprehensive test suite using PowerShell jobs.

```powershell
.\scripts\test-heartbeat.ps1
```

More sophisticated test that:
- Uses PowerShell background jobs
- Captures real-time output
- Validates all components
- Includes timeout handling

## Utility Scripts

### create-test-agent.go
```bash
go run scripts/create-test-agent.go <agent-id>
```
Creates a test agent entry in the database.

### set-shutdown-flag.go
```bash
go run scripts/set-shutdown-flag.go <agent-id> "reason"
```
Sets shutdown flag for an agent.

### delete-agent.go
```bash
go run scripts/delete-agent.go <agent-id>
```
Removes agent from database.

### check-db-schema.go
```bash
go run scripts/check-db-schema.go
```
Verifies schema version and agent_control table.

### run-migration-003.go
```bash
go run scripts/run-migration-003.go
```
Manually applies migration 003 (agent_control table).

## Database Schema

The `agent_control` table (created by migration 003):

```sql
CREATE TABLE agent_control (
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

## Workflow Example

### 1. Spawner creates agent
```go
// In internal/agents/spawner.go
agent := &memory.AgentControl{
    AgentID:    "agent-snt-green-001",
    ConfigName: "snt-green",
    Role:       "developer",
    Status:     "starting",
}
memDB.RegisterAgent(agent)
```

### 2. Spawner launches heartbeat script
```powershell
Start-Process powershell.exe -ArgumentList @(
    "-File", "scripts/agent-heartbeat.ps1",
    "-AgentID", "agent-snt-green-001",
    "-IntervalSeconds", "30"
) -PassThru
```

### 3. Heartbeat script runs
```
[HEARTBEAT] Started monitor for agent: agent-snt-green-001
[HEARTBEAT] #1 OK
[HEARTBEAT] #2 OK
[HEARTBEAT] #3 OK
```

### 4. Captain signals shutdown
```go
memDB.SetShutdownFlag("agent-snt-green-001", "Task completed")
```

### 5. Heartbeat detects shutdown
```
[HEARTBEAT] SHUTDOWN SIGNAL RECEIVED
[HEARTBEAT] Reason: Task completed
[HEARTBEAT] Created shutdown marker: data/shutdown-agent-snt-green-001.flag
[HEARTBEAT] Exiting...
```

### 6. Agent detects marker file
Agent's system prompt includes instruction to check for shutdown file periodically:
```
Check for data/shutdown-{AgentID}.flag before starting new tasks.
If found, exit gracefully after current task.
```

## Troubleshooting

### Heartbeat script fails immediately
```powershell
# Check if dbctl is built
ls bin/dbctl.exe

# Build if missing
go build -o bin/dbctl.exe ./cmd/dbctl
```

### Agent entry not found
```bash
# Check if migration 003 was applied
go run scripts/check-db-schema.go

# Should show: Current schema version: 4
# If not, run migration manually:
go run scripts/run-migration-003.go
```

### Shutdown marker not created
```bash
# Check shutdown flag in DB
bin/dbctl.exe -db data/memory.db -action check-shutdown -agent <agent-id>

# Manually set flag for testing
go run scripts/set-shutdown-flag.go <agent-id> "test"
```

### Database locked errors
The database uses WAL mode with 5-second busy timeout. If you still see locks:
```bash
# Check for stale connections
# Close all apps using memory.db
# Restart Captain server
```

## Performance

- **Heartbeat overhead:** ~10ms per update (SQLite WAL mode)
- **Token cost:** 0 (background PowerShell script)
- **Disk I/O:** Minimal (WAL mode batches writes)
- **Memory:** ~5MB per heartbeat script process

## Security Notes

- **SQL Injection:** Agent IDs are parameterized in all queries
- **File Access:** Heartbeat script only reads/writes to data/ directory
- **Process Isolation:** Each agent has separate heartbeat process
- **Shutdown Safety:** Agents can ignore shutdown markers (graceful, not forced)

## Next Steps

This is **Task 1B** of the implementation plan. Next tasks:

- **Task 2A:** Auto-cleanup service (detect stale agents)
- **Task 2B:** Spawner integration (launch heartbeat with agent)
- **Task 2C:** MCP signal tool (agent-to-captain notifications)

See [db-agent-control-design.md](../docs/plans/2025-12-02-db-agent-control-design.md) for full plan.
