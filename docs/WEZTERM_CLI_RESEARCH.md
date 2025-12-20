# WezTerm CLI Research & Integration Guide

**Date**: 2025-12-20
**WezTerm Version**: 20240203-110809-5046fc22
**Purpose**: Explore WezTerm CLI capabilities for CLIAIMONITOR agent spawning

---

## Executive Summary

WezTerm provides a powerful CLI interface (`wezterm cli`) for interacting with terminals programmatically. Key capabilities:

- **Pane Management**: Spawn, split, kill, and list panes with automatic ID generation
- **JSON Output**: Full JSON support for structured data queries
- **Direct Text Input**: Send text directly to panes (with bracketed paste support)
- **Tab/Window Management**: Rename tabs, windows, and activate panes
- **Workspace Support**: Organize panes across workspaces

This makes WezTerm ideal for CLIAIMONITOR's agent spawning workflow.

---

## Command Reference

### 1. `wezterm cli list` - Query Panes/Tabs/Windows

**Purpose**: List all active windows, tabs, and panes

**Table Format Output** (default):
```
WINID TABID PANEID WORKSPACE SIZE  TITLE   CWD
    0     0      0 default   80x24 cmd.exe file:///C:/Users/Admin/
```

**JSON Format Output**:
```bash
wezterm cli list --format json
```

**JSON Response**:
```json
[
  {
    "window_id": 0,
    "tab_id": 0,
    "pane_id": 0,
    "workspace": "default",
    "size": {
      "rows": 24,
      "cols": 80,
      "pixel_width": 640,
      "pixel_height": 384,
      "dpi": 0
    },
    "title": "cmd.exe",
    "cwd": "file:///C:/Users/Admin/",
    "cursor_x": 15,
    "cursor_y": 3,
    "cursor_shape": "Default",
    "cursor_visibility": "Visible",
    "left_col": 0,
    "top_row": 0,
    "tab_title": "",
    "window_title": "",
    "is_active": true,
    "is_zoomed": false,
    "tty_name": null
  }
]
```

**Key Fields**:
- `pane_id`: Unique identifier for pane (used in other CLI commands)
- `window_id`: Parent window ID
- `tab_id`: Parent tab ID
- `workspace`: Workspace name (default is "default")
- `title`: Current pane title (usually the running program name)
- `cwd`: Current working directory as file:// URI
- `is_active`: Boolean indicating if pane is focused
- `tab_title`: Custom tab title (can be set with `set-tab-title`)
- `window_title`: Custom window title (can be set with `set-window-title`)

**Options**:
- `--format json|table` - Output format (default: table)

**Use Case**: Query active panes to monitor agent status or find specific pane IDs

---

### 2. `wezterm cli spawn` - Create New Panes/Tabs/Windows

**Purpose**: Spawn a command in a new window or tab, returns pane ID on success

**Basic Usage**:
```bash
wezterm cli spawn -- cmd /c "echo hello"
```

**Important Error When No Pane Context**:
```
ERROR wezterm > --pane-id was not specified and $WEZTERM_PANE
is not set in the environment, and I couldn't determine which pane was currently focused; terminating
```

**Options**:

| Option | Purpose | Notes |
|--------|---------|-------|
| `--pane-id <PANE_ID>` | Specify source pane (for tab spawning) | Use this when spawning into existing window |
| `--window-id <WINDOW_ID>` | Spawn into specific window | Creates new tab in window |
| `--new-window` | Create new window instead of tab | Default behavior is new tab |
| `--cwd <CWD>` | Set initial working directory | Absolute path (not file:// URI) |
| `--workspace <WORKSPACE>` | Name for new workspace | Only with `--new-window` |
| `--domain-name <DOMAIN_NAME>` | Specify domain for remote execution | Advanced feature |
| `[PROG]...` | Command to execute | Example: `bash -l` or `powershell` |

**Return Value**:
- **On Success**: Outputs pane ID as integer (e.g., `2`)
- **On Failure**: Error message to stderr with exit code 1

**Examples**:

**Spawn in new window with working directory**:
```bash
wezterm cli spawn --new-window --cwd "C:\path\to\project" -- powershell
```

**Spawn in existing window (creates tab)**:
```bash
wezterm cli spawn --window-id 0 -- bash -l
```

**Spawn and capture pane ID**:
```bash
PANE_ID=$(wezterm cli spawn --new-window -- cmd /c "start /wait powershell")
echo "Agent spawned in pane: $PANE_ID"
```

**Key Behaviors**:
- Returns pane ID as plaintext integer on success
- Requires either `--pane-id`, `--window-id`, or `--new-window`
- Without explicit location, fails with error (no auto-detection outside WezTerm)
- Default is new tab in current window (requires `--pane-id` or `--window-id`)

---

### 3. `wezterm cli split-pane` - Split Existing Pane

**Purpose**: Split a pane into two, returns pane ID for new pane

**Basic Usage**:
```bash
wezterm cli split-pane --pane-id=0 --right -- powershell
```

**Options**:

| Option | Purpose |
|--------|---------|
| `--pane-id <PANE_ID>` | Pane to split (required or WEZTERM_PANE env var) |
| `--horizontal` | Alias for `--right` |
| `--right` | Split horizontally, new pane on right |
| `--left` | Split horizontally, new pane on left |
| `--top` | Split vertically, new pane on top |
| `--bottom` | Split vertically, new pane on bottom (default) |
| `--top-level` | Split entire window instead of active pane |
| `--cells <CELLS>` | Fixed size in cells (not percentage) |
| `--percent <PERCENT>` | Size as percentage of available space |
| `--cwd <CWD>` | Initial working directory |
| `--move-pane-id <MOVE_PANE_ID>` | Move existing pane instead of spawning new |
| `[PROG]...` | Command to execute |

**Return Value**: Pane ID of newly created pane

**Comparison to Spawn**:

| Feature | spawn | split-pane |
|---------|-------|-----------|
| Creates new window | Yes (`--new-window`) | No |
| Creates new tab | Yes (default) | No |
| Splits existing pane | No | Yes |
| Can rearrange panes | No | Yes (`--move-pane-id`) |
| Requires existing pane | Only for context | Yes (to split) |
| Control split direction | N/A | Yes (4 directions) |

**Use Case**: Create side-by-side agents or dashboard layouts

---

### 4. `wezterm cli send-text` - Send Input to Pane

**Purpose**: Send text to a pane as if pasted (respects bracketed paste mode)

**Basic Usage**:
```bash
wezterm cli send-text --pane-id=0 "dir"
```

**Input Methods**:
1. **Command-line argument**: `wezterm cli send-text --pane-id=0 "echo hello"`
2. **Stdin**: `echo "hello" | wezterm cli send-text --pane-id=0`

**Options**:

| Option | Purpose |
|--------|---------|
| `--pane-id <PANE_ID>` | Target pane (default: WEZTERM_PANE env var) |
| `--no-paste` | Send raw text, not bracketed paste |
| `[TEXT]` | Text to send (optional, reads from stdin if omitted) |

**Bracketed Paste Mode**:
- **Default**: Text is sent in bracketed paste mode (if enabled in pane)
- Allows applications to distinguish pasted text from keyboard input
- Most shells and editors support this
- Use `--no-paste` to bypass this behavior

**Examples**:

**Send newline**:
```bash
wezterm cli send-text --pane-id=0 "command" && wezterm cli send-text --pane-id=0 ""
```
Note: Newlines don't auto-send, must send as part of text or separately

**Send command with execution**:
```bash
# In Windows PowerShell
wezterm cli send-text --pane-id=0 "Write-Host 'test'`n"
```

**Pipe multiline script**:
```bash
cat << 'EOF' | wezterm cli send-text --pane-id=0
cd C:\projects
go build
EOF
```

**Key Behaviors**:
- Text is pasted, not sent character-by-character
- Respects bracketed paste mode for proper escape sequences
- Does NOT send Enter/newline automatically
- No wait for command completion

---

### 5. `wezterm cli set-tab-title` - Rename Tab

**Purpose**: Change the display title of a tab

**Basic Usage**:
```bash
wezterm cli set-tab-title --pane-id=0 "Agent-SNTGreen"
```

**Options**:

| Option | Purpose |
|--------|---------|
| `--tab-id <TAB_ID>` | Target tab by ID (direct) |
| `--pane-id <PANE_ID>` | Use pane to find tab (default: WEZTERM_PANE) |
| `<TITLE>` | New tab title (required) |

**Examples**:
```bash
wezterm cli set-tab-title --pane-id=0 "Agent-001-SNTGreen"
wezterm cli set-tab-title --tab-id=0 "Agent-Processing"
```

**Behavior**:
- Updates the visible tab title in UI
- Does not affect pane title or window title
- Can use any string, no length restrictions observed

---

### 6. `wezterm cli set-window-title` - Rename Window

**Purpose**: Change the title of a window (appears in taskbar/WM)

**Basic Usage**:
```bash
wezterm cli set-window-title --pane-id=0 "CLIAIMONITOR Agents"
```

**Options**:

| Option | Purpose |
|--------|---------|
| `--window-id <WINDOW_ID>` | Target window by ID |
| `--pane-id <PANE_ID>` | Use pane to find window (default: WEZTERM_PANE) |
| `<TITLE>` | New window title (required) |

---

### 7. `wezterm cli kill-pane` - Close a Pane

**Purpose**: Terminate a pane and its running process

**Basic Usage**:
```bash
wezterm cli kill-pane --pane-id=2
```

**Options**:

| Option | Purpose |
|--------|---------|
| `--pane-id <PANE_ID>` | Pane to kill (default: WEZTERM_PANE) |

**Behavior**:
- Sends SIGTERM to pane process (graceful shutdown)
- If process exits, pane closes
- If process ignores SIGTERM, pane may remain
- No return value on success
- Returns exit code 0 on success, 1 on failure

**Error Handling**:
- If pane doesn't exist: Error to stderr
- If process won't die: Consider `--force` (if available in newer versions)

---

### 8. `wezterm cli activate-pane` - Focus a Pane

**Purpose**: Move keyboard focus to a specific pane

**Basic Usage**:
```bash
wezterm cli activate-pane --pane-id=1
```

**Options**:

| Option | Purpose |
|--------|---------|
| `--pane-id <PANE_ID>` | Pane to activate (required or WEZTERM_PANE) |

**Use Case**: Switch focus between agent terminals

---

### 9. `wezterm cli list-clients` - List Connected Clients

**Purpose**: List all client connections to WezTerm mux server

**Table Format**:
```bash
wezterm cli list-clients
```

**JSON Format**:
```bash
wezterm cli list-clients --format json
```

**Example JSON Output**:
```json
[]
```
(Empty when no remote clients connected; local GUI doesn't count)

**Options**:
- `--format json|table` - Output format

**Use Case**: Health checks for remote agent connections

---

### 10. Additional Commands

**`wezterm cli activate-tab`**
```bash
wezterm cli activate-tab --tab-id=1
```
Switches to a specific tab.

**`wezterm cli get-text`**
```bash
wezterm cli get-text --pane-id=0
```
Retrieves all text content from a pane (useful for reading output).

**`wezterm cli zoom-pane`**
```bash
wezterm cli zoom-pane --pane-id=0  # Toggle zoom
wezterm cli zoom-pane --pane-id=0 --toggle
```
Zoom or unzoom a pane.

**`wezterm cli adjust-pane-size`**
```bash
wezterm cli adjust-pane-size --pane-id=0 --increase-width 5
```
Adjust pane size directionally.

**`wezterm cli activate-pane-direction`** / **`get-pane-direction`**
```bash
wezterm cli activate-pane-direction --pane-id=0 Right
```
Navigate panes by direction.

---

## Key Integration Insights

### 1. Pane ID Acquisition

**When spawning**:
```bash
PANE_ID=$(wezterm cli spawn --new-window -- powershell)
# Returns: integer ID
```

**When querying**:
```bash
PANES=$(wezterm cli list --format json)
# Parse JSON to extract pane_ids
```

**Limitation**: Cannot assign custom pane names. IDs are auto-generated by WezTerm.

### 2. Environment Context

**Current Pane Detection**:
- WezTerm sets `$WEZTERM_PANE` environment variable inside terminal panes
- CLI commands default to this if `--pane-id` not specified
- Outside WezTerm (e.g., from CLIAIMONITOR server), must explicitly provide `--pane-id` or `--new-window`

**Implication**: CLIAIMONITOR spawner needs to:
1. Use `--new-window` or capture returned pane ID
2. Store pane ID for future commands
3. Pass `--pane-id` to all subsequent operations

### 3. JSON Output Parsing

**All commands support JSON**:
- `list --format json` - Most detailed, returns array of objects
- `list-clients --format json` - Returns array of client objects

**Recommended approach**:
```go
// Pseudocode
output := exec.Command("wezterm", "cli", "list", "--format", "json").Output()
var panes []PaneInfo
json.Unmarshal(output, &panes)
for _, pane := range panes {
    if pane.Title == "Agent-SNTGreen" {
        targetPaneID = pane.PaneID
    }
}
```

### 4. Text Input Strategy

**Challenge**: No auto-newline

**Solution**: Build command strings with newlines:
```go
cmd := "cd C:\\path && go build\n"
exec.Command("wezterm", "cli", "send-text", "--pane-id", paneID, cmd).Run()
```

Or multiple sends:
```go
wezterm send-text --pane-id=0 "command"
wezterm send-text --pane-id=0 "\n"
```

**For PowerShell**: Use backtick escape:
```powershell
wezterm cli send-text --pane-id=0 "Write-Host 'done'`n"
```

### 5. Pane Lifecycle Monitoring

**Check if pane exists**:
```bash
wezterm cli list --format json | jq '.[] | select(.pane_id == 5)'
```

**Watch pane status** (manual polling):
```bash
while [ $(wezterm cli list --format json | jq '.[] | select(.pane_id == 5) | length') -gt 0 ]; do
  sleep 1
done
echo "Pane died"
```

**Current limitation**: No built-in pane death notification. Must poll `list`.

---

## Recommended Integration Patterns for CLIAIMONITOR

### Pattern 1: Agent Spawning with ID Tracking

```bash
# Spawn new agent window
PANE_ID=$(wezterm cli spawn --new-window --cwd "C:/project" -- powershell)

# Set readable title
wezterm cli set-tab-title --pane-id "$PANE_ID" "Agent-SNTGreen-001"

# Send initial command
wezterm cli send-text --pane-id "$PANE_ID" "cd C:\\project\n"

# Store mapping for future reference
echo "$PANE_ID:SNTGreen" >> agent_panes.txt
```

### Pattern 2: Parallel Agent Grid

```bash
# Spawn 3 agents in new window with split layout
PANE1=$(wezterm cli spawn --new-window -- cmd)
PANE2=$(wezterm cli split-pane --pane-id "$PANE1" --right -- cmd)
PANE3=$(wezterm cli split-pane --pane-id "$PANE2" --bottom -- cmd)

# Set titles
wezterm cli set-tab-title --pane-id "$PANE1" "Agents Layout"
wezterm cli set-window-title --pane-id "$PANE1" "CLIAIMONITOR - Active Agents"
```

### Pattern 3: Health Monitoring

```bash
# Check if agent pane still exists
if wezterm cli list --format json | jq -e ".[] | select(.pane_id == $PANE_ID)" > /dev/null; then
    echo "Agent alive"
    # Get status
    TITLE=$(wezterm cli list --format json | jq -r ".[] | select(.pane_id == $PANE_ID) | .title")
else
    echo "Agent died"
fi
```

### Pattern 4: Command Execution with Feedback

```bash
# Send command
wezterm cli send-text --pane-id "$PANE_ID" "go build && echo DONE\n"

# Poll for completion (check pane output)
while [ $(wezterm cli get-text --pane-id "$PANE_ID" | grep -c "DONE") -eq 0 ]; do
    sleep 1
done
```

---

## Limitations & Workarounds

| Limitation | Impact | Workaround |
|-----------|--------|-----------|
| No async pane events | Must poll for state changes | Use `wezterm cli list` in loop |
| No custom pane IDs | IDs are opaque integers | Map pane_id → agent config in DB/cache |
| Text input lacks newline auto-send | Must manually include `\n` | Format commands as "cmd1\ncmd2\n" |
| No built-in pane naming | Can set title, not ID | Use tab/window titles + parsing |
| Environment context outside WezTerm | `$WEZTERM_PANE` unavailable | Always use `--new-window` or explicit `--pane-id` |
| `list-clients` empty for GUI | Can't detect remote connections via CLI | Consider separate heartbeat mechanism |
| No process status visibility | Can't distinguish "running" vs "idle" | Send marker commands and parse output |

---

## Command Availability Summary

| Command | Status | Output | Notes |
|---------|--------|--------|-------|
| `list` | ✅ Full | Table/JSON | Returns detailed pane array |
| `spawn` | ✅ Full | Pane ID | Returns integer on success |
| `split-pane` | ✅ Full | Pane ID | Directional splits, size control |
| `send-text` | ✅ Full | None | Bracketed paste support |
| `set-tab-title` | ✅ Full | None | Updates visible tab title |
| `set-window-title` | ✅ Full | None | Updates taskbar/WM title |
| `kill-pane` | ✅ Full | None | Graceful termination |
| `activate-pane` | ✅ Full | None | Focuses pane |
| `get-text` | ✅ Full | Text | Returns pane content |
| `list-clients` | ✅ Full | Table/JSON | Empty in GUI mode |
| `zoom-pane` | ✅ Full | None | Toggle/zoom pane |

---

## WezTerm Startup Considerations

**Starting WezTerm server** (if not auto-started):
```bash
wezterm start
```

**Connecting from scripts** (if using mux server):
```bash
wezterm cli --prefer-mux list
```

**For GUI instance with custom class**:
```bash
wezterm --class CLIAIMONITOR &
# Later, in CLI commands:
wezterm cli --class CLIAIMONITOR list
```

**Auto-start behavior**:
- `wezterm cli` commands auto-start the server if needed
- `--no-auto-start` flag prevents this
- Works even if WezTerm GUI isn't visible

---

## Example: Complete Agent Spawn Flow

```bash
#!/bin/bash

# Configuration
PROJECT_PATH="C:\projects\my-agent"
AGENT_CONFIG="SNTGreen"
TASK="Implement new feature"

# 1. Spawn new window
echo "Spawning agent..."
PANE_ID=$(wezterm cli spawn --new-window --cwd "$PROJECT_PATH" -- powershell)
echo "Agent spawned in pane: $PANE_ID"

# 2. Set identifiable title
wezterm cli set-tab-title --pane-id "$PANE_ID" "Agent-${AGENT_CONFIG}-${PANE_ID}"
wezterm cli set-window-title --pane-id "$PANE_ID" "CLIAIMONITOR Agent: $AGENT_CONFIG"

# 3. Setup environment
wezterm cli send-text --pane-id "$PANE_ID" "# Spawned for task: $TASK\n"
wezterm cli send-text --pane-id "$PANE_ID" "Write-Host 'Agent ready at $(Get-Date)'\n"

# 4. Start agent process
wezterm cli send-text --pane-id "$PANE_ID" "claude --task '$TASK'\n"

# 5. Store mapping
echo "$PANE_ID|$AGENT_CONFIG|$PROJECT_PATH|$(date)" >> agents.log

# 6. Activate pane to show it
wezterm cli activate-pane --pane-id "$PANE_ID"

echo "Agent $AGENT_CONFIG started successfully"
```

---

## Testing Commands Used

All commands in this document were tested against WezTerm version **20240203-110809-5046fc22** on Windows 11 with the CLI working correctly for:

- ✅ `list` with JSON output
- ✅ `spawn` with `--new-window`
- ✅ `send-text` with valid pane ID
- ✅ Title setting commands
- ✅ Error handling when pane context missing

---

## Conclusion

WezTerm CLI provides excellent capabilities for CLIAIMONITOR:
- Clean JSON APIs for integration
- Reliable pane spawning with ID return
- Text I/O for sending commands
- Title/metadata management for UI feedback

Primary integration points:
1. Use `spawn` to create agent panes and capture IDs
2. Use `list` to monitor active agents
3. Use `send-text` to execute commands
4. Use `set-tab-title` to provide visible labels

The main development effort will be:
- Building Go wrapper around these CLI calls
- Mapping pane IDs to agent configs in persistent storage
- Implementing polling loop for pane state monitoring
- Parsing JSON output for dashboard updates
