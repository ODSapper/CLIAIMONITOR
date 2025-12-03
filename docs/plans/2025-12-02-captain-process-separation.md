# Captain Process Separation Design

**Date:** 2025-12-02
**Status:** Approved
**Problem:** Captain crash kills entire server, losing agent state and persistence

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Server Process (main)                        │
│                     cliaimonitor.exe                             │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────────────────┐ │
│  │ HTTP Server │  │ MCP Server  │  │ Captain Supervisor       │ │
│  │ :3000       │  │ SSE         │  │ - Spawns Captain terminal│ │
│  │ Dashboard   │  │ Agent comms │  │ - Monitors exit codes    │ │
│  └─────────────┘  └─────────────┘  │ - Auto-respawn on crash  │ │
│         │                │         │ - Crash loop protection  │ │
│         └────────────────┴─────────┴──────────────────────────┘ │
│                              │                                   │
│                    ┌─────────▼─────────┐                        │
│                    │   Agent Spawner   │                        │
│                    │   State/Metrics   │                        │
│                    └───────────────────┘                        │
└─────────────────────────────────────────────────────────────────┘
                               │
                    Windows Terminal spawn
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Captain Process (separate)                     │
│                   claude --model opus ...                        │
├─────────────────────────────────────────────────────────────────┤
│  Interactive Claude session with full shell access              │
│  Exit 0 = user quit → triggers server shutdown                  │
│  Exit non-zero = crash → server respawns Captain                │
└─────────────────────────────────────────────────────────────────┘
```

## CaptainSupervisor Component

Location: `internal/captain/supervisor.go`

```go
type CaptainSupervisor struct {
    basePath       string
    serverPort     int

    // Process tracking
    captainPID     int
    captainCmd     *exec.Cmd

    // Crash loop protection
    respawnCount   int
    respawnWindow  time.Time  // Reset counter after 1 minute
    maxRespawns    int        // Default: 3

    // State
    running        bool
    shutdownChan   chan struct{}
}
```

### Behavior

1. **On start**: Spawns Captain in Windows Terminal
2. **Monitoring**: Goroutine watches Captain process, captures exit code
3. **Exit code 0**: Signals server shutdown (intentional quit)
4. **Exit code non-zero**:
   - Check respawn counter (reset if >1 minute since first crash)
   - If under limit (3): increment counter, spawn new Captain
   - If at limit: log error, stop respawning, update dashboard

## Startup Flow

**New flow:**
```
main() → start server → start CaptainSupervisor → wait for shutdown signal → shutdown all
```

**Shutdown triggers:**
- SIGTERM/SIGINT (Ctrl+C on server terminal)
- `POST /api/shutdown` from dashboard
- Captain exits with code 0 (via supervisor)

## Implementation

### Files to create
- `internal/captain/supervisor.go` - CaptainSupervisor component

### Files to modify
- `cmd/cliaimonitor/main.go` - Remove embedded Captain, add supervisor
- `internal/server/handlers.go` - Add `/api/captain/status` and `/api/captain/restart`
- `internal/server/server.go` - Wire supervisor reference

### Captain spawn command
```powershell
wt new-tab --title "CLIAIMONITOR Captain" -- claude --model claude-opus-4-5-20251101 --dangerously-skip-permissions --system-prompt "..."
```

### API Endpoints
- `GET /api/captain/status` - Returns running/crashed/restarting state
- `POST /api/captain/restart` - Manual restart when crash loop protection triggered

## Crash Loop Protection

- Max 3 respawns within 1 minute window
- Counter resets after 1 minute of stability
- After limit: stop respawning, require manual restart via dashboard/API
