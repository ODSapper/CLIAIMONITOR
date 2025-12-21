# Agent Pane Visual Distinction Plan

**Date:** 2025-12-21
**Status:** DRAFT
**Author:** Captain

## Overview

Make each agent visually distinct when spawned in the WezTerm pane grid through colored backgrounds, banners, and styling.

## WezTerm Limitations

Per the [WezTerm documentation](https://wezterm.org/config/appearance.html) and [GitHub discussions](https://github.com/wezterm/wezterm/discussions/3337):

- **Per-pane border colors are NOT supported** - `split` color is global
- **Per-pane config overrides are NOT supported**
- **ANSI escape sequences CAN change pane colors** - This is our path forward

## Color Scheme

| Agent Type | Color Name | Hex Code | RGB |
|------------|------------|----------|-----|
| Green (Implementation) | Emerald | `#22c55e` | 34, 197, 94 |
| Purple (Review) | Violet | `#a855f7` | 168, 85, 247 |
| Red (Security) | Rose | `#ef4444` | 239, 68, 68 |
| Captain | Gold | `#eab308` | 234, 179, 8 |
| Snake (Recon) | Cyan | `#06b6d4` | 6, 182, 212 |
| Blue (Testing) | Sky | `#0ea5e9` | 14, 165, 233 |

## Implementation Approach

### Option A: Background Tint (Recommended)

Set a subtle background color when the pane spawns:

```bash
# ANSI escape sequence to set background color (24-bit true color)
# Format: \e[48;2;R;G;Bm
# For subtle tint, use dark versions of the colors

# Green agent - dark emerald background
echo -e "\e[48;2;5;30;15m"

# Purple agent - dark violet background
echo -e "\e[48;2;20;10;35m"

# Red agent - dark rose background
echo -e "\e[48;2;35;10;10m"
```

**Pros:**
- Entire pane is visually distinct
- Works with any terminal content
- Subtle but noticeable

**Cons:**
- May affect readability if too strong
- Need to balance visibility vs usability

### Option B: Colored Banner on Spawn

Echo a colored header when the agent starts:

```bash
# Green agent banner
echo -e "\e[48;2;34;197;94m\e[38;2;0;0;0m"
echo "╔════════════════════════════════════════════════╗"
echo "║  SGT GREEN - Implementation Agent              ║"
echo "║  Agent ID: team-sntgreen001                    ║"
echo "╚════════════════════════════════════════════════╝"
echo -e "\e[0m"
```

**Pros:**
- Clear identification at spawn
- Doesn't affect ongoing readability
- Can include agent metadata

**Cons:**
- Scrolls away during long sessions
- Only visible at top of buffer

### Option C: Colored Border Box (Hybrid)

Draw a persistent colored border using Unicode box-drawing:

```
┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃  🟢 SGT GREEN | team-sntgreen001 | WORKING     ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
```

### Recommended: Combine A + B

1. **Set subtle background tint** for the entire pane
2. **Show colored banner** with agent info at spawn
3. **Use colored emoji** in the banner for quick identification

## Implementation in Spawner

Update `internal/agents/spawner.go` to inject color sequences:

```go
// getAgentColor returns ANSI escape codes for agent type
func getAgentColor(configName string) (bgDark, bgBright, fgColor string) {
    switch {
    case strings.Contains(strings.ToLower(configName), "green"):
        return "\x1b[48;2;5;30;15m", "\x1b[48;2;34;197;94m", "\x1b[38;2;34;197;94m"
    case strings.Contains(strings.ToLower(configName), "purple"):
        return "\x1b[48;2;20;10;35m", "\x1b[48;2;168;85;247m", "\x1b[38;2;168;85;247m"
    case strings.Contains(strings.ToLower(configName), "red"):
        return "\x1b[48;2;35;10;10m", "\x1b[48;2;239;68;68m", "\x1b[38;2;239;68;68m"
    case strings.Contains(strings.ToLower(configName), "snake"):
        return "\x1b[48;2;5;25;30m", "\x1b[48;2;6;182;212m", "\x1b[38;2;6;182;212m"
    default:
        return "\x1b[48;2;20;20;20m", "\x1b[48;2;100;100;100m", "\x1b[38;2;200;200;200m"
    }
}

// generateBanner creates the startup banner for an agent
func generateBanner(agentID, configName, role string) string {
    emoji := getAgentEmoji(configName)
    bgDark, bgBright, _ := getAgentColor(configName)
    reset := "\x1b[0m"

    return fmt.Sprintf(`%s%s
╔══════════════════════════════════════════════════════╗
║  %s %-10s │ %-20s          ║
║  Agent: %-44s ║
╚══════════════════════════════════════════════════════╝
%s%s`,
        bgBright, "\x1b[38;2;0;0;0m",
        emoji, role, configName,
        agentID,
        reset, bgDark)
}

func getAgentEmoji(configName string) string {
    switch {
    case strings.Contains(strings.ToLower(configName), "green"):
        return "🟢"
    case strings.Contains(strings.ToLower(configName), "purple"):
        return "🟣"
    case strings.Contains(strings.ToLower(configName), "red"):
        return "🔴"
    case strings.Contains(strings.ToLower(configName), "snake"):
        return "🐍"
    default:
        return "⚪"
    }
}
```

## WezTerm Split Command Update

When spawning panes, send the banner as the first command:

```go
// In SpawnAgentPane or equivalent
bannerCmd := generateBanner(agentID, configName, role)
weztermArgs := []string{
    "cli", "split-pane",
    "--", "cmd", "/c",
    fmt.Sprintf("echo %s && claude ...", bannerCmd),
}
```

## Visual Mockup

```
┌──────────────────────────────┬──────────────────────────────┐
│ ╔════════════════════════╗   │ ╔════════════════════════╗   │
│ ║ 🟢 SGT GREEN           ║   │ ║ 🟣 SGT PURPLE          ║   │
│ ║ team-sntgreen001       ║   │ ║ team-sntpurple001      ║   │
│ ╚════════════════════════╝   │ ╚════════════════════════╝   │
│ (dark green background)      │ (dark purple background)     │
│                              │                              │
│ > Working on task...         │ > Reviewing code...          │
│                              │                              │
├──────────────────────────────┼──────────────────────────────┤
│ ╔════════════════════════╗   │ ╔════════════════════════╗   │
│ ║ 🔴 SGT RED             ║   │ ║ 🐍 SNAKE               ║   │
│ ║ team-sntred001         ║   │ ║ team-snake001          ║   │
│ ╚════════════════════════╝   │ ╚════════════════════════╝   │
│ (dark red background)        │ (dark cyan background)       │
│                              │                              │
│ > Security analysis...       │ > Scanning codebase...       │
│                              │                              │
└──────────────────────────────┴──────────────────────────────┘
```

## Testing Plan

1. Spawn each agent type manually with color test
2. Verify background tint is visible but not distracting
3. Verify banner displays correctly with Unicode
4. Test in different WezTerm themes (dark/light)
5. Check color accessibility (contrast ratios)

## Files to Modify

| File | Changes |
|------|---------|
| `internal/agents/spawner.go` | Add color/banner generation functions |
| `internal/agents/configs.go` | Add color field to AgentConfig |
| `configs/teams.yaml` | Add color definitions per agent type |

## Future Enhancements

1. **Dynamic status bar** - Update pane title with current status
2. **Activity indicator** - Blinking cursor color when active
3. **Progress bar** - Show task completion in title
4. **Tab coloring** - If using tabs, color the tab bar

---

## References

- [WezTerm Colors & Appearance](https://wezterm.org/config/appearance.html)
- [GitHub: Pane styling options request](https://github.com/wezterm/wezterm/issues/297)
- [GitHub: Active pane border color discussion](https://github.com/wezterm/wezterm/discussions/3337)
