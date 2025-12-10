# CLIAIMONITOR - Claude Context

## Project Overview
AI agent orchestration system using Claude via MCP (Model Context Protocol).

## Quick Start
```bash
# Build
go build -o cliaimonitor.exe ./cmd/cliaimonitor/

# Run
./cliaimonitor.exe

# Health check
curl http://localhost:3000/api/health
```

## Current Status (2025-12-08)

### Session Context Persistence (NEW)
Captain can now save and restore context across restarts using `memory.db`.

**MCP Tools**:
- `save_context` - Save key-value context with priority and TTL
- `get_context` - Get a specific context entry
- `get_all_context` - Get all saved context (use at startup)
- `log_session` - Log significant events

**API Endpoints**:
- `GET /api/captain/context` - Get all context
- `POST /api/captain/context` - Save context
- `DELETE /api/captain/context/{key}` - Delete context
- `GET /api/captain/context/summary` - Get formatted summary

**Common Context Keys**:
- `current_focus` - What Captain is currently working on
- `recent_work` - Summary of recent completed work
- `pending_tasks` - Tasks waiting to be done
- `known_issues` - Issues discovered but not yet fixed

**At Startup**: Call `get_all_context` MCP tool to restore previous session state.

### Previous Work: Captain Card Redesign
**Completed**:
1. Captain card with embedded chat at top of agent grid (2-column span)
2. Captain health check - sets `captain_connected=true` on startup when NATS running
3. Removed New Task button/modal - use Captain Chat instead
4. Fixed missing `updateConnectionStatus()` method
5. Bottom row now 2 columns (Alerts + Metrics)
6. Tested spawner - agents spawn via API, register via MCP, send NATS heartbeats

**Dashboard shows**:
- Captain card at top with chat
- Agent cards below in grid
- Green dots for NATS/Captain when connected

## Key Directories
- `cmd/cliaimonitor/` - Main entry point
- `internal/server/` - HTTP server, WebSocket hub, NATS bridge
- `internal/agents/` - Agent spawner and configs
- `internal/nats/` - NATS messaging
- `internal/tasks/` - Task queue and store
- `web/` - Dashboard (HTML, CSS, JS)
- `configs/teams.yaml` - Agent configurations

## Key APIs
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/state` | GET | Dashboard state |
| `/api/health` | GET | Server health |
| `/api/captain/health` | GET | Captain/NATS health |
| `/api/agents/spawn` | POST | Spawn new agent terminal |
| `/api/captain/command` | POST | Send command to Captain via NATS |
| `/ws` | WebSocket | Real-time updates |

## Spawning Agents
```bash
curl -X POST http://localhost:3000/api/agents/spawn \
  -H "Content-Type: application/json" \
  -d '{"config_name": "SNTGreen", "project_path": "C:/path/to/project", "task": "Your task here"}'
```

**Available configs** (from `configs/teams.yaml`):
- `HaikuGreen`, `HaikuPurple` - Fast/cheap for simple tasks
- `SNTGreen`, `SNTPurple`, `SNTRed` - Sonnet for standard work
- `OpusGreen`, `OpusPurple`, `OpusRed` - Opus for complex work
- `Snake` - Reconnaissance & Special Ops

## Architecture
```
Browser Dashboard ←→ WebSocket Hub ←→ Server State
                                    ↓
NATS Server (embedded, port 4222) ←→ Captain/Agents
                                    ↓
                            MCP SSE Endpoints (/mcp/sse)
```

**Agent Connection Flow**:
1. Agent spawned via `/api/agents/spawn` (creates terminal window)
2. Agent connects to MCP SSE endpoint
3. Agent calls `register_agent` MCP tool
4. Agent sends heartbeats via NATS (`agent.{id}.heartbeat`)
5. Dashboard updates via WebSocket broadcast

## Debug Tips
- Browser console: Look for `[DASHBOARD]` prefixed logs
- Server logs: Look for `[NATS-BRIDGE]` and `[CAPTAIN]` prefixed logs
- API test: `curl http://localhost:3000/api/state`

## Session Context Files
- `docs/context/2025-12-08-captain-card-session.md` - Current session details
- `docs/plans/2025-12-08-captain-card-redesign.md` - Implementation plan
