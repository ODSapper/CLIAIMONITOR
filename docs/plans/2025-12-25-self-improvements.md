# CLIAIMONITOR Self-Improvements Plan
**Date**: 2025-12-25
**Status**: Ready for parallel execution

## Overview
Visual and UX improvements for CLIAIMONITOR including WezTerm styling, RTS-style spawn quotes, and hourly status announcements.

## Architecture Notes

### WezTerm Background Layers
- **Per-window/workspace**: `window_background_gradient` in Lua (current approach)
- **Per-pane OSC 11**: ANSI escape sequence `\x1b]11;#RRGGBB\x07` (spawner already uses this)
- **Random backgrounds**: Can cycle through a list in Lua or Go

### Current Pane Layout
```
Window 0 (CLITCOMMANDER workspace):
├── Tab 0: Captain tab
│   ├── Pane 0: Server output (top, ~50% height)
│   └── Pane 1: Captain Claude session (bottom, ~50% height)
└── Tab 1+: Agent tabs (up to 9 agents per tab in 3x3 grid)
```

---

## Tracks for Parallel Execution

### Track 1: Tab Bar Styling (WezTerm Lua)
**File**: `~/.wezterm.lua`
**Agent**: SNTGreen

Changes:
1. Set `tab_max_width = 60` (3x default ~20)
2. Add distinctive font for tabs (e.g., "Fira Code" bold or "Cascadia Code")
3. Add padding/spacing to tab titles

```lua
-- Tab width
config.tab_max_width = 60

-- Tab bar font (in window_frame)
config.window_frame.font = wezterm.font('Cascadia Code', { weight = 'Bold' })
config.window_frame.font_size = 12.0
```

---

### Track 2: Random Agent Backgrounds (WezTerm Lua)
**File**: `~/.wezterm.lua`
**Agent**: SNTPurple

Add a list of cyber/gradient backgrounds and select randomly based on tab index:

```lua
local agent_backgrounds = {
  { colors = { '#0f0c29', '#302b63', '#24243e' }, name = 'DeepSpace' },
  { colors = { '#1a2a6c', '#b21f1f', '#fdbb2d' }, name = 'Sunset' },
  { colors = { '#0f2027', '#203a43', '#2c5364' }, name = 'Ocean' },
  { colors = { '#200122', '#6f0000' }, name = 'Vampire' },
  { colors = { '#000000', '#434343' }, name = 'Graphite' },
  { colors = { '#0f0c29', '#1e3c72', '#0f0c29' }, name = 'Nebula' },
}

-- In update-status, select based on workspace or tab index
```

---

### Track 3: Server Pane Background (WezTerm Lua + Go)
**Files**: `~/.wezterm.lua`, `cmd/cliaimonitor/main.go`
**Agent**: SNTRed

The server pane (Pane 0) needs a distinct background from Captain (Pane 1).

**Lua approach**: Detect pane by title containing "Server" or by position
**Go approach**: Send OSC 11 sequence at startup for server pane

In main.go after server starts:
```go
// Set server pane background (dark matrix green)
fmt.Print("\x1b]11;#0a1f0a\x07")
```

---

### Track 4: RTS Spawn Quotes (Go)
**File**: `internal/agents/spawner.go`
**Agent**: SNTGreen

Add spawn quotes when agent is launched. Quotes displayed in server log.

```go
var spawnQuotes = []string{
    "Unit ready.",
    "Acknowledged.",
    "Standing by.",
    "At your service.",
    "Orders received.",
    "Ready to comply.",
    "Awaiting instructions.",
    "Online and operational.",
    "Reporting for duty.",
    "Systems nominal.",
    "Weapons hot.",
    "Let's rock.",
    "In position.",
    "Moving out.",
    "I live to serve.",
}

// In SpawnAgentWithOptions after successful spawn:
quote := spawnQuotes[rand.Intn(len(spawnQuotes))]
log.Printf("[SPAWNER] %s: \"%s\"", agentID, quote)
```

---

### Track 5: RTS Shutdown Quotes (Go)
**File**: `internal/agents/spawner.go`
**Agent**: SNTPurple

Add shutdown quotes when agent stops gracefully.

```go
var shutdownQuotes = []string{
    "Mission complete.",
    "Returning to base.",
    "Objective achieved.",
    "Signing off.",
    "Task finished.",
    "Going offline.",
    "Until next time.",
    "Power down sequence initiated.",
    "Farewell, commander.",
    "Work complete.",
    "Construction complete.",
    "Job's done!",
    "All clear.",
    "Package delivered.",
    "Target neutralized.",
}

// In StopAgentWithReason:
quote := shutdownQuotes[rand.Intn(len(shutdownQuotes))]
log.Printf("[SPAWNER] %s: \"%s\"", agentID, quote)
```

---

### Track 6: Hourly Uptime Announcements (Go)
**File**: `cmd/cliaimonitor/main.go`
**Agent**: SNTRed

Add a ticker goroutine that announces uptime every hour.

```go
var hourlyQuotes = []string{
    "All systems nominal.",
    "The spice must flow.",
    "Holding the line.",
    "Perimeter secure.",
    "Standing watch.",
    "Vigilance maintained.",
    "Still here, still watching.",
    "No anomalies detected.",
    "Defense grid online.",
    "Ready for anything.",
    "Peace through vigilance.",
    "The night is darkest before the dawn.",
}

// After server starts, spawn hourly ticker:
go func() {
    startTime := time.Now()
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()

    for range ticker.C {
        uptime := time.Since(startTime).Round(time.Minute)
        hours := int(uptime.Hours())
        quote := hourlyQuotes[rand.Intn(len(hourlyQuotes))]
        log.Printf("[UPTIME] %02d:00 - Running for %d hour(s). %s",
            time.Now().Hour(), hours, quote)
    }
}()
```

---

### Track 7: Server Pane Scroll Behavior (WezTerm Lua)
**File**: `~/.wezterm.lua`
**Agent**: HaikuGreen (simple task)

Ensure scrollback is enabled and scroll bar is visible:

```lua
config.scrollback_lines = 10000
config.enable_scroll_bar = true
config.min_scroll_bar_height = '2cell'

-- Enable mouse scrolling
config.mouse_bindings = {
  {
    event = { Down = { streak = 1, button = { WheelUp = 1 } } },
    mods = 'NONE',
    action = wezterm.action.ScrollByLine(-3),
  },
  {
    event = { Down = { streak = 1, button = { WheelDown = 1 } } },
    mods = 'NONE',
    action = wezterm.action.ScrollByLine(3),
  },
}
```

---

## Parallel Execution Plan

### Phase 1: WezTerm Styling (3 agents parallel)
| Track | Agent | File | Time |
|-------|-------|------|------|
| 1 | SNTGreen | wezterm.lua | ~10 min |
| 2 | SNTPurple | wezterm.lua | ~10 min |
| 7 | HaikuGreen | wezterm.lua | ~5 min |

**Note**: These all modify wezterm.lua so must be done SEQUENTIALLY or merged carefully.

### Phase 2: Go Code Changes (3 agents parallel)
| Track | Agent | File | Time |
|-------|-------|------|------|
| 4 | SNTGreen | spawner.go | ~10 min |
| 5 | SNTPurple | spawner.go | ~10 min |
| 6 | SNTRed | main.go | ~10 min |

**Note**: Tracks 4+5 both modify spawner.go - can be done by one agent.

### Optimized Plan: 2 Agents
| Agent | Tasks |
|-------|-------|
| SNTGreen | Tracks 1, 2, 3, 7 (all WezTerm Lua changes) |
| SNTPurple | Tracks 4, 5, 6 (all Go changes) |

---

## Quote Lists (For Reference)

### Spawn Quotes (RTS/Military style)
- "Unit ready." (C&C)
- "Acknowledged." (StarCraft)
- "Standing by." (Generic)
- "At your service." (Warcraft)
- "Orders received."
- "Ready to comply."
- "Awaiting instructions."
- "Online and operational."
- "Reporting for duty."
- "Systems nominal."
- "Weapons hot."
- "Let's rock." (StarCraft Marine)
- "In position."
- "Moving out."
- "I live to serve."
- "Born ready."
- "Lock and load."
- "Calibrating sensors."
- "Spooling up."
- "Neural link established."

### Shutdown Quotes
- "Mission complete."
- "Returning to base."
- "Objective achieved."
- "Signing off."
- "Task finished."
- "Going offline."
- "Until next time."
- "Power down sequence initiated."
- "Farewell, commander."
- "Work complete." (Warcraft Peon)
- "Construction complete." (C&C)
- "Job's done!" (Warcraft Peon)
- "All clear."
- "Package delivered."
- "Target neutralized."
- "End of line." (Tron)
- "Disengaging."
- "See you, space cowboy."

### Hourly Quotes
- "All systems nominal."
- "The spice must flow." (Dune)
- "Holding the line."
- "Perimeter secure."
- "Standing watch."
- "Vigilance maintained."
- "Still here, still watching."
- "No anomalies detected."
- "Defense grid online."
- "Ready for anything."
- "Peace through vigilance."
- "The night is darkest before the dawn."
- "Watching. Waiting."
- "Situation normal."
- "Coffee break? What's that?"
- "I never sleep."
