# CLIAIMONITOR

A supervisor dashboard and agent coordination system for managing Claude Code AI agents.

## Features

- **Dashboard Server**: HTTP server with embedded web UI on localhost:3000
- **MCP Integration**: SSE-based MCP server for agent communication
- **Supervisor Agent**: Opus agent for judgment calls and monitoring
- **Team Agents**: Spawn and manage multiple agents in Windows Terminal tabs
- **Real-time Metrics**: Track token usage, test failures, idle time
- **Alert System**: Configurable thresholds with sound notifications
- **Human Input Queue**: Handle agent questions requiring human answers
- **JSON Persistence**: State survives restarts

## Quick Start

```bash
# Build
go build -o cliaimonitor.exe ./cmd/cliaimonitor

# Run (spawns supervisor automatically)
./cliaimonitor.exe

# Run without supervisor
./cliaimonitor.exe -no-supervisor

# Custom port
./cliaimonitor.exe -port 8080
```

Dashboard: http://localhost:3000

## Agent Types

| Name | Model | Role | Color |
|------|-------|------|-------|
| SNTGreen | claude-sonnet-4-5-20250929 | Go Developer | #00cc66 |
| SNTPurple | claude-sonnet-4-5-20250929 | Code Auditor | #9933cc |
| SNTRed | claude-sonnet-4-5-20250929 | Engineer | #cc3333 |
| OpusGreen | claude-opus-4-5-20251101 | Go Developer | #33ff99 |
| OpusPurple | claude-opus-4-5-20251101 | Code Auditor | #bb66ff |
| OpusRed | claude-opus-4-5-20251101 | Security | #ff4444 |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    CLIAIMONITOR Dashboard                        │
│                    (Go HTTP Server :3000)                        │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐   │
│  │ Web UI   │  │ REST API │  │ WebSocket│  │ MCP SSE      │   │
│  │ (embed)  │  │ /api/*   │  │ /ws      │  │ /mcp/sse     │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────┘   │
│        │              │             │              │            │
│        └──────────────┴─────────────┴──────────────┘            │
│                              │                                   │
│  ┌───────────────────────────┴───────────────────────────┐     │
│  │                    Core Services                        │     │
│  ├─────────────┬─────────────┬─────────────┬─────────────┤     │
│  │ Agent       │ Metrics     │ Alert       │ Persistence │     │
│  │ Manager     │ Collector   │ Engine      │ (JSON)      │     │
│  └─────────────┴─────────────┴─────────────┴─────────────┘     │
└─────────────────────────────────────────────────────────────────┘
         │                                          │
         │ PowerShell spawn                         │ MCP SSE
         ▼                                          ▼
┌─────────────────┐                    ┌─────────────────┐
│ Windows Terminal│                    │ Claude Code     │
│ Tab: Supervisor │◄───MCP SSE────────►│ Agents          │
└─────────────────┘                    └─────────────────┘
```

## MCP Tools

Agents communicate via MCP tools:

| Tool | Purpose |
|------|---------|
| `register_agent` | Agent identifies itself on connect |
| `report_status` | Update current activity |
| `report_metrics` | Token usage, test results |
| `request_human_input` | Ask human a question |
| `log_activity` | General activity logging |
| `get_agent_metrics` | Supervisor: get all metrics |
| `get_pending_questions` | Supervisor: check queue |
| `escalate_alert` | Create alert for human |
| `submit_judgment` | Record supervisor decision |

## Configuration

Edit `configs/teams.yaml` to modify agent definitions.

Alert thresholds can be adjusted via the dashboard UI.

## Project Structure

```
CLIAIMONITOR/
├── cmd/cliaimonitor/main.go    # Entry point
├── internal/
│   ├── types/                  # Shared types
│   ├── persistence/            # JSON state storage
│   ├── agents/                 # Agent spawner
│   ├── mcp/                    # MCP SSE server
│   ├── metrics/                # Metrics & alerts
│   └── server/                 # HTTP server & WebSocket
├── web/                        # Embedded web UI
├── configs/
│   ├── teams.yaml              # Agent definitions
│   └── prompts/                # System prompts by role
├── scripts/
│   └── agent-launcher.ps1      # PowerShell launcher
└── data/                       # State persistence
```

## Requirements

- Go 1.25.3+
- Windows Terminal (for tab colors)
- Claude Code CLI installed

## License

MIT
