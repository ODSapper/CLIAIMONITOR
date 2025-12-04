# CLIAIR: Aider Sergeant with Qwen

## Overview

CLIAIR is a simplified fork of CLIAIMONITOR designed for local LLM agents using Aider CLI with Qwen models. It removes MCP, HTTP heartbeats, and PowerShell complexity in favor of pure NATS messaging.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      CLIAIR Server                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │    NATS     │  │   SQLite    │  │    Dashboard        │  │
│  │  Embedded   │  │  Persistence│  │    (HTTP :3000)     │  │
│  │  (port 4222)│  │  memory.db  │  │                     │  │
│  └──────┬──────┘  └─────────────┘  └─────────────────────┘  │
│         │                                                    │
│  ┌──────┴──────────────────────────────────────────────┐    │
│  │              Aider Sergeant                          │    │
│  │  - Spawns Aider terminals                           │    │
│  │  - Manages agent lifecycle                          │    │
│  │  - Routes NATS messages                             │    │
│  └──────┬──────────────────────────────────────────────┘    │
└─────────┼────────────────────────────────────────────────────┘
          │
     ┌────┴────┬─────────────┬─────────────┐
     │         │             │             │
 ┌───▼───┐ ┌───▼───┐    ┌────▼────┐  ┌─────▼─────┐
 │ Aider │ │ Aider │    │  Aider  │  │   Aider   │
 │Bridge1│ │Bridge2│    │ Bridge3 │  │  Bridge4  │
 │   +   │ │   +   │    │    +    │  │     +     │
 │ Qwen  │ │ Qwen  │    │  Qwen   │  │   Qwen    │
 └───────┘ └───────┘    └─────────┘  └───────────┘
```

## What Gets REMOVED (Redundancies)

### From CLIAIMONITOR → CLIAIR

| Component | Reason |
|-----------|--------|
| `internal/mcp/` | Aider doesn't use MCP |
| `scripts/agent-heartbeat.ps1` | NATS connection events replace heartbeats |
| `scripts/agent-launcher.ps1` | Replaced by Go-native Aider spawner |
| HTTP `/api/heartbeat` endpoint | NATS handles connection awareness |
| `internal/server/heartbeat.go` | No more heartbeat polling |
| PowerShell heartbeat spawning in spawner.go | Direct NATS |
| MCP SSE endpoints (`/mcp/sse`, `/mcp/messages/`) | Not needed |
| `configs/prompts/*.md` | Aider has built-in prompting |
| `internal/agents/spawner.go` | Replace entirely with aider/spawner.go |

### Simplified Directory Structure

```
CLIAIR/
├── cmd/
│   └── cliair/
│       └── main.go              # Server entry point
├── internal/
│   ├── nats/                    # KEEP - messaging layer
│   │   ├── server.go
│   │   ├── client.go
│   │   ├── messages.go
│   │   └── handler.go
│   ├── aider/                   # NEW - Aider integration
│   │   ├── spawner.go           # Spawn Aider processes
│   │   ├── bridge.go            # NATS ↔ Aider stdout/stdin
│   │   └── config.go            # Aider configuration
│   ├── server/                  # KEEP (simplified)
│   │   ├── server.go            # Remove MCP wiring
│   │   ├── handlers.go          # Remove heartbeat handlers
│   │   ├── hub.go               # WebSocket for dashboard
│   │   └── nats_bridge.go       # KEEP - message routing
│   ├── memory/                  # KEEP - persistence
│   │   └── sqlite.go
│   └── types/                   # KEEP - type definitions
│       └── types.go
├── web/                         # KEEP - dashboard UI
│   └── templates/
├── configs/
│   └── agents.yaml              # Simplified agent configs
└── data/
    └── memory.db
```

## Component Design

### 1. Aider Spawner (`internal/aider/spawner.go`)

```go
type AiderSpawner struct {
    natsClient  *nats.Client
    basePath    string
    agents      map[string]*AiderAgent
    mu          sync.RWMutex
}

type AiderAgent struct {
    ID          string
    Bridge      *AiderBridge
    Process     *os.Process
    ProjectPath string
    Model       string  // e.g., "ollama/qwen2.5-coder:32b"
    StartedAt   time.Time
}

func (s *AiderSpawner) SpawnAgent(config AgentConfig) (*AiderAgent, error) {
    // 1. Create bridge for NATS communication
    // 2. Spawn aider process with Qwen model
    // 3. Connect bridge to process stdout/stdin
    // 4. Register agent in NATS
    // 5. Return agent handle
}
```

### 2. Aider Bridge (`internal/aider/bridge.go`)

The bridge connects Aider's CLI to NATS:

```go
type AiderBridge struct {
    agentID     string
    natsClient  *nats.Client
    stdin       io.WriteCloser
    stdout      io.ReadCloser
    stderr      io.ReadCloser
    status      string
    currentTask string
}

func (b *AiderBridge) Start() error {
    // Subscribe to commands
    b.natsClient.Subscribe(fmt.Sprintf("agent.%s.command", b.agentID), b.handleCommand)

    // Start output parser goroutine
    go b.parseAiderOutput()

    // Announce connection
    b.publishStatus("connected", "Ready")
}

func (b *AiderBridge) parseAiderOutput() {
    scanner := bufio.NewScanner(b.stdout)
    for scanner.Scan() {
        line := scanner.Text()

        // Parse Aider status patterns
        if strings.Contains(line, "Thinking...") {
            b.publishStatus("working", "Thinking")
        } else if strings.Contains(line, "Applied edit") {
            b.publishStatus("working", "Editing files")
        } else if strings.Contains(line, ">") {
            b.publishStatus("idle", "Awaiting input")
        }
    }
}

func (b *AiderBridge) handleCommand(msg *nats.Message) {
    var cmd CommandMessage
    json.Unmarshal(msg.Data, &cmd)

    switch cmd.Type {
    case "prompt":
        // Send prompt to Aider stdin
        fmt.Fprintln(b.stdin, cmd.Payload["text"])
    case "stop":
        // Send /quit to Aider
        fmt.Fprintln(b.stdin, "/quit")
    }
}

func (b *AiderBridge) publishStatus(status, task string) {
    msg := StatusMessage{
        AgentID:   b.agentID,
        Status:    status,
        Message:   task,
        Timestamp: time.Now(),
    }
    b.natsClient.PublishJSON(fmt.Sprintf("agent.%s.status", b.agentID), msg)
}
```

### 3. Aider Configuration (`internal/aider/config.go`)

```go
type AiderConfig struct {
    Model           string   `yaml:"model"`           // ollama/qwen2.5-coder:32b
    OllamaURL       string   `yaml:"ollama_url"`      // http://localhost:11434
    AutoCommit      bool     `yaml:"auto_commit"`     // false recommended
    EditFormat      string   `yaml:"edit_format"`     // whole, diff, udiff
    MapTokens       int      `yaml:"map_tokens"`      // 1024
    MaxChatHistory  int      `yaml:"max_chat_history"` // 10
}

func DefaultQwenConfig() AiderConfig {
    return AiderConfig{
        Model:          "ollama/qwen2.5-coder:32b",
        OllamaURL:      "http://localhost:11434",
        AutoCommit:     false,
        EditFormat:     "diff",
        MapTokens:      1024,
        MaxChatHistory: 10,
    }
}

func (c *AiderConfig) ToArgs() []string {
    return []string{
        "--model", c.Model,
        "--no-auto-commits",
        "--edit-format", c.EditFormat,
        "--map-tokens", fmt.Sprintf("%d", c.MapTokens),
    }
}
```

### 4. Simplified Server (`internal/server/server.go` changes)

Remove MCP-related code:

```diff
 type Server struct {
     httpServer *http.Server
     router     *mux.Router
     hub        *Hub

     // Dependencies
     store          *persistence.JSONStore
-    spawner        *agents.ProcessSpawner
+    spawner        *aider.AiderSpawner
-    mcp            *mcp.Server           // REMOVE
     natsServer     *natslib.EmbeddedServer
     natsClient     *natslib.Client
     natsBridge     *NATSBridge
     memDB          memory.MemoryDB

-    // Heartbeat tracking                 // REMOVE
-    agentHeartbeats map[string]*HeartbeatInfo
-    heartbeatMu     sync.RWMutex
 }
```

### 5. Agent Config (`configs/agents.yaml`)

```yaml
# CLIAIR Agent Configuration
server:
  port: 3000
  nats_port: 4222

ollama:
  url: http://localhost:11434
  model: qwen2.5-coder:32b

agents:
  - name: Qwen-Dev-1
    role: developer
    color: "#00FF00"
    project_path: "C:\\Projects\\MyApp"

  - name: Qwen-Dev-2
    role: developer
    color: "#0088FF"
    project_path: "C:\\Projects\\OtherApp"

sergeant:
  max_concurrent_agents: 4
  idle_timeout: 300  # seconds before asking if should stop
```

## NATS Subject Patterns

Simplified from CLIAIMONITOR:

| Subject | Direction | Purpose |
|---------|-----------|---------|
| `agent.{id}.status` | Agent → Server | Status updates (replaces heartbeat) |
| `agent.{id}.command` | Server → Agent | Send prompts, stop commands |
| `agent.{id}.output` | Agent → Server | Aider stdout for logging |
| `agent.*.status` | Server subscribe | Monitor all agents |
| `sergeant.spawn` | Server internal | Spawn new agent request |
| `sergeant.stop` | Server internal | Stop agent request |

## Connection Awareness (No Heartbeats)

NATS provides connection events:

```go
func (s *AiderSpawner) setupConnectionHandlers() {
    // Bridge registers disconnect handler when connecting
    s.natsClient.SetDisconnectHandler(func(nc *nats.Conn) {
        // Find which agent disconnected by checking bridges
        for id, agent := range s.agents {
            if !agent.Bridge.IsConnected() {
                s.handleAgentDisconnect(id)
            }
        }
    })
}

func (s *AiderSpawner) handleAgentDisconnect(agentID string) {
    log.Printf("[SERGEANT] Agent %s disconnected", agentID)

    // Update dashboard
    s.store.UpdateAgent(agentID, func(a *types.Agent) {
        a.Status = types.StatusDisconnected
    })

    // Cleanup
    s.mu.Lock()
    delete(s.agents, agentID)
    s.mu.Unlock()
}
```

## Implementation Tasks

### Phase 1: Fork and Clean (Day 1)
1. [ ] Create CLIAIR branch from CLIAIMONITOR
2. [ ] Delete `internal/mcp/` directory
3. [ ] Delete `scripts/agent-heartbeat.ps1`
4. [ ] Delete `scripts/agent-launcher.ps1`
5. [ ] Remove heartbeat handlers from `handlers.go`
6. [ ] Remove MCP routes from `server.go`
7. [ ] Remove heartbeat checker from `server.go`
8. [ ] Update go.mod (remove MCP dependencies if any)

### Phase 2: Aider Integration (Day 2)
1. [ ] Create `internal/aider/config.go`
2. [ ] Create `internal/aider/spawner.go`
3. [ ] Create `internal/aider/bridge.go`
4. [ ] Create `cmd/cliair/main.go`

### Phase 3: Server Simplification (Day 2-3)
1. [ ] Simplify `server.go` - remove MCP, heartbeat code
2. [ ] Update `nats_bridge.go` for Aider message patterns
3. [ ] Create simplified `configs/agents.yaml`
4. [ ] Update dashboard to show Aider agents

### Phase 4: Testing (Day 3)
1. [ ] Test single Aider spawn with Qwen
2. [ ] Test NATS status updates
3. [ ] Test command sending (prompts)
4. [ ] Test graceful shutdown
5. [ ] Test crash recovery

## Aider CLI Reference

Key Aider commands for integration:

```bash
# Start Aider with Qwen
aider --model ollama/qwen2.5-coder:32b --no-auto-commits

# Aider CLI commands (sent via stdin)
/add file.go          # Add file to context
/drop file.go         # Remove from context
/clear                # Clear chat history
/quit                 # Exit Aider
/help                 # Show commands

# Aider output patterns to parse
"Thinking..."         # Model is processing
"Applied edit to X"   # File was modified
"> "                  # Awaiting input (idle)
"Error: ..."          # Something went wrong
```

## Prerequisites

1. **Ollama** installed and running
2. **Qwen model** pulled: `ollama pull qwen2.5-coder:32b`
3. **Aider** installed: `pip install aider-chat`

## Success Criteria

- [ ] Server starts with embedded NATS
- [ ] Can spawn Aider agent via dashboard
- [ ] Aider status appears in dashboard
- [ ] Can send prompt to Aider via dashboard
- [ ] Agent disconnection detected automatically
- [ ] No PowerShell scripts required
- [ ] No MCP code in codebase
- [ ] No heartbeat polling

## File Deletion Checklist

Before starting implementation, delete these from CLIAIR fork:

```bash
# MCP (not needed for Aider)
rm -rf internal/mcp/

# PowerShell scripts (replaced by Go)
rm scripts/agent-heartbeat.ps1
rm scripts/agent-launcher.ps1

# Old prompts (Aider has built-in)
rm -rf configs/prompts/

# Old spawner (replace entirely)
rm internal/agents/spawner.go

# Heartbeat code (will remove from handlers.go and server.go)
rm internal/server/heartbeat.go
```

## Notes

- Qwen 2.5 Coder 32B is excellent for code tasks
- Aider's `--no-auto-commits` lets us control git ourselves
- NATS connection events are more reliable than HTTP heartbeats
- The bridge pattern keeps Aider process management simple
- SQLite persistence survives server restarts
