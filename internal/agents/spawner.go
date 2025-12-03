package agents

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/CLIAIMONITOR/internal/instance"
	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/types"
)

// Spawner manages agent process lifecycle
type Spawner interface {
	SpawnAgent(config types.AgentConfig, agentID string, projectPath string, initialPrompt string) (pid int, err error)
	SpawnSupervisor(config types.AgentConfig) (pid int, err error)
	StopAgent(agentID string) error
	StopAgentWithReason(agentID string, reason string) error
	IsAgentRunning(pid int) bool
	GetRunningAgents() map[string]int // agentID -> PID
}

// ProcessSpawner implements Spawner using PowerShell
type ProcessSpawner struct {
	mu             sync.RWMutex
	basePath       string // CLIAIMONITOR directory
	mcpServerURL   string
	promptsPath    string
	scriptsPath    string
	configsPath    string
	runningAgents  map[string]int // agentID -> PID
	agentCounters  map[string]int // agentType -> sequence counter
	memDB          memory.MemoryDB
	heartbeatPIDs  map[string]int // agentID -> heartbeat script PID
}

// NewSpawner creates a new process spawner
func NewSpawner(basePath string, mcpServerURL string, memDB memory.MemoryDB) *ProcessSpawner {
	return &ProcessSpawner{
		basePath:      basePath,
		mcpServerURL:  mcpServerURL,
		promptsPath:   filepath.Join(basePath, "configs", "prompts"),
		scriptsPath:   filepath.Join(basePath, "scripts"),
		configsPath:   filepath.Join(basePath, "configs"),
		runningAgents: make(map[string]int),
		agentCounters: make(map[string]int),
		memDB:         memDB,
		heartbeatPIDs: make(map[string]int),
	}
}

// GenerateAgentID creates a team-compatible agent ID in format: team-{type}{seq}
// Example: team-opusgreen001, team-sntpurple002, team-snake003
func (s *ProcessSpawner) GenerateAgentID(agentType string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Increment counter for this agent type
	s.agentCounters[agentType]++
	seq := s.agentCounters[agentType]

	// Normalize agent type to lowercase for team ID
	normalizedType := strings.ToLower(agentType)

	// Format: team-{type}{seq:03d}
	return fmt.Sprintf("team-%s%03d", normalizedType, seq)
}

// GetNextSequence returns the next sequence number for an agent type (for preview)
func (s *ProcessSpawner) GetNextSequence(agentType string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agentCounters[agentType] + 1
}

// MCPConfig structure for agent config file
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// MCPServerConfig defines an MCP server connection
type MCPServerConfig struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// createMCPConfig creates agent-specific MCP config file with project context
func (s *ProcessSpawner) createMCPConfig(agentID string, projectPath string, accessLevel types.AccessLevel) (string, error) {
	config := MCPConfig{
		MCPServers: map[string]MCPServerConfig{
			"cliaimonitor": {
				Type: "sse",
				URL:  s.mcpServerURL,
				Headers: map[string]string{
					"X-Agent-ID":     agentID,
					"X-Project-Path": projectPath,
					"X-Access-Level": string(accessLevel),
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

// createSystemPrompt creates agent-specific system prompt with project context
func (s *ProcessSpawner) createSystemPrompt(agentID string, config types.AgentConfig, projectPath string, projectName string) (string, error) {
	// Use override prompt file if specified, otherwise derive from role
	promptFile := config.PromptFile
	if promptFile == "" {
		promptFile = GetPromptFilename(config.Role)
	}
	basePath := filepath.Join(s.promptsPath, promptFile)

	data, err := os.ReadFile(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt %s: %w", promptFile, err)
	}

	// Replace placeholder with actual agent ID
	prompt := strings.ReplaceAll(string(data), "{{AGENT_ID}}", agentID)

	// Add project context section
	projectContext := s.buildProjectContext(projectPath, projectName, config.Role, agentID)
	prompt = strings.ReplaceAll(prompt, "{{PROJECT_CONTEXT}}", projectContext)
	prompt = strings.ReplaceAll(prompt, "{{PROJECT_NAME}}", projectName)
	prompt = strings.ReplaceAll(prompt, "{{PROJECT_PATH}}", projectPath)
	prompt = strings.ReplaceAll(prompt, "{{ACCESS_RULES}}", s.getAccessRules(config.Role, projectPath))

	// If placeholders weren't in template, append project context
	if !strings.Contains(string(data), "{{PROJECT_CONTEXT}}") && projectContext != "" {
		prompt += "\n\n" + projectContext
	}

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

// buildProjectContext builds the project context section for prompts
func (s *ProcessSpawner) buildProjectContext(projectPath string, projectName string, role types.AgentRole, agentID string) string {
	var sb strings.Builder

	sb.WriteString("# Project Context\n\n")
	sb.WriteString(fmt.Sprintf("You are working on: **%s**\n", projectName))
	sb.WriteString(fmt.Sprintf("Project path: `%s`\n\n", projectPath))

	// Try to read CLAUDE.md from the project
	claudeMD, err := ReadClaudeMD(projectPath)
	if err == nil && claudeMD != "" {
		sb.WriteString("## Project Instructions (from CLAUDE.md)\n\n")
		sb.WriteString(claudeMD)
		sb.WriteString("\n\n")
	}

	// Add team context override to suppress team ID questions
	sb.WriteString("## Team Context Override\n\n")
	sb.WriteString(fmt.Sprintf("Your team ID is '%s'. Use this for all Planner API interactions.\n", agentID))
	sb.WriteString("Do NOT ask about team assignments or workflow procedures from project CLAUDE.md.\n")
	sb.WriteString("Work autonomously on your assigned tasks. Use MCP tools to communicate status.\n")
	sb.WriteString(fmt.Sprintf("For Planner API calls, use header: X-API-Key: %s\n\n", agentID))

	// Add access rules
	sb.WriteString("## Access Rules\n\n")
	sb.WriteString(s.getAccessRules(role, projectPath))

	return sb.String()
}

// getAccessRules returns the access rules text for a role
func (s *ProcessSpawner) getAccessRules(role types.AgentRole, projectPath string) string {
	switch role {
	case types.RoleCodeAuditor, types.RoleSecurity:
		return fmt.Sprintf("You may read files from any project in the Magnolia ecosystem for review purposes. You may only WRITE files within `%s`.", projectPath)
	case types.RoleSupervisor:
		return "You may read files from all projects. You should not write code files directly."
	default:
		// Go Developer, Engineer - strict isolation
		return fmt.Sprintf("You may ONLY read and write files within `%s`. Do not access other repositories.", projectPath)
	}
}

// SpawnAgent launches a team agent in Windows Terminal
func (s *ProcessSpawner) SpawnAgent(config types.AgentConfig, agentID string, projectPath string, initialPrompt string) (int, error) {
	// Derive project name from path
	projectName := filepath.Base(projectPath)

	// Get access level for this role
	accessLevel := types.GetAccessLevelForRole(config.Role)

	// Create MCP config for this agent with project context
	mcpConfigPath, err := s.createMCPConfig(agentID, projectPath, accessLevel)
	if err != nil {
		return 0, fmt.Errorf("failed to create MCP config: %w", err)
	}

	// Create system prompt for this agent with project context
	promptPath, err := s.createSystemPrompt(agentID, config, projectPath, projectName)
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
		"-InitialPrompt", initialPrompt,
	}

	// Add skip permissions flag if enabled
	if config.SkipPermissions {
		args = append(args, "-SkipPermissions")
	}

	cmd := exec.Command("powershell.exe", args...)

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start agent: %w", err)
	}

	pid := cmd.Process.Pid

	// Don't wait - let it run independently
	// The launcher script spawns a detached Windows Terminal process and exits,
	// so we don't track the launcher PID. Agent registration happens via MCP.
	go cmd.Wait()

	// Note: We can't reliably track the agent PID since it runs in Windows Terminal.
	// Agents register themselves via MCP when they connect.

	// Register in DB
	if s.memDB != nil {
		agentControl := &memory.AgentControl{
			AgentID:     agentID,
			ConfigName:  config.Name,
			Role:        string(config.Role),
			ProjectPath: projectPath,
			PID:         &pid,
			Status:      "starting",
			Model:       config.Model,
			Color:       config.Color,
		}
		if err := s.memDB.RegisterAgent(agentControl); err != nil {
			log.Printf("Warning: Failed to register agent in DB: %v", err)
		}

		// Spawn heartbeat script
		if err := s.spawnHeartbeatScript(agentID); err != nil {
			log.Printf("Warning: Failed to spawn heartbeat script: %v", err)
		}
	}

	return pid, nil
}

// SpawnSupervisor launches the supervisor agent
func (s *ProcessSpawner) SpawnSupervisor(config types.AgentConfig) (int, error) {
	initialPrompt := "MANDATORY FIRST ACTION: Say exactly: 'CONTEXT LOADED: I am Supervisor (Supervisor). Ready for mission.' " +
		"If you cannot see your agent ID in your system prompt, say 'NO CONTEXT: System prompt not loaded' instead. " +
		"THEN call mcp__cliaimonitor__register_agent with agent_id='Supervisor' and role='Supervisor'. Begin your monitoring cycle."
	return s.SpawnAgent(config, "Supervisor", s.basePath, initialPrompt)
}

// spawnHeartbeatScript spawns the heartbeat monitor script for an agent
func (s *ProcessSpawner) spawnHeartbeatScript(agentID string) error {
	scriptPath := filepath.Join(s.basePath, "scripts", "agent-heartbeat.ps1")
	dbPath := filepath.Join(s.basePath, "data", "memory.db")

	cmd := exec.Command("powershell.exe",
		"-ExecutionPolicy", "Bypass",
		"-WindowStyle", "Hidden",
		"-File", scriptPath,
		"-AgentID", agentID,
		"-DBPath", dbPath,
		"-IntervalSeconds", "30")

	cmd.Dir = s.basePath

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start heartbeat script: %w", err)
	}

	// Track the heartbeat PID for cleanup
	s.trackHeartbeatPID(agentID, cmd.Process.Pid)

	// Let it run independently
	go cmd.Wait()

	return nil
}

// trackHeartbeatPID tracks the heartbeat script PID for an agent
func (s *ProcessSpawner) trackHeartbeatPID(agentID string, pid int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.heartbeatPIDs == nil {
		s.heartbeatPIDs = make(map[string]int)
	}
	s.heartbeatPIDs[agentID] = pid
}

// StopAgent terminates an agent process (backward compatible - no reason)
func (s *ProcessSpawner) StopAgent(agentID string) error {
	return s.StopAgentWithReason(agentID, "manual stop")
}

// StopAgentWithReason terminates an agent process with a specific reason
func (s *ProcessSpawner) StopAgentWithReason(agentID string, reason string) error {
	// 1. Set shutdown flag in DB (heartbeat script will see this)
	if s.memDB != nil {
		if err := s.memDB.SetShutdownFlag(agentID, reason); err != nil {
			log.Printf("Warning: Failed to set shutdown flag: %v", err)
		}
	}

	// 2. Kill heartbeat script
	s.mu.Lock()
	if pid, ok := s.heartbeatPIDs[agentID]; ok {
		if err := instance.KillProcess(pid); err != nil {
			log.Printf("Warning: Failed to kill heartbeat script: %v", err)
		}
		delete(s.heartbeatPIDs, agentID)
	}
	s.mu.Unlock()

	// 3. Mark stopped in DB
	if s.memDB != nil {
		if err := s.memDB.MarkStopped(agentID, reason); err != nil {
			log.Printf("Warning: Failed to mark agent stopped: %v", err)
		}
	}

	// 4. Remove from running agents map
	s.mu.Lock()
	pid, exists := s.runningAgents[agentID]
	if exists {
		delete(s.runningAgents, agentID)
	}
	s.mu.Unlock()

	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	// 5. Try to kill the agent process
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
