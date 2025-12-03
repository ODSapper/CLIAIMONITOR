package agents

import (
	"testing"

	"github.com/CLIAIMONITOR/internal/types"
)

func TestGetPromptFilename_Snake(t *testing.T) {
	role := types.RoleReconSpecialOps
	filename := GetPromptFilename(role)
	expected := "snake.md"

	if filename != expected {
		t.Errorf("GetPromptFilename(%v) = %v, want %v", role, filename, expected)
	}
}

func TestSnakeAgentConfig(t *testing.T) {
	config := types.AgentConfig{
		Name:       "Snake",
		Model:      "claude-opus-4-5-20251101",
		Role:       types.RoleReconSpecialOps,
		Color:      "#2d5016",
		Prefix:     "Snake",
		Numbering:  true,
		PromptFile: "snake.md",
	}

	// Verify config fields
	if config.Prefix != "Snake" {
		t.Errorf("Expected Prefix 'Snake', got '%s'", config.Prefix)
	}

	if !config.Numbering {
		t.Errorf("Expected Numbering to be true")
	}

	if config.PromptFile != "snake.md" {
		t.Errorf("Expected PromptFile 'snake.md', got '%s'", config.PromptFile)
	}

	if config.Role != types.RoleReconSpecialOps {
		t.Errorf("Expected Role 'Reconnaissance & Special Ops', got '%s'", config.Role)
	}
}

func TestSnakeAccessLevel(t *testing.T) {
	role := types.RoleReconSpecialOps
	accessLevel := types.GetAccessLevelForRole(role)
	expected := types.AccessReadOnlyCross

	if accessLevel != expected {
		t.Errorf("GetAccessLevelForRole(%v) = %v, want %v", role, accessLevel, expected)
	}
}
