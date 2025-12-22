package agents

import (
	"fmt"
	"strings"
)

// AgentColors holds ANSI escape sequences for styling agent panes
type AgentColors struct {
	BgDark   string // Dark background tint for the entire pane (SGR)
	BgBright string // Bright background for the banner
	FgColor  string // Foreground color for text
	BgHex    string // Hex color for terminal default background (OSC 11)
	BgRGB    string // RGB format for OSC 11: "rgb:RR/GG/BB" (WezTerm compatible)
	Emoji    string // Emoji for quick visual identification
	Reset    string // Reset sequence to clear all formatting
}

// GetAgentColors returns the color scheme for a given agent configuration name
func GetAgentColors(configName string) AgentColors {
	lowerName := strings.ToLower(configName)
	reset := "\x1b[0m"

	switch {
	case strings.Contains(lowerName, "green"):
		return AgentColors{
			BgDark:   "\x1b[48;2;5;30;15m",      // Dark emerald background
			BgBright: "\x1b[48;2;34;197;94m",    // Bright emerald background
			FgColor:  "\x1b[38;2;34;197;94m",    // Emerald text
			BgHex:    "#051E0F",                 // Hex for OSC 11 terminal background
			BgRGB:    "rgb:05/1e/0f",            // RGB for WezTerm OSC 11
			Emoji:    "🟢",                       // Green circle
			Reset:    reset,
		}
	case strings.Contains(lowerName, "purple"):
		return AgentColors{
			BgDark:   "\x1b[48;2;20;10;35m",     // Dark violet background
			BgBright: "\x1b[48;2;168;85;247m",   // Bright violet background
			FgColor:  "\x1b[38;2;168;85;247m",   // Violet text
			BgHex:    "#140A23",                 // Hex for OSC 11 terminal background
			BgRGB:    "rgb:14/0a/23",            // RGB for WezTerm OSC 11
			Emoji:    "🟣",                       // Purple circle
			Reset:    reset,
		}
	case strings.Contains(lowerName, "red"):
		return AgentColors{
			BgDark:   "\x1b[48;2;35;10;10m",     // Dark rose background
			BgBright: "\x1b[48;2;239;68;68m",    // Bright rose background
			FgColor:  "\x1b[38;2;239;68;68m",    // Rose text
			BgHex:    "#230A0A",                 // Hex for OSC 11 terminal background
			BgRGB:    "rgb:23/0a/0a",            // RGB for WezTerm OSC 11
			Emoji:    "🔴",                       // Red circle
			Reset:    reset,
		}
	case strings.Contains(lowerName, "snake"):
		return AgentColors{
			BgDark:   "\x1b[48;2;5;25;30m",      // Dark cyan background
			BgBright: "\x1b[48;2;6;182;212m",    // Bright cyan background
			FgColor:  "\x1b[38;2;6;182;212m",    // Cyan text
			BgHex:    "#05191E",                 // Hex for OSC 11 terminal background
			BgRGB:    "rgb:05/19/1e",            // RGB for WezTerm OSC 11
			Emoji:    "🐍",                       // Snake emoji
			Reset:    reset,
		}
	case strings.Contains(lowerName, "captain"):
		return AgentColors{
			BgDark:   "\x1b[48;2;35;27;3m",      // Dark gold background
			BgBright: "\x1b[48;2;234;179;8m",    // Bright gold background
			FgColor:  "\x1b[38;2;234;179;8m",    // Gold text
			BgHex:    "#231B03",                 // Hex for OSC 11 terminal background
			BgRGB:    "rgb:23/1b/03",            // RGB for WezTerm OSC 11
			Emoji:    "⭐",                       // Star emoji
			Reset:    reset,
		}
	case strings.Contains(lowerName, "blue"):
		return AgentColors{
			BgDark:   "\x1b[48;2;2;25;35m",      // Dark sky background
			BgBright: "\x1b[48;2;14;165;233m",   // Bright sky background
			FgColor:  "\x1b[38;2;14;165;233m",   // Sky text
			BgHex:    "#021923",                 // Hex for OSC 11 terminal background
			BgRGB:    "rgb:02/19/23",            // RGB for WezTerm OSC 11
			Emoji:    "🔵",                       // Blue circle
			Reset:    reset,
		}
	default:
		return AgentColors{
			BgDark:   "\x1b[48;2;20;20;20m",     // Dark gray background
			BgBright: "\x1b[48;2;100;100;100m",  // Gray background
			FgColor:  "\x1b[38;2;200;200;200m",  // Light gray text
			BgHex:    "#141414",                 // Hex for OSC 11 terminal background
			BgRGB:    "rgb:14/14/14",            // RGB for WezTerm OSC 11
			Emoji:    "⚪",                       // White circle
			Reset:    reset,
		}
	}
}

// GenerateBanner creates a colored Unicode box banner for agent identification
// The banner displays at the top of the agent's pane when spawned
func GenerateBanner(agentID, configName, role string) string {
	colors := GetAgentColors(configName)

	// Black text on bright background for banner
	blackText := "\x1b[38;2;0;0;0m"

	// Create the banner with Unicode box-drawing characters
	banner := fmt.Sprintf("%s%s\n", colors.BgBright, blackText)
	banner += "╔══════════════════════════════════════════════════════╗\n"
	banner += fmt.Sprintf("║  %s %-10s │ %-20s          ║\n", colors.Emoji, role, configName)
	banner += fmt.Sprintf("║  Agent: %-44s ║\n", agentID)
	banner += "╚══════════════════════════════════════════════════════╝\n"
	banner += colors.Reset

	return banner
}

// GenerateBackgroundTint returns the ANSI escape sequence to set a subtle
// background tint for the entire pane based on agent type
func GenerateBackgroundTint(configName string) string {
	colors := GetAgentColors(configName)
	return colors.BgDark
}
