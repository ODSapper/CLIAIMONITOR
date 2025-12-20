# WezTerm Integration Roadmap for CLIAIMONITOR

**Status**: Research Complete - Ready for Implementation
**Date**: 2025-12-20
**Target**: Replace PowerShell spawner with WezTerm CLI integration

---

## Current State

CLIAIMONITOR currently uses **PowerShell-based spawning** (`scripts/agent-launcher.ps1`):
- Spawns agents in separate terminal windows
- No CLI API for pane management
- Limited programmatic control
- No structured output for pane tracking

---

## WezTerm Integration Benefits

| Aspect | Current | WezTerm |
|--------|---------|---------|
| Pane tracking | Manual/logged | Automatic via `list` JSON |
| Pane creation | PowerShell windowing | Structured `spawn` API |
| Command execution | Window-based | Direct pane-targeted `send-text` |
| Grid layouts | Manual window arrangement | Automatic via `split-pane` |
| Agent identification | Window title only | Tab title + ID mapping |
| Monitoring | External process watching | Built-in `list` queries |
| Cleanup | Kill by title (fragile) | Kill by pane_id (reliable) |

---

## Implementation Phases

### Phase 1: Core Spawner Wrapper (Week 1)

**Goal**: Replace PowerShell spawner with WezTerm CLI

**Tasks**:
1. Create `internal/wezterm/spawner.go`
   - `SpawnAgent(config AgentConfig) (int, error)` → returns pane_id
   - `ListAgents() ([]AgentInfo, error)` → returns active panes
   - `KillAgent(paneID int) error` → graceful termination

2. Update `internal/agents/spawner.go`
   - Replace PowerShell exec with WezTerm CLI calls
   - Store pane_id in agent registry

3. Add to `internal/memory/` (if using context persistence)
   - Save agent → pane_id mappings
   - Restore on restart from memory.db

4. Tests: `internal/wezterm/spawner_test.go`
   - Mock `wezterm cli` commands
   - Test spawn → list → kill flow

**Output**: Agents spawn in WezTerm, pane IDs tracked in CLIAIMONITOR

---

### Phase 2: Dashboard Integration (Week 2)

**Goal**: Visual agent management in web dashboard

**Tasks**:
1. Update `web/` dashboard
   - Query `/api/agents/list` → calls WezTerm `list`
   - Display pane_id, title, cwd, is_active status
   - Add "Focus" button → `activate-pane` CLI call
   - Add "Kill" button → `kill-pane` CLI call
   - Add "Rename" button → `set-tab-title` CLI call

2. WebSocket updates
   - Poll WezTerm every 2-5 seconds
   - Broadcast pane state changes to clients
   - Show real-time agent status (alive/dead)

3. New API endpoints
   - `GET /api/agents/{pane_id}` → single pane status
   - `POST /api/agents/{pane_id}/focus` → activate-pane
   - `POST /api/agents/{pane_id}/rename` → set-tab-title
   - `DELETE /api/agents/{pane_id}` → kill-pane

**Output**: Dashboard shows live agent grid with pane details

---

### Phase 3: Advanced Features (Week 3+)

**Goal**: Enhanced monitoring and control

**Tasks**:
1. **Agent Output Capture**
   - Use `get-text` to capture pane output
   - Store in database for logging/review
   - Show in dashboard UI

2. **Workspace Layouts**
   - Support predefined workspace configs
   - Spawn multiple agents with split layout
   - Save/restore layouts

3. **Command Pipeline**
   - Queue commands to agents
   - Use `send-text` to execute
   - Track completion via output parsing

4. **Health Monitoring**
   - Regular pane status polling
   - Detect hung agents (no output for N seconds)
   - Auto-restart if configured

5. **Agent Grouping**
   - Group agents by project/task
   - One WezTerm window per group
   - Tabs for individual agents

---

## File Changes Required

### New Files

```
internal/wezterm/
  ├── spawner.go          # Core WezTerm CLI wrapper
  ├── spawner_test.go     # Unit tests
  ├── types.go            # PaneInfo, AgentInfo types
  └── json.go             # JSON parsing from wezterm cli list

docs/
  ├── WEZTERM_CLI_RESEARCH.md         # ✅ COMPLETE
  ├── WEZTERM_QUICK_REFERENCE.md      # ✅ COMPLETE
  └── WEZTERM_INTEGRATION_ROADMAP.md  # ✅ THIS FILE
```

### Modified Files

```
internal/agents/spawner.go
  - Replace PowerShell spawner with WezTerm
  - Update agentRegistry type to include pane_id

internal/agents/spawner_test.go
  - Update tests to mock WezTerm CLI

internal/server/handlers.go
  - Add /api/agents/{id}/focus endpoint
  - Add /api/agents/{id}/rename endpoint
  - Add /api/agents/{id}/kill endpoint

internal/server/websocket.go
  - Add polling loop for `wezterm cli list`
  - Broadcast pane state changes

web/index.html
  - Add pane_id display
  - Add focus/rename/kill buttons
  - Update agent card layout

web/dashboard.js
  - Call new agent control endpoints
  - Parse pane status from API
  - Show is_active indicator
```

### Files to Remove/Archive

```
scripts/agent-launcher.ps1    # Replaced by wezterm CLI
scripts/kill-agents.ps1       # Replaced by /api/agents/{id}/kill
```

---

## Implementation Details

### Type Definitions

```go
// internal/wezterm/types.go

// PaneInfo mirrors WezTerm's JSON output
type PaneInfo struct {
    WindowID    int    `json:"window_id"`
    TabID       int    `json:"tab_id"`
    PaneID      int    `json:"pane_id"`
    Workspace   string `json:"workspace"`
    Title       string `json:"title"`        // Current running program
    TabTitle    string `json:"tab_title"`    // Custom tab name
    CWD         string `json:"cwd"`          // file:// URI
    IsActive    bool   `json:"is_active"`
    IsZoomed    bool   `json:"is_zoomed"`
}

// AgentInfo represents CLIAIMONITOR's agent with pane mapping
type AgentInfo struct {
    PaneID      int
    ConfigName  string        // e.g., "SNTGreen"
    ProjectPath string
    Status      string        // "spawning", "running", "completed", "failed"
    Title       string
    SpawnedAt   time.Time
    LastSeen    time.Time
}

// AgentRegistry maps agents to panes
type AgentRegistry struct {
    mu      sync.RWMutex
    agents  map[int]*AgentInfo  // pane_id → agent
}
```

### Core Spawner Interface

```go
// internal/wezterm/spawner.go

type Spawner interface {
    // Spawn creates new pane and returns ID
    Spawn(ctx context.Context, config AgentConfig) (int, error)

    // List returns all active panes
    List(ctx context.Context) ([]PaneInfo, error)

    // SendText sends command to pane
    SendText(ctx context.Context, paneID int, text string) error

    // SetTitle renames pane tab
    SetTitle(ctx context.Context, paneID int, title string) error

    // Kill terminates pane
    Kill(ctx context.Context, paneID int) error

    // Activate focuses pane
    Activate(ctx context.Context, paneID int) error

    // GetText reads pane output
    GetText(ctx context.Context, paneID int) (string, error)
}

type WezTermSpawner struct {
    // Can have options like auto-start, preferred mux, etc.
}

func (s *WezTermSpawner) Spawn(ctx context.Context, config AgentConfig) (int, error) {
    cmd := exec.CommandContext(ctx,
        "wezterm", "cli", "spawn",
        "--new-window",
        "--cwd", config.ProjectPath,
        "--", "powershell")

    output, err := cmd.Output()
    if err != nil {
        return 0, fmt.Errorf("spawn failed: %w", err)
    }

    paneID, err := strconv.Atoi(strings.TrimSpace(string(output)))
    return paneID, err
}

func (s *WezTermSpawner) List(ctx context.Context) ([]PaneInfo, error) {
    cmd := exec.CommandContext(ctx, "wezterm", "cli", "list", "--format", "json")
    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }

    var panes []PaneInfo
    err = json.Unmarshal(output, &panes)
    return panes, err
}
```

### Polling Loop (for dashboard)

```go
// internal/server/websocket.go - Add polling goroutine

func (h *Hub) pollWezTermPanes(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    lastState := make(map[int]PaneInfo)

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            panes, err := h.wezterm.List(ctx)
            if err != nil {
                log.Printf("WezTerm list failed: %v", err)
                continue
            }

            // Detect changes
            for _, pane := range panes {
                if old, exists := lastState[pane.PaneID]; !exists || old.IsActive != pane.IsActive {
                    h.broadcastStateChange(pane)
                }
            }

            // Detect removals
            for id := range lastState {
                found := false
                for _, pane := range panes {
                    if pane.PaneID == id {
                        found = true
                        break
                    }
                }
                if !found {
                    h.broadcastStateChange(PaneInfo{PaneID: id, Title: "DEAD"})
                }
            }

            lastState = toMap(panes)
        }
    }
}

// Called during Hub.Start()
go h.pollWezTermPanes(ctx, 2*time.Second)
```

---

## Testing Strategy

### Unit Tests

```go
// internal/wezterm/spawner_test.go

func TestSpawn(t *testing.T) {
    spawner := NewMockSpawner()
    spawner.MockOutput = "5"  // Pane ID

    id, err := spawner.Spawn(context.Background(), testConfig)
    assert.NoError(t, err)
    assert.Equal(t, 5, id)
}

func TestListParseJSON(t *testing.T) {
    json := `[{"pane_id": 1, "title": "bash", ...}]`
    panes, err := parseWezTermJSON(json)
    assert.Equal(t, 1, panes[0].PaneID)
}

func TestKillPane(t *testing.T) {
    spawner := NewMockSpawner()
    err := spawner.Kill(context.Background(), 5)
    assert.NoError(t, err)
}
```

### Integration Tests

```bash
#!/bin/bash
# tests/integration/wezterm_test.sh

# Start fresh WezTerm instance
wezterm start --class TEST &
WEZTERM_PID=$!

# Wait for startup
sleep 2

# Test spawn
PANE_ID=$(wezterm cli --class TEST spawn --new-window -- powershell)
echo "Spawned: $PANE_ID"

# Test list
PANES=$(wezterm cli --class TEST list --format json)
echo "Listed: $PANES"

# Test send-text
wezterm cli --class TEST send-text --pane-id $PANE_ID "Write-Host 'Test'\n"

# Test kill
wezterm cli --class TEST kill-pane --pane-id $PANE_ID

# Cleanup
kill $WEZTERM_PID
```

---

## Rollback Plan

If WezTerm integration encounters issues:

1. **Keep PowerShell spawner** in `internal/agents/spawner_legacy.go`
2. **Use feature flag** to switch spawners
3. **Gradual rollout**: Test with single agent config first
4. **Metrics**: Track pane creation success rate
5. **Fallback**: If WezTerm fails, use legacy spawner

---

## Success Criteria

- [ ] Agents spawn in WezTerm windows via `wezterm cli spawn`
- [ ] Pane IDs captured and stored in agent registry
- [ ] Dashboard displays live agent status (alive/dead)
- [ ] Focus/Rename/Kill buttons work in dashboard
- [ ] Agent output retrievable via `wezterm cli get-text`
- [ ] No memory leaks from polling loop
- [ ] Tests pass with 80%+ coverage
- [ ] PowerShell spawner fully removed

---

## Timeline Estimate

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| 1: Core spawner | 3-4 days | Agents spawn via WezTerm, basic tracking |
| 2: Dashboard UI | 2-3 days | Live agent grid with controls |
| 3: Advanced features | 2-3 weeks | Output capture, layouts, health monitoring |
| Testing & Polish | 1-2 weeks | Integration tests, bug fixes, documentation |

**Total**: 4-5 weeks from start to full production-ready integration

---

## Dependencies

- WezTerm CLI (version 20240203-110809-5046fc22 or newer)
- Go 1.19+ (for context support)
- No external Go packages required (uses stdlib `exec`, `json`)

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| WezTerm not installed | Low | Spawn fails | Check for executable at startup, clear error message |
| Pane ID collisions | Very Low | Agent tracking broken | Use UUID + pane_id mapping |
| Polling loop CPU spike | Low | High resource usage | Tune polling interval, implement exponential backoff |
| JSON parsing errors | Low | Runtime panic | Add validation, fallback parsing |
| Command queueing delays | Medium | Slow agent responses | Buffer commands, implement retry logic |

---

## Next Steps

1. **Review Research Documents**
   - Read `WEZTERM_CLI_RESEARCH.md` for full technical details
   - Use `WEZTERM_QUICK_REFERENCE.md` for implementation snippets

2. **Create WezTerm Wrapper**
   - Implement `internal/wezterm/spawner.go`
   - Add unit tests

3. **Update Agent Spawner**
   - Modify `internal/agents/spawner.go` to use WezTerm
   - Update agent registry structure

4. **Test Locally**
   - Verify spawn/list/kill flow works
   - Check pane ID persistence

5. **Dashboard Integration**
   - Add polling loop
   - Update UI components
   - Test live updates

6. **Deploy & Monitor**
   - Gradual rollout
   - Track success metrics
   - Gather feedback

---

## References

- **WezTerm Official Docs**: https://wezfurlong.org/wezterm/
- **WezTerm CLI Reference**: https://wezfurlong.org/wezterm/cli/index.html
- **Research Document**: `docs/WEZTERM_CLI_RESEARCH.md`
- **Quick Reference**: `docs/WEZTERM_QUICK_REFERENCE.md`
- **Current Spawner**: `internal/agents/spawner.go`
- **Current Tests**: `internal/agents/spawner_test.go`

---

**Document Status**: Ready for Implementation
**Last Updated**: 2025-12-20
**Author**: Claude Code Research
