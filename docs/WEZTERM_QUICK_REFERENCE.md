# WezTerm CLI Quick Reference

**For CLIAIMONITOR Integration**

---

## One-Liners

```bash
# List all panes (JSON)
wezterm cli list --format json

# Spawn new agent window
PANE_ID=$(wezterm cli spawn --new-window --cwd "C:\project" -- powershell)

# Send command to pane
wezterm cli send-text --pane-id 0 "go build\n"

# Rename tab (visible in UI)
wezterm cli set-tab-title --pane-id 0 "Agent-SNTGreen"

# Rename window (taskbar title)
wezterm cli set-window-title --pane-id 0 "CLIAIMONITOR Agents"

# Close pane
wezterm cli kill-pane --pane-id 0

# Focus pane
wezterm cli activate-pane --pane-id 1

# Get pane text output
wezterm cli get-text --pane-id 0

# Split pane (right side)
PANE_ID=$(wezterm cli split-pane --pane-id 0 --right -- powershell)
```

---

## Key Answers

| Question | Answer |
|----------|--------|
| **Output format of `list`?** | JSON array of objects with fields: `pane_id`, `window_id`, `tab_id`, `title`, `cwd`, `is_active`, etc. |
| **Get pane ID when spawning?** | Yes - command outputs integer ID to stdout. Capture with `$(... spawn ...)` |
| **Difference between `spawn` and `split-pane`?** | `spawn` creates new window/tab (requires `--new-window` or `--window-id`); `split-pane` divides existing pane into two |
| **Target panes by title?** | No direct support. Parse JSON from `list` to find pane_id matching title, then use `--pane-id` |
| **What if WezTerm not running?** | `wezterm cli` auto-starts server. Use `--no-auto-start` to prevent this |
| **Set pane title?** | Use `set-tab-title` or `set-window-title` (no direct pane title) |
| **Query if pane alive?** | Parse `wezterm cli list --format json` and check if pane_id exists |
| **What's `--class`?** | Used with `wezterm start --class X` to run multiple instances; CLI needs matching `--class` to find it |

---

## Integration Checklist

- [ ] Capture pane ID from `spawn` command
- [ ] Store pane_id → agent_config mapping in database
- [ ] Use `list --format json` to monitor active panes
- [ ] Send commands via `send-text` with `\n` in command string
- [ ] Set `set-tab-title` immediately after spawn for visibility
- [ ] Poll `list` periodically to detect pane death
- [ ] Use `kill-pane` for graceful agent shutdown
- [ ] Parse JSON output in Go using json.Unmarshal()

---

## JSON Structure (from `list`)

```go
type WezTermPane struct {
    WindowID     int    `json:"window_id"`
    TabID        int    `json:"tab_id"`
    PaneID       int    `json:"pane_id"`
    Workspace    string `json:"workspace"`
    Size struct {
        Rows        int `json:"rows"`
        Cols        int `json:"cols"`
    } `json:"size"`
    Title        string `json:"title"`
    CWD          string `json:"cwd"`      // file:// URI format
    IsActive     bool   `json:"is_active"`
    IsZoomed     bool   `json:"is_zoomed"`
    TabTitle     string `json:"tab_title"`
    WindowTitle  string `json:"window_title"`
    CursorX      int    `json:"cursor_x"`
    CursorY      int    `json:"cursor_y"`
    LeftCol      int    `json:"left_col"`
    TopRow       int    `json:"top_row"`
    CursorShape  string `json:"cursor_shape"`
    TTYName      *string `json:"tty_name"`
}
```

---

## Error Scenarios

```bash
# Missing pane context (not in WezTerm pane, no --pane-id)
ERROR wezterm > --pane-id was not specified and $WEZTERM_PANE
is not set in the environment

# Solution: Use --new-window or explicitly provide --pane-id

# Pane doesn't exist
wezterm cli send-text --pane-id 999  # Fails silently or errors

# Solution: Check with `list` first
```

---

## Go Implementation Pattern

```go
package main

import (
    "encoding/json"
    "fmt"
    "os/exec"
    "strconv"
)

type WezTermPane struct {
    PaneID     int    `json:"pane_id"`
    Title      string `json:"title"`
    CWD        string `json:"cwd"`
    IsActive   bool   `json:"is_active"`
    TabTitle   string `json:"tab_title"`
}

// Spawn agent and return pane ID
func SpawnAgent(workDir, configName string) (int, error) {
    cmd := exec.Command("wezterm", "cli", "spawn",
        "--new-window",
        "--cwd", workDir,
        "--", "powershell")

    output, err := cmd.Output()
    if err != nil {
        return 0, err
    }

    paneID, err := strconv.Atoi(string(output))
    return paneID, err
}

// List all panes
func ListPanes() ([]WezTermPane, error) {
    cmd := exec.Command("wezterm", "cli", "list", "--format", "json")
    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }

    var panes []WezTermPane
    err = json.Unmarshal(output, &panes)
    return panes, err
}

// Send command to pane
func SendCommand(paneID int, cmd string) error {
    return exec.Command("wezterm", "cli", "send-text",
        "--pane-id", strconv.Itoa(paneID),
        cmd+"\n").Run()
}

// Set pane title
func SetTitle(paneID int, title string) error {
    return exec.Command("wezterm", "cli", "set-tab-title",
        "--pane-id", strconv.Itoa(paneID),
        title).Run()
}
```

---

## Common Tasks

### Spawn agent with proper setup
```bash
PANE_ID=$(wezterm cli spawn --new-window --cwd "C:\project" -- powershell)
wezterm cli set-tab-title --pane-id $PANE_ID "Agent-001"
wezterm cli send-text --pane-id $PANE_ID "# Starting agent process\n"
```

### Check if pane alive
```bash
if wezterm cli list --format json | jq "any(.pane_id == $PANE_ID)" | grep -q true; then
    echo "Alive"
else
    echo "Dead"
fi
```

### Get pane output
```bash
wezterm cli get-text --pane-id $PANE_ID | tail -20
```

### Kill all agents
```bash
wezterm cli list --format json | \
  jq -r '.[] | select(.tab_title | contains("Agent")) | .pane_id' | \
  xargs -I {} wezterm cli kill-pane --pane-id {}
```

### Side-by-side agents
```bash
P1=$(wezterm cli spawn --new-window -- cmd)
P2=$(wezterm cli split-pane --pane-id $P1 --right -- cmd)
wezterm cli set-window-title --pane-id $P1 "Dual Agents"
```

---

## Gotchas

1. **Text doesn't auto-newline** - Always include `\n` in send-text
2. **Pane ID only from spawn** - Must use `list` to find existing panes later
3. **No pane events** - Must poll with `list` to detect state changes
4. **CWD in JSON is file:// URI** - Convert to normal path for string comparison
5. **WEZTERM_PANE only inside WezTerm** - CLIAIMONITOR server needs explicit `--pane-id`
6. **Multiple WezTerm instances** - Use `--class` flag to distinguish

---

## Resource Links

- **Full Research**: `docs/WEZTERM_CLI_RESEARCH.md`
- **WezTerm Docs**: https://wezfurlong.org/wezterm/index.html
- **CLI Docs**: https://wezfurlong.org/wezterm/cli/index.html
