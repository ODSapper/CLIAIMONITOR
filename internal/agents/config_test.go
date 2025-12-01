package agents

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CLIAIMONITOR/internal/types"
)

func TestLoadTeamsConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "teams.yaml")

	configYAML := `agents:
  - name: SNTGreen
    model: claude-sonnet-4-5-20250929
    role: Go Developer
    color: "#00cc66"
  - name: OpusPurple
    model: claude-opus-4-5-20251101
    role: Code Auditor
    color: "#bb66ff"

supervisor:
  name: Supervisor
  model: claude-opus-4-5-20251101
  role: Supervisor
  color: "#ffd700"
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadTeamsConfig(configPath)
	if err != nil {
		t.Fatalf("LoadTeamsConfig() error = %v", err)
	}

	if len(config.Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(config.Agents))
	}

	if config.Agents[0].Name != "SNTGreen" {
		t.Errorf("expected first agent name 'SNTGreen', got '%s'", config.Agents[0].Name)
	}

	if config.Agents[0].Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("expected model 'claude-sonnet-4-5-20250929', got '%s'", config.Agents[0].Model)
	}

	if config.Agents[0].Color != "#00cc66" {
		t.Errorf("expected color '#00cc66', got '%s'", config.Agents[0].Color)
	}

	if config.Supervisor.Name != "Supervisor" {
		t.Errorf("expected supervisor name 'Supervisor', got '%s'", config.Supervisor.Name)
	}

	if config.Supervisor.Model != "claude-opus-4-5-20251101" {
		t.Errorf("expected supervisor model 'claude-opus-4-5-20251101', got '%s'", config.Supervisor.Model)
	}
}

func TestLoadTeamsConfigNonExistent(t *testing.T) {
	_, err := LoadTeamsConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestLoadTeamsConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	// Write invalid YAML
	if err := os.WriteFile(configPath, []byte("{{invalid yaml"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := LoadTeamsConfig(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestGetAgentConfig(t *testing.T) {
	config := &types.TeamsConfig{
		Agents: []types.AgentConfig{
			{Name: "SNTGreen", Model: "sonnet", Role: types.RoleGoDeveloper, Color: "#00cc66"},
			{Name: "OpusPurple", Model: "opus", Role: types.RoleCodeAuditor, Color: "#bb66ff"},
			{Name: "SNTRed", Model: "sonnet", Role: types.RoleEngineer, Color: "#cc3333"},
		},
	}

	tests := []struct {
		name     string
		expected *types.AgentConfig
	}{
		{"SNTGreen", &config.Agents[0]},
		{"OpusPurple", &config.Agents[1]},
		{"SNTRed", &config.Agents[2]},
		{"NonExistent", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAgentConfig(config, tt.name)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil for '%s', got %v", tt.name, result)
				}
			} else {
				if result == nil {
					t.Errorf("expected non-nil for '%s'", tt.name)
				} else if result.Name != tt.expected.Name {
					t.Errorf("expected name '%s', got '%s'", tt.expected.Name, result.Name)
				}
			}
		})
	}
}

func TestGetAgentConfigEmptyConfig(t *testing.T) {
	config := &types.TeamsConfig{
		Agents: []types.AgentConfig{},
	}

	result := GetAgentConfig(config, "AnyName")
	if result != nil {
		t.Error("expected nil for empty agents list")
	}
}

func TestGetPromptFilename(t *testing.T) {
	tests := []struct {
		role     types.AgentRole
		expected string
	}{
		{types.RoleSupervisor, "supervisor.md"},
		{types.RoleGoDeveloper, "go-developer.md"},
		{types.RoleCodeAuditor, "code-auditor.md"},
		{types.RoleEngineer, "engineer.md"},
		{types.RoleSecurity, "security.md"},
		{types.AgentRole("Unknown"), "engineer.md"},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			result := GetPromptFilename(tt.role)
			if result != tt.expected {
				t.Errorf("GetPromptFilename(%s) = %s, want %s", tt.role, result, tt.expected)
			}
		})
	}
}

func TestLoadTeamsConfigEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty.yaml")

	// Write empty file
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config, err := LoadTeamsConfig(configPath)
	if err != nil {
		t.Fatalf("LoadTeamsConfig() should not error on empty file: %v", err)
	}

	// Empty YAML should result in zero agents
	if len(config.Agents) != 0 {
		t.Errorf("expected 0 agents for empty config, got %d", len(config.Agents))
	}
}

func TestLoadTeamsConfigPartialConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "partial.yaml")

	// Config with only agents, no supervisor
	configYAML := `agents:
  - name: TestAgent
    model: test-model
    role: Engineer
    color: "#ffffff"
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadTeamsConfig(configPath)
	if err != nil {
		t.Fatalf("LoadTeamsConfig() error = %v", err)
	}

	if len(config.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(config.Agents))
	}

	// Supervisor should be zero value
	if config.Supervisor.Name != "" {
		t.Errorf("expected empty supervisor name, got '%s'", config.Supervisor.Name)
	}
}
