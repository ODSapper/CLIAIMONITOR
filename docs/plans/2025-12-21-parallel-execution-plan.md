# Parallel Execution Plan: Pure MCP + Pane Colors

**Date:** 2025-12-21
**Status:** READY FOR EXECUTION
**Initiatives:**
- Remove NATS, go Pure MCP
- Add colored backgrounds/banners to agent panes

## Agent Assignment Strategy

| Model | Role | Use For |
|-------|------|---------|
| **Sonnet** | Implementation | New code, complex refactoring, integration |
| **Haiku** | Cleanup/Simple | Deletions, simple edits, testing, verification |

## Execution Waves

```
Timeline:
═══════════════════════════════════════════════════════════════════════
WAVE 1 (Parallel - Foundation)
├── SNT-Green-A: SSE Presence Tracker [MCP]
├── SNT-Green-B: Pane Color Functions [COLORS]
└── Haiku-Purple: Review both as they complete
═══════════════════════════════════════════════════════════════════════
WAVE 2 (Parallel - Cleanup & Wiring)
├── Haiku-Green-A: Delete internal/nats/ package [MCP]
├── Haiku-Green-B: Delete internal/aider/ package [MCP]
├── Haiku-Green-C: Update server.go NATS removal [MCP]
└── SNT-Green-C: Wire colors into WezTerm spawn [COLORS]
═══════════════════════════════════════════════════════════════════════
WAVE 3 (Sequential - Integration)
├── SNT-Green: Wire SSE presence into server [MCP]
├── Haiku: Update main.go, health endpoints [MCP]
└── Haiku: go mod tidy, build verification
═══════════════════════════════════════════════════════════════════════
WAVE 4 (Validation)
├── Captain: Spawn test agents, verify colors
├── Captain: Verify presence tracking works
└── Captain: Full system test
═══════════════════════════════════════════════════════════════════════
```

---

## WAVE 1: Foundation (Parallel)

### Task 1A: SSE Presence Tracker [SONNET]
**Agent:** SNT-Green-A
**Branch:** `feat/sse-presence-tracker`
**Files:** `internal/mcp/presence.go` (new)

Create new SSE-based presence tracking:

```go
// internal/mcp/presence.go
package mcp

type SSEPresenceTracker struct {
    connections sync.Map  // agentID -> *SSEConnection
    server      *server.Server
    lastSeen    sync.Map  // agentID -> time.Time
}

func (p *SSEPresenceTracker) OnConnect(agentID string, conn *SSEConnection)
func (p *SSEPresenceTracker) OnDisconnect(agentID string)
func (p *SSEPresenceTracker) UpdateLastSeen(agentID string)  // called by report_status
func (p *SSEPresenceTracker) StartStaleMonitor()  // background goroutine
```

**Acceptance Criteria:**
- [ ] Tracks SSE connections by agent ID
- [ ] Marks agent online when SSE connects
- [ ] Marks agent offline when SSE disconnects
- [ ] Has stale detection (2 min threshold)
- [ ] Unit tests pass

---

### Task 1B: Pane Color Functions [SONNET]
**Agent:** SNT-Green-B
**Branch:** `feat/pane-colors`
**Files:** `internal/agents/colors.go` (new)

Create color/banner generation:

```go
// internal/agents/colors.go
package agents

type AgentColors struct {
    BgDark   string  // Subtle background: "\x1b[48;2;R;G;Bm"
    BgBright string  // Banner background
    FgColor  string  // Text color
    Emoji    string  // 🟢 🟣 🔴 🐍
    Reset    string  // "\x1b[0m"
}

func GetAgentColors(configName string) AgentColors
func GenerateBanner(agentID, configName, role string) string
func GenerateBackgroundTint(configName string) string
```

**Color Map:**
```go
var colorMap = map[string]AgentColors{
    "green":  {BgDark: "\x1b[48;2;5;30;15m",   Emoji: "🟢", ...},
    "purple": {BgDark: "\x1b[48;2;20;10;35m",  Emoji: "🟣", ...},
    "red":    {BgDark: "\x1b[48;2;35;10;10m",  Emoji: "🔴", ...},
    "snake":  {BgDark: "\x1b[48;2;5;25;30m",   Emoji: "🐍", ...},
    "blue":   {BgDark: "\x1b[48;2;5;15;35m",   Emoji: "🔵", ...},
}
```

**Acceptance Criteria:**
- [ ] GetAgentColors returns correct colors for each type
- [ ] GenerateBanner creates formatted Unicode box
- [ ] Colors work in WezTerm (24-bit true color)
- [ ] Unit tests pass

---

### Task 1C: Review Wave 1 [HAIKU]
**Agent:** Haiku-Purple
**Role:** Code review as tasks complete

Review criteria:
- Code quality and Go idioms
- Test coverage
- No regressions

---

## WAVE 2: Cleanup & Wiring (Parallel)

### Task 2A: Delete internal/nats/ [HAIKU]
**Agent:** Haiku-Green-A
**Branch:** `refactor/remove-nats` (shared)

```bash
# Simply delete the package
rm -rf internal/nats/
```

**Acceptance Criteria:**
- [ ] Directory deleted
- [ ] No references remain in codebase

---

### Task 2B: Delete internal/aider/ [HAIKU]
**Agent:** Haiku-Green-B
**Branch:** `refactor/remove-nats` (shared)

```bash
# Delete legacy spawner
rm -rf internal/aider/
```

**Acceptance Criteria:**
- [ ] Directory deleted
- [ ] No references remain

---

### Task 2C: Remove NATS from server.go [HAIKU]
**Agent:** Haiku-Green-C
**Branch:** `refactor/remove-nats` (shared)
**Files:** `internal/server/server.go`

Remove:
- NATS client field from Server struct
- NATS connection in NewServer or Start
- Any NATS-related methods
- Import of nats package

**Acceptance Criteria:**
- [ ] No NATS imports in server.go
- [ ] Server struct has no NATS fields
- [ ] Build passes (may have temporary errors until Wave 3)

---

### Task 2D: Wire Colors into Spawner [SONNET]
**Agent:** SNT-Green-C
**Branch:** `feat/pane-colors`
**Files:** `internal/agents/spawner.go`

Update `SpawnAgentPane` (or equivalent) to:
1. Get colors for agent type
2. Inject background tint escape sequence
3. Show banner before launching claude

```go
func (s *Spawner) SpawnAgentPane(...) error {
    colors := GetAgentColors(config.Name)
    banner := GenerateBanner(agentID, config.Name, config.Role)
    bgTint := colors.BgDark

    // Modify spawn command to include colors
    spawnCmd := fmt.Sprintf(
        "echo -e '%s' && echo -e '%s' && claude --mcp-config ...",
        bgTint, banner,
    )
    ...
}
```

**Acceptance Criteria:**
- [ ] Spawner calls color functions
- [ ] Background tint applied to pane
- [ ] Banner displayed before claude starts
- [ ] Works on Windows (cmd.exe or powershell)

---

## WAVE 3: Integration (Sequential)

### Task 3A: Wire SSE Presence into Server [SONNET]
**Agent:** SNT-Green
**Branch:** `feat/sse-presence-tracker`
**Files:** `internal/server/server.go`, `internal/mcp/sse.go`

Integrate the new presence tracker:
1. Add SSEPresenceTracker to Server
2. Call OnConnect when SSE client connects
3. Call OnDisconnect when SSE client disconnects
4. Call UpdateLastSeen in report_status handler
5. Remove old presence.go (NATS-based)

**Acceptance Criteria:**
- [ ] SSE connections trigger presence updates
- [ ] report_status updates lastSeen
- [ ] Old presence.go deleted
- [ ] Build passes

---

### Task 3B: Update main.go & Health [HAIKU]
**Agent:** Haiku-Green
**Branch:** `refactor/remove-nats`
**Files:** `cmd/cliaimonitor/main.go`, `internal/server/handlers.go`

1. Remove NATS server startup from main.go
2. Remove NATS connection initialization
3. Update `/api/health` to not check NATS
4. Update `/api/captain/health` to remove nats_connected

**Acceptance Criteria:**
- [ ] No NATS in main.go
- [ ] Health endpoints work without NATS
- [ ] Server starts successfully

---

### Task 3C: Cleanup & Build [HAIKU]
**Agent:** Haiku-Green
**Branch:** `refactor/remove-nats`

```bash
go mod tidy
go build ./...
go test ./...
```

**Acceptance Criteria:**
- [ ] go.mod has no nats dependency
- [ ] Build succeeds
- [ ] All tests pass

---

## WAVE 4: Validation (Captain)

### Task 4A: Visual Color Test
Spawn each agent type and verify colors:

```bash
# Test each color
curl -X POST http://localhost:3000/api/agents/spawn \
  -d '{"config_name":"HaikuGreen","task":"Color test"}'

curl -X POST http://localhost:3000/api/agents/spawn \
  -d '{"config_name":"HaikuPurple","task":"Color test"}'

# Verify:
# - Background tint visible
# - Banner displayed with correct emoji
# - Colors match specification
```

### Task 4B: Presence Test
1. Spawn agent via API
2. Verify agent shows as "connected" in dashboard
3. Kill agent terminal
4. Verify agent shows as "disconnected" within 2 minutes

### Task 4C: Full System Test
1. Spawn multiple agents in grid
2. Verify all colors distinct
3. Verify dashboard updates in real-time
4. Verify no NATS port (4222) in use

---

## Branch Strategy

```
master
├── feat/sse-presence-tracker (Task 1A, 3A)
├── feat/pane-colors (Task 1B, 2D)
└── refactor/remove-nats (Task 2A, 2B, 2C, 3B, 3C)

Merge order:
1. feat/pane-colors → master (independent)
2. refactor/remove-nats → master
3. feat/sse-presence-tracker → master (after NATS removed)
```

---

## Agent Spawn Commands

### Wave 1
```bash
# SNT-Green-A: SSE Presence
curl -X POST http://localhost:3000/api/agents/spawn \
  -H "Content-Type: application/json" \
  -d '{"config_name":"SNTGreen","project_path":"C:/Users/Admin/Documents/VS Projects/CLIAIMONITOR","task":"Task 1A: Create SSE-based presence tracker in internal/mcp/presence.go. See docs/plans/2025-12-21-pure-mcp-architecture.md for requirements. Branch: feat/sse-presence-tracker"}'

# SNT-Green-B: Pane Colors
curl -X POST http://localhost:3000/api/agents/spawn \
  -H "Content-Type: application/json" \
  -d '{"config_name":"SNTGreen","project_path":"C:/Users/Admin/Documents/VS Projects/CLIAIMONITOR","task":"Task 1B: Create pane color functions in internal/agents/colors.go. See docs/plans/2025-12-21-agent-pane-colors.md for color specs. Branch: feat/pane-colors"}'
```

### Wave 2
```bash
# Haiku-Green-A: Delete nats/
curl -X POST http://localhost:3000/api/agents/spawn \
  -d '{"config_name":"HaikuGreen","project_path":"C:/Users/Admin/Documents/VS Projects/CLIAIMONITOR","task":"Task 2A: Delete internal/nats/ directory. Branch: refactor/remove-nats"}'

# Haiku-Green-B: Delete aider/
curl -X POST http://localhost:3000/api/agents/spawn \
  -d '{"config_name":"HaikuGreen","project_path":"C:/Users/Admin/Documents/VS Projects/CLIAIMONITOR","task":"Task 2B: Delete internal/aider/ directory. Branch: refactor/remove-nats"}'

# Continue with remaining Wave 2 tasks...
```

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Build breaks during NATS removal | Wave 2 tasks on same branch, merge together |
| Color escape sequences not working on Windows | Test cmd.exe vs PowerShell, may need adjustment |
| SSE disconnects not detected | Implement ping/pong or connection timeout |
| Merge conflicts between branches | Pane colors branch is independent, merge first |

---

## Success Criteria

- [ ] No NATS dependency in go.mod
- [ ] No port 4222 in use
- [ ] Agents show colored backgrounds when spawned
- [ ] Banners display with correct emoji and info
- [ ] Presence tracking works via SSE
- [ ] Dashboard shows agent status correctly
- [ ] All tests pass
- [ ] Server starts and runs cleanly
