# Pure MCP Architecture Plan

**Date:** 2025-12-21
**Status:** DRAFT
**Author:** Captain

## Overview

Remove NATS dependency and consolidate all agent communication through MCP SSE endpoints. This simplifies the architecture and reduces operational complexity while maintaining all current functionality.

## Current Architecture (NATS + MCP)

```
┌─────────────────────────────────────────────────────────────────┐
│                         Current Flow                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Agent ──NATS──> presence.online.{id} ──> PresenceTracker       │
│         ──NATS──> agent.{id}.heartbeat ──> Server               │
│         ──MCP───> register_agent, report_status ──> Handlers    │
│         ──MCP───> signal_captain ──> Event Bus                  │
│                                                                  │
│  Captain ──NATS──> captain.command ──> Agent                    │
│           ──MCP───> send_to_agent ──> Event Bus ──> Agent       │
│                                                                  │
│  Dashboard <──WebSocket── Server State (bridged from NATS)      │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Problems:**
- Duplicate communication channels (NATS + MCP event bus)
- NATS requires embedded server (port 4222)
- Extra dependency to maintain
- Presence tracking splits between NATS and MCP

## Target Architecture (Pure MCP)

```
┌─────────────────────────────────────────────────────────────────┐
│                         Target Flow                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Agent ──MCP SSE──> /mcp/sse (persistent connection)            │
│         ──MCP tool──> register_agent ──> DB + State             │
│         ──MCP tool──> report_status (periodic) ──> Presence     │
│         ──MCP tool──> signal_captain ──> Event Bus              │
│         <──MCP event── wait_for_events ──> Receive tasks        │
│                                                                  │
│  Captain ──MCP tool──> send_to_agent ──> Event Bus ──> Agent    │
│                                                                  │
│  Dashboard <──WebSocket── Server State (direct, no NATS)        │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Benefits:**
- Single communication channel
- No embedded NATS server needed
- Simpler debugging (one protocol)
- SSE connection already indicates presence
- Event bus already has 100-event buffer with backpressure

## Files Affected

### Remove/Delete
- `internal/nats/` - Entire package (client.go, handler.go, streams.go, server_test.go)
- `internal/aider/bridge.go` - Legacy NATS bridge
- `internal/aider/spawner.go` - Legacy spawner (already replaced by WezTerm)

### Modify
| File | Changes |
|------|---------|
| `internal/server/presence.go` | Replace NATS subscriptions with SSE connection tracking |
| `internal/server/server.go` | Remove NATS server startup, remove NATS client |
| `internal/server/handlers.go` | Remove NATS-related handlers if any |
| `cmd/cliaimonitor/main.go` | Remove NATS initialization |
| `go.mod` | Remove `github.com/nats-io/nats.go` dependency |

### Keep (already MCP-based)
- `internal/mcp/handlers.go` - MCP tool handlers
- `internal/mcp/sse.go` - SSE endpoint
- `internal/events/bus.go` - Event bus for agent messaging

## Implementation Phases

### Phase 1: Presence Migration
Replace NATS presence with SSE connection-based presence:

```go
// New approach: Track SSE connections directly
type SSEPresenceTracker struct {
    connections map[string]*SSEConnection  // agentID -> connection
    mu          sync.RWMutex
}

func (t *SSEPresenceTracker) OnConnect(agentID string, conn *SSEConnection) {
    t.mu.Lock()
    t.connections[agentID] = conn
    t.mu.Unlock()
    // Mark agent online
}

func (t *SSEPresenceTracker) OnDisconnect(agentID string) {
    t.mu.Lock()
    delete(t.connections, agentID)
    t.mu.Unlock()
    // Mark agent offline
}
```

### Phase 2: Heartbeat Migration
Replace NATS heartbeats with MCP `report_status` calls:

**Agent side (prompt template update):**
```
Call report_status every 30 seconds while working.
This keeps Captain informed and prevents stale detection.
```

**Server side:**
- `report_status` handler updates `lastSeen` timestamp
- Stale detection remains the same (2 min threshold)

### Phase 3: Remove NATS
1. Delete `internal/nats/` package
2. Delete `internal/aider/` package (legacy)
3. Remove NATS imports from server.go
4. Remove NATS server startup from main.go
5. Update health endpoint to not check NATS
6. Run `go mod tidy` to clean dependencies

### Phase 4: Dashboard Updates
- Remove any NATS status indicators (or repurpose)
- Ensure WebSocket broadcasts work without NATS bridge
- Update health display (remove NATS connected indicator)

## Rollback Plan

If issues arise:
1. Revert commits
2. NATS code remains functional until deleted
3. Can run both systems in parallel during transition

## Testing Checklist

- [ ] Agent spawns and registers via MCP
- [ ] Agent appears online in dashboard
- [ ] Agent disconnect detected within 2 minutes
- [ ] Captain can send tasks via `send_to_agent`
- [ ] Agent receives tasks via `wait_for_events`
- [ ] Dashboard updates in real-time via WebSocket
- [ ] Server health endpoint reports correct status
- [ ] No NATS port (4222) in use after migration

## Timeline

**Wave 0 (Prep):**
- Document current NATS usage patterns
- Ensure all MCP equivalents work

**Wave 1 (Migration):**
- Implement SSE-based presence
- Update agent prompts for periodic status reports
- Test parallel operation

**Wave 2 (Removal):**
- Delete NATS code
- Update health checks
- Clean dependencies

**Wave 3 (Validation):**
- Full system test
- Multi-agent spawn test
- Disconnect/reconnect scenarios

---

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2025-12-21 | Remove NATS | MCP event bus provides equivalent functionality with less complexity |
