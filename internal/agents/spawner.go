package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/CLIAIMONITOR/internal/types"
)

// Spawner manages agent process lifecycle
type Spawner interface {
	SpawnAgent(config types.AgentConfig, agentID string, projectPath string) (pid int, err error)
	SpawnSupervisor(config types.AgentConfig) (pid int, err error)
	StopAgent(agentID string) error
	IsAgentRunning(pid int) bool
	GetRunningAgents() map[string]int // agentID -> PID
}

// ProcessSpawner implements Spawner using PowerShell
type ProcessSpawner struct {
	mu            sync.RWMutex
	basePath      string // CLIAIMONITOR directory
	mcpServerURL  string
	promptsPath   string
	scriptsPath   string
	configsPath   string
	runningAgents map[string]int // agentID -> PID
}

// NewSpawner creates a new process spawner
func NewSpawner(basePath string, mcpServerURL string) *ProcessSpawner {
	return &ProcessSpawner{
		basePath:      basePath,
		mcpServerURL:  mcpServerURL,
		promptsPath:   filepath.Join(basePath, "configs", "prompts"),
		scriptsPath:   filepath.Join(basePath, "scripts"),
		configsPath:   filepath.Join(basePath, "configs"),
		runningAgents: make(map[string]int),
	}
}

// MCPConfig structure for agent config file
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// MCPServerConfig defines an MCP server connection
type MCPServerConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// createMCPConfig creates agent-specific MCP config file
func (s *ProcessSpawner) createMCPConfig(agentID string) (string, error) {
	config := MCPConfig{
		MCPServers: map[string]MCPServerConfig{
			"cliaimonitor": {
				URL: s.mcpServerURL,
				Headers: map[string]string{
					"X-Agent-ID": agentID,
				},
			},
		},
	}

	mcpDir := filepath.Join(s.configsPath, "mcp")
	if err := os.MkdirAll(mcpDir, 0755); err != nil {
		return "", err
	}

	configPath := filepath.Join(mcpDir, fmt.Sprintf("%s-mcp.json", agentID))

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", err
	}

	return configPath, nil
}

// createSystemPrompt creates agent-specific system prompt
func (s *ProcessSpawner) createSystemPrompt(agentID string, role types.AgentRole) (string, error) {
	// Read base prompt for role
	promptFile := GetPromptFilename(role)
	basePath := filepath.Join(s.promptsPath, promptFile)

	data, err := os.ReadFile(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt %s: %w", promptFile, err)
	}

	// Replace placeholder with actual agent ID
	prompt := strings.ReplaceAll(string(data), "{{AGENT_ID}}", agentID)

	// Write agent-specific prompt
	activeDir := filepath.Join(s.promptsPath, "active")
	if err := os.MkdirAll(activeDir, 0755); err != nil {
		return "", err
	}

	outPath := filepath.Join(activeDir, fmt.Sprintf("%s-prompt.md", agentID))

	if err := os.WriteFile(outPath, []byte(prompt), 0644); err != nil {
		return "", err
	}

	return outPath, nil
}

// SpawnAgent launches a team agent in Windows Terminal
func (s *ProcessSpawner) SpawnAgent(config types.AgentConfig, agentID string, projectPath string) (int, error) {
	// Create MCP config for this agent
	mcpConfigPath, err := s.createMCPConfig(agentID)
	if err != nil {
		return 0, fmt.Errorf("failed to create MCP config: %w", err)
	}

	// Create system prompt for this agent
	promptPath, err := s.createSystemPrompt(agentID, config.Role)
	if err != nil {
		return 0, fmt.Errorf("failed to create system prompt: %w", err)
	}

	// Build PowerShell command
	scriptPath := filepath.Join(s.scriptsPath, "agent-launcher.ps1")

	args := []string{
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
		"-AgentID", agentID,
		"-AgentName", config.Name,
		"-Model", config.Model,
		"-Role", string(config.Role),
		"-Color", config.Color,
		"-ProjectPath", projectPath,
		"-MCPConfigPath", mcpConfigPath,
		"-SystemPromptPath", promptPath,
	}

	cmd := exec.Command("powershell.exe", args...)

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start agent: %w", err)
	}

	pid := cmd.Process.Pid

	// Don't wait - let it run independently
	go cmd.Wait()

	// Brief delay to allow process to initialize or fail immediately
	time.Sleep(500 * time.Millisecond)

	// Verify the process is still running
	if !s.IsAgentRunning(pid) {
		return 0, fmt.Errorf("agent process exited immediately after spawn")
	}

	s.mu.Lock()
	s.runningAgents[agentID] = pid
	s.mu.Unlock()

	return pid, nil
}

// SpawnSupervisor launches the supervisor agent
func (s *ProcessSpawner) SpawnSupervisor(config types.AgentConfig) (int, error) {
	return s.SpawnAgent(config, "Supervisor", s.basePath)
}

// StopAgent terminates an agent process
func (s *ProcessSpawner) StopAgent(agentID string) error {
	s.mu.Lock()
	pid, exists := s.runningAgents[agentID]
	if exists {
		delete(s.runningAgents, agentID)
	}
	s.mu.Unlock()

	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	// Send terminate signal
	return proc.Kill()
}

// IsAgentRunning checks if a process is still running
func (s *ProcessSpawner) IsAgentRunning(pid int) bool {
	// On Windows, use tasklist to check if process exists
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	// If process exists, output contains the PID
	return strings.Contains(string(output), fmt.Sprintf("%d", pid))
}

// GetRunningAgents returns map of running agents
func (s *ProcessSpawner) GetRunningAgents() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]int)
	for k, v := range s.runningAgents {
		result[k] = v
	}
	return result
}

// RemoveAgent removes an agent from tracking (called when agent disconnects)
func (s *ProcessSpawner) RemoveAgent(agentID string) {
	s.mu.Lock()
	delete(s.runningAgents, agentID)
	s.mu.Unlock()
}

// GetAgentByPID returns the agent ID for a given PID
func (s *ProcessSpawner) GetAgentByPID(pid int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for agentID, agentPID := range s.runningAgents {
		if agentPID == pid {
			return agentID
		}
	}
	return ""
}

// CleanupAgentFiles removes MCP config and prompt files for an agent
func (s *ProcessSpawner) CleanupAgentFiles(agentID string) error {
	// Remove MCP config
	mcpConfigPath := filepath.Join(s.configsPath, "mcp", fmt.Sprintf("%s-mcp.json", agentID))
	if err := os.Remove(mcpConfigPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove MCP config: %w", err)
	}

	// Remove active prompt
	promptPath := filepath.Join(s.promptsPath, "active", fmt.Sprintf("%s-prompt.md", agentID))
	if err := os.Remove(promptPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove prompt: %w", err)
	}

	return nil
}

// CleanupAllAgentFiles removes all generated config and prompt files
func (s *ProcessSpawner) CleanupAllAgentFiles() error {
	// Clean MCP configs
	mcpDir := filepath.Join(s.configsPath, "mcp")
	entries, err := os.ReadDir(mcpDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), "-mcp.json") {
				os.Remove(filepath.Join(mcpDir, entry.Name()))
			}
		}
	}

	// Clean active prompts
	activeDir := filepath.Join(s.promptsPath, "active")
	entries, err = os.ReadDir(activeDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), "-prompt.md") {
				os.Remove(filepath.Join(activeDir, entry.Name()))
			}
		}
	}

	return nil
}

// StopAllAgents stops all running agents
func (s *ProcessSpawner) StopAllAgents() []error {
	s.mu.Lock()
	agents := make(map[string]int)
	for k, v := range s.runningAgents {
		agents[k] = v
	}
	s.mu.Unlock()

	var errors []error
	for agentID := range agents {
		if err := s.StopAgent(agentID); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop %s: %w", agentID, err))
		}
	}
	return errors
}
