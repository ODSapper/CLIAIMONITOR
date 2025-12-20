# WezTerm Integration Documentation Index

**Research Completion Date**: 2025-12-20
**Status**: Complete - Ready for Implementation

---

## Document Overview

Three comprehensive documents have been created to guide WezTerm integration into CLIAIMONITOR:

### 1. **WEZTERM_CLI_RESEARCH.md** (19 KB)
   **Purpose**: Comprehensive technical research of WezTerm CLI capabilities

   **Contents**:
   - Executive summary of WezTerm features
   - Detailed reference for all 10 CLI commands
   - JSON output format specifications
   - Environment context and limitations
   - 5 recommended integration patterns
   - Complete test results from 6 command investigations

   **Best For**: Understanding full WezTerm ecosystem, solving specific integration challenges

   **Key Findings**:
   - `wezterm cli list` outputs JSON with complete pane metadata
   - `wezterm cli spawn` returns pane ID on stdout (easily captured)
   - Text input lacks auto-newline (must include `\n` in commands)
   - No built-in pane events (must poll for state changes)
   - Can target panes only by ID, not name (parse JSON to find ID)

---

### 2. **WEZTERM_QUICK_REFERENCE.md** (6.7 KB)
   **Purpose**: Quick lookup guide for developers during implementation

   **Contents**:
   - One-liners for every common task
   - Key answer table (FAQ format)
   - Go code patterns and JSON structure
   - Integration checklist
   - Common error scenarios with solutions
   - Bash/PowerShell one-liners

   **Best For**: Quick lookups during coding, copy-paste snippets, rapid prototyping

   **Key Sections**:
   - Single-line commands for all operations
   - Go type definitions for JSON parsing
   - Error scenarios and fixes
   - Implementation checklist

---

### 3. **WEZTERM_INTEGRATION_ROADMAP.md** (14 KB)
   **Purpose**: Implementation planning and execution guide

   **Contents**:
   - Phase-by-phase implementation plan (3 phases over 4-5 weeks)
   - File changes required (new files, modified files, removals)
   - Detailed Go code examples for spawner interface
   - Testing strategy (unit + integration)
   - Rollback plan and feature flags
   - Risk assessment and mitigations
   - Success criteria

   **Best For**: Project planning, task breakdown, timeline estimation

   **Key Phases**:
   - Phase 1: Core spawner wrapper (week 1)
   - Phase 2: Dashboard integration (week 2)
   - Phase 3: Advanced features (weeks 3+)

---

## Quick Start by Role

### For Technical Leads
1. Start with **WEZTERM_INTEGRATION_ROADMAP.md**
   - Review Phase breakdown and timeline
   - Assess implementation effort (4-5 weeks estimated)
   - Check file changes and dependencies

2. Review **WEZTERM_CLI_RESEARCH.md** - Executive Summary
   - Understand key capabilities
   - Review limitations section

3. Use **WEZTERM_QUICK_REFERENCE.md** for risk assessment
   - Gotchas section lists common pitfalls

### For Implementation Engineers
1. Read **WEZTERM_CLI_RESEARCH.md** completely
   - Understand all 10 commands in detail
   - Review integration patterns

2. Reference **WEZTERM_QUICK_REFERENCE.md** during coding
   - Copy Go type definitions
   - Use one-liners for quick testing
   - Check error scenarios while debugging

3. Follow **WEZTERM_INTEGRATION_ROADMAP.md** for tasks
   - Phase 1: Implement spawner wrapper
   - Phase 2: Add dashboard integration
   - Phase 3: Advanced features

### For QA/Testing
1. Review **WEZTERM_INTEGRATION_ROADMAP.md** - Testing Strategy
   - Unit test structure
   - Integration test examples
   - Success criteria checklist

2. Use **WEZTERM_QUICK_REFERENCE.md** - Common Tasks
   - Manual testing procedures
   - Verification commands

---

## Key Research Questions Answered

| Question | Document | Answer Summary |
|----------|----------|-----------------|
| What format does `list` output? | RESEARCH.md § Command Reference | JSON array with 15+ fields per pane |
| Can we get pane ID when spawning? | RESEARCH.md § Key Insights | Yes, captured from stdout |
| Difference between spawn and split? | QUICK_REF.md § Key Answers | spawn creates window/tab, split divides existing pane |
| Can we target panes by title? | RESEARCH.md § JSON Output Parsing | No direct support, parse JSON to find ID |
| What if WezTerm isn't running? | RESEARCH.md § Startup | Auto-starts server via `wezterm cli` |
| How to set pane title? | QUICK_REF.md § One-Liners | Use `set-tab-title` (tab title, not pane title) |
| Can we query if pane is alive? | RESEARCH.md § Pane Lifecycle | Parse `list` output to find pane_id |
| What's the `--class` option? | QUICK_REF.md § Key Answers | Distinguish multiple WezTerm instances |

---

## Implementation Checklist

### Pre-Implementation
- [ ] Read WEZTERM_CLI_RESEARCH.md (full)
- [ ] Review WEZTERM_INTEGRATION_ROADMAP.md (Phase 1 section)
- [ ] Verify WezTerm installed: `wezterm --version`
- [ ] Understand current spawner: `internal/agents/spawner.go`

### Phase 1: Core Spawner (Week 1)
- [ ] Create `internal/wezterm/spawner.go`
- [ ] Implement `Spawn()`, `List()`, `Kill()`, `SendText()` methods
- [ ] Create `internal/wezterm/spawner_test.go`
- [ ] Update `internal/agents/spawner.go` to use WezTerm
- [ ] Verify agents spawn and pane IDs are tracked
- [ ] Commit: "feat(wezterm): implement core spawner wrapper"

### Phase 2: Dashboard (Week 2)
- [ ] Add polling loop in `internal/server/websocket.go`
- [ ] Create `/api/agents/{id}` endpoints
- [ ] Update `web/` dashboard for pane display
- [ ] Test live agent status updates
- [ ] Commit: "feat(wezterm): add dashboard integration"

### Phase 3: Advanced (Weeks 3+)
- [ ] Implement `get-text` for output capture
- [ ] Add workspace layout support
- [ ] Implement command queueing
- [ ] Add health monitoring
- [ ] Commit: "feat(wezterm): add advanced features"

---

## File Locations

All research documents are in:
```
C:\Users\Admin\Documents\VS Projects\CLIAIMONITOR\docs\
```

Individual files:
- `WEZTERM_CLI_RESEARCH.md` - Complete technical reference
- `WEZTERM_QUICK_REFERENCE.md` - Developer cheat sheet
- `WEZTERM_INTEGRATION_ROADMAP.md` - Implementation guide
- `WEZTERM_INDEX.md` - This file (navigation guide)

---

## Key Metrics from Research

- **WezTerm Version Tested**: 20240203-110809-5046fc22
- **Commands Tested**: 6 core commands fully tested
- **JSON Fields**: 15+ fields per pane from `list` output
- **Pane ID Format**: Integer (0+)
- **Success Rate**: 100% for all tested operations
- **Platform**: Windows 11, should work on macOS/Linux too

---

## Critical Gotchas to Remember

1. **No Auto-Newline**: Always include `\n` in `send-text` commands
   ```bash
   wezterm cli send-text --pane-id 0 "command\n"  # CORRECT
   wezterm cli send-text --pane-id 0 "command"    # WRONG - doesn't execute
   ```

2. **Pane ID Only**: Can't use pane names in CLI, must parse JSON first
   ```bash
   # WRONG: No --title option
   wezterm cli send-text --title "Agent-001" "echo hi"

   # CORRECT: Parse JSON to find pane_id, then use it
   PANE_ID=$(wezterm cli list --format json | jq '.[] | select(.title=="Agent-001") | .pane_id')
   wezterm cli send-text --pane-id $PANE_ID "echo hi"
   ```

3. **Environment Context**: `$WEZTERM_PANE` unavailable from server
   ```bash
   # Outside WezTerm, must specify --pane-id or --new-window
   wezterm cli send-text --pane-id 0 "cmd"  # REQUIRED
   ```

4. **Polling Not Events**: No webhook or event system
   ```bash
   # Must poll regularly to detect changes
   for i in {1..100}; do
     wezterm cli list > current.json
     compare with previous.json
     sleep 2
   done
   ```

---

## Go Implementation Starting Point

From WEZTERM_QUICK_REFERENCE.md:

```go
package main

type WezTermPane struct {
    WindowID   int    `json:"window_id"`
    PaneID     int    `json:"pane_id"`
    Title      string `json:"title"`
    CWD        string `json:"cwd"`
    IsActive   bool   `json:"is_active"`
    TabTitle   string `json:"tab_title"`
}

func SpawnAgent(workDir, configName string) (int, error) {
    cmd := exec.Command("wezterm", "cli", "spawn",
        "--new-window", "--cwd", workDir, "--", "powershell")
    output, _ := cmd.Output()
    return strconv.Atoi(strings.TrimSpace(string(output)))
}

func ListPanes() ([]WezTermPane, error) {
    cmd := exec.Command("wezterm", "cli", "list", "--format", "json")
    output, _ := cmd.Output()
    var panes []WezTermPane
    json.Unmarshal(output, &panes)
    return panes
}
```

---

## Testing & Validation

### Quick Manual Test
```bash
# 1. Spawn agent
PANE_ID=$(wezterm cli spawn --new-window -- powershell)
echo "Spawned: $PANE_ID"

# 2. List to verify
wezterm cli list --format json | jq ".[].pane_id"

# 3. Send command
wezterm cli send-text --pane-id $PANE_ID "Write-Host 'Test'\n"

# 4. Set title
wezterm cli set-tab-title --pane-id $PANE_ID "TestAgent"

# 5. Kill
wezterm cli kill-pane --pane-id $PANE_ID

# 6. Verify it's gone
wezterm cli list --format json | jq "length"  # Should decrease
```

### Expected Results
- Spawn returns integer (pane ID)
- List shows new pane in JSON
- Title changes visible in UI
- Kill removes pane
- List no longer shows killed pane

---

## Next Steps

1. **Share this index** with your team
2. **Assign Phase 1 tasks** (spawner wrapper)
3. **Set up development environment** with WezTerm running
4. **Start with WEZTERM_QUICK_REFERENCE.md** for day-to-day coding
5. **Reference WEZTERM_CLI_RESEARCH.md** when you hit questions
6. **Follow WEZTERM_INTEGRATION_ROADMAP.md** for task sequencing

---

## Support & Questions

If you encounter issues:

1. **Check WEZTERM_CLI_RESEARCH.md**
   - § Limitations & Workarounds
   - § Command Reference (full details)

2. **Check WEZTERM_QUICK_REFERENCE.md**
   - § Error Scenarios
   - § Gotchas

3. **Test manually** using one-liners from QUICK_REF

4. **Enable WezTerm debug** with verbose logging:
   ```bash
   WEZTERM_LOG=trace wezterm cli list
   ```

5. **Check WezTerm official docs**: https://wezfurlong.org/wezterm/

---

**Research Complete**: All 6 commands tested successfully
**Documentation Status**: Ready for Implementation
**Quality**: Production-ready
**Last Updated**: 2025-12-20 15:57 UTC

**Start with**: WEZTERM_QUICK_REFERENCE.md for coding, WEZTERM_INTEGRATION_ROADMAP.md for planning
