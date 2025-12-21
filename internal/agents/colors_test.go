package agents

import (
	"strings"
	"testing"
)

func TestGetAgentColors(t *testing.T) {
	tests := []struct {
		name        string
		configName  string
		wantEmoji   string
		wantBgDark  string
		wantBgBright string
	}{
		{
			name:        "Green agent",
			configName:  "SNTGreen",
			wantEmoji:   "🟢",
			wantBgDark:  "\x1b[48;2;5;30;15m",
			wantBgBright: "\x1b[48;2;34;197;94m",
		},
		{
			name:        "Green agent case insensitive",
			configName:  "haikugreen",
			wantEmoji:   "🟢",
			wantBgDark:  "\x1b[48;2;5;30;15m",
			wantBgBright: "\x1b[48;2;34;197;94m",
		},
		{
			name:        "Purple agent",
			configName:  "SNTPurple",
			wantEmoji:   "🟣",
			wantBgDark:  "\x1b[48;2;20;10;35m",
			wantBgBright: "\x1b[48;2;168;85;247m",
		},
		{
			name:        "Red agent",
			configName:  "SNTRed",
			wantEmoji:   "🔴",
			wantBgDark:  "\x1b[48;2;35;10;10m",
			wantBgBright: "\x1b[48;2;239;68;68m",
		},
		{
			name:        "Snake agent",
			configName:  "Snake",
			wantEmoji:   "🐍",
			wantBgDark:  "\x1b[48;2;5;25;30m",
			wantBgBright: "\x1b[48;2;6;182;212m",
		},
		{
			name:        "Captain agent",
			configName:  "Captain",
			wantEmoji:   "⭐",
			wantBgDark:  "\x1b[48;2;35;27;3m",
			wantBgBright: "\x1b[48;2;234;179;8m",
		},
		{
			name:        "Blue agent",
			configName:  "BlueTest",
			wantEmoji:   "🔵",
			wantBgDark:  "\x1b[48;2;2;25;35m",
			wantBgBright: "\x1b[48;2;14;165;233m",
		},
		{
			name:        "Unknown agent defaults to gray",
			configName:  "UnknownAgent",
			wantEmoji:   "⚪",
			wantBgDark:  "\x1b[48;2;20;20;20m",
			wantBgBright: "\x1b[48;2;100;100;100m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAgentColors(tt.configName)

			if got.Emoji != tt.wantEmoji {
				t.Errorf("GetAgentColors(%q).Emoji = %q, want %q", tt.configName, got.Emoji, tt.wantEmoji)
			}
			if got.BgDark != tt.wantBgDark {
				t.Errorf("GetAgentColors(%q).BgDark = %q, want %q", tt.configName, got.BgDark, tt.wantBgDark)
			}
			if got.BgBright != tt.wantBgBright {
				t.Errorf("GetAgentColors(%q).BgBright = %q, want %q", tt.configName, got.BgBright, tt.wantBgBright)
			}
			if got.Reset != "\x1b[0m" {
				t.Errorf("GetAgentColors(%q).Reset = %q, want %q", tt.configName, got.Reset, "\x1b[0m")
			}
			if got.FgColor == "" {
				t.Errorf("GetAgentColors(%q).FgColor should not be empty", tt.configName)
			}
		})
	}
}

func TestGenerateBanner(t *testing.T) {
	tests := []struct {
		name       string
		agentID    string
		configName string
		role       string
		wantEmoji  string
		wantID     bool
		wantRole   bool
	}{
		{
			name:       "Green SGT banner",
			agentID:    "team-sntgreen001",
			configName: "SNTGreen",
			role:       "Go Developer",
			wantEmoji:  "🟢",
			wantID:     true,
			wantRole:   true,
		},
		{
			name:       "Purple SGT banner",
			agentID:    "team-sntpurple001",
			configName: "SNTPurple",
			role:       "Code Reviewer",
			wantEmoji:  "🟣",
			wantID:     true,
			wantRole:   true,
		},
		{
			name:       "Snake banner",
			agentID:    "team-snake001",
			configName: "Snake",
			role:       "Reconnaissance",
			wantEmoji:  "🐍",
			wantID:     true,
			wantRole:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateBanner(tt.agentID, tt.configName, tt.role)

			// Check that banner contains expected elements
			if !strings.Contains(got, tt.wantEmoji) {
				t.Errorf("GenerateBanner() missing emoji %q", tt.wantEmoji)
			}
			if tt.wantID && !strings.Contains(got, tt.agentID) {
				t.Errorf("GenerateBanner() missing agent ID %q", tt.agentID)
			}
			if tt.wantRole && !strings.Contains(got, tt.role) {
				t.Errorf("GenerateBanner() missing role %q", tt.role)
			}

			// Check for Unicode box-drawing characters
			if !strings.Contains(got, "╔") || !strings.Contains(got, "╚") {
				t.Error("GenerateBanner() missing Unicode box-drawing characters")
			}

			// Check for ANSI codes
			if !strings.Contains(got, "\x1b[") {
				t.Error("GenerateBanner() missing ANSI escape sequences")
			}

			// Check for reset sequence at the end
			if !strings.HasSuffix(got, "\x1b[0m") {
				t.Error("GenerateBanner() should end with reset sequence")
			}
		})
	}
}

func TestGenerateBackgroundTint(t *testing.T) {
	tests := []struct {
		name       string
		configName string
		want       string
	}{
		{
			name:       "Green background",
			configName: "SNTGreen",
			want:       "\x1b[48;2;5;30;15m",
		},
		{
			name:       "Purple background",
			configName: "SNTPurple",
			want:       "\x1b[48;2;20;10;35m",
		},
		{
			name:       "Red background",
			configName: "SNTRed",
			want:       "\x1b[48;2;35;10;10m",
		},
		{
			name:       "Snake background",
			configName: "Snake",
			want:       "\x1b[48;2;5;25;30m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateBackgroundTint(tt.configName)
			if got != tt.want {
				t.Errorf("GenerateBackgroundTint(%q) = %q, want %q", tt.configName, got, tt.want)
			}
		})
	}
}

func TestAgentColorsConsistency(t *testing.T) {
	// Verify that all color fields are populated for all agent types
	testConfigs := []string{"SNTGreen", "SNTPurple", "SNTRed", "Snake", "Captain", "BlueTest", "Unknown"}

	for _, config := range testConfigs {
		t.Run(config, func(t *testing.T) {
			colors := GetAgentColors(config)

			if colors.BgDark == "" {
				t.Errorf("BgDark is empty for %s", config)
			}
			if colors.BgBright == "" {
				t.Errorf("BgBright is empty for %s", config)
			}
			if colors.FgColor == "" {
				t.Errorf("FgColor is empty for %s", config)
			}
			if colors.Emoji == "" {
				t.Errorf("Emoji is empty for %s", config)
			}
			if colors.Reset != "\x1b[0m" {
				t.Errorf("Reset is not standard for %s", config)
			}
		})
	}
}
