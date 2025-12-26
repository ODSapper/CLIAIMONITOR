# CLIAIMONITOR - Claude Context

## Project Overview
AI agent orchestration system using Claude via MCP (Model Context Protocol).

Use spawned agents unless directed specifically to use subagents (sonnet for most things. Opus is for planning and esclations). Save your DB context when completing milestones or making plans. Prefer to use the db over creating extra .md documents. Only documents for end users should be left in production dirs. When you make a plan you should always include a code review and debugging phase(you can use agents to review code while the next phase of a plan is being written. Plan in parallel). We prefer quality over speed when producing code. Try to remember to close agents when they are done working. 

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


## Debug Tips
- Browser console: Look for `[DASHBOARD]` prefixed logs
- Server logs: Look for `[NATS-BRIDGE]` and `[CAPTAIN]` prefixed logs
- API test: `curl http://localhost:3000/api/state`


