package agents

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CLIAIMONITOR/internal/instance"
	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/types"
)

// Spawner manages agent process lifecycle
type Spawner interface {
	SpawnAgent(config types.AgentConfig, agentID string, projectPath string, initialPrompt string) (pid int, err error)
	StopAgent(agentID string) error
	StopAgentWithReason(agentID string, reason string) error
	IsAgentRunning(pid int) bool
	GetRunningAgents() map[string]int // agentID -> PID
}

// ProcessSpawner implements Spawner using WezTerm
type ProcessSpawner struct {
	mu             sync.RWMutex
	basePath       string // CLIAIMONITOR directory
	mcpServerURL   string
	natsURL        string // NATS server URL for agent connections
	scriptsPath    string
	configsPath    string
	runningAgents  map[string]int // agentID -> PID
	agentPanes     map[string]int // agentID -> WezTerm pane ID
	agentCounters  map[string]int // agentType -> sequence counter
	memDB          memory.MemoryDB
	heartbeatPIDs  map[string]int // agentID -> heartbeat script PID
}

// NewSpawner creates a new process spawner
func NewSpawner(basePath string, mcpServerURL string, memDB memory.MemoryDB) *ProcessSpawner {
	return &ProcessSpawner{
		basePath:      basePath,
		mcpServerURL:  mcpServerURL,
		scriptsPath:   filepath.Join(basePath, "scripts"),
		configsPath:   filepath.Join(basePath, "configs"),
		runningAgents: make(map[string]int),
		agentPanes:    make(map[string]int),
		agentCounters: make(map[string]int),
		memDB:         memDB,
		heartbeatPIDs: make(map[string]int),
	}
}

// SetNATSURL sets the NATS URL for agent connections
func (s *ProcessSpawner) SetNATSURL(url string) {
	s.natsURL = url
}

// GetNATSURL returns the configured NATS URL
func (s *ProcessSpawner) GetNATSURL() string {
	return s.natsURL
}

// SetMemoryDB sets the memory database for the spawner
func (s *ProcessSpawner) SetMemoryDB(db memory.MemoryDB) {
	s.memDB = db
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

// GetAgentPaneID returns the WezTerm pane ID for an agent
func (s *ProcessSpawner) GetAgentPaneID(agentID string) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	paneID, ok := s.agentPanes[agentID]
	return paneID, ok
}

// SetAgentPaneID stores the WezTerm pane ID for an agent
func (s *ProcessSpawner) SetAgentPaneID(agentID string, paneID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.agentPanes == nil {
		s.agentPanes = make(map[string]int)
	}
	s.agentPanes[agentID] = paneID
}

// MCPConfig structure for agent config file
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// MCPServerConfig defines an MCP server connection
type MCPServerConfig struct {
	Type    string            `json:"type"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	// NATS connection info
	NATSURL string `json:"nats_url,omitempty"`
}

// createMCPConfig creates agent-specific MCP config file with project context
func (s *ProcessSpawner) createMCPConfig(agentID string, projectPath string, accessLevel types.AccessLevel) (string, error) {
	mcpServer := MCPServerConfig{
		Type: "sse",
		URL:  s.mcpServerURL,
		Headers: map[string]string{
			"X-Agent-ID":     agentID,
			"X-Project-Path": projectPath,
			"X-Access-Level": string(accessLevel),
		},
	}

	// Add NATS URL if available
	if s.natsURL != "" {
		mcpServer.NATSURL = s.natsURL
	}

	config := MCPConfig{
		MCPServers: map[string]MCPServerConfig{
			"cliaimonitor": mcpServer,
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

// SpawnAgent launches a team agent in WezTerm
func (s *ProcessSpawner) SpawnAgent(config types.AgentConfig, agentID string, projectPath string, initialPrompt string) (int, error) {
	// Generate NATS client ID using convention: agent-{configName}-{seq}
	// Extract sequence from agentID (e.g., "team-coder001" -> 1)
	var seq int
	if _, err := fmt.Sscanf(agentID, "team-%*[a-z]%d", &seq); err != nil {
		// Fallback: use counter
		seq = s.GetNextSequence(config.Name)
	}
	natsClientID := fmt.Sprintf("agent-%s-%d", strings.ToLower(config.Name), seq)

	// Build inline commands for agent setup and launch
	mcpServerName := fmt.Sprintf("cliaimonitor-%s", agentID)

	// Escape the initial prompt for shell
	escapedPrompt := strings.ReplaceAll(initialPrompt, `"`, `\"`)
	escapedPrompt = strings.ReplaceAll(escapedPrompt, `'`, `''`)

	// Build command chain: set title, configure MCP, run Claude
	// Using cmd.exe for simplicity and avoiding PowerShell overhead
	cmdChain := fmt.Sprintf(
		`title %s && claude mcp remove %s 2>nul & claude mcp add --transport sse %s http://localhost:3000/mcp/sse --header "X-Agent-ID: %s" --header "X-Project-Path: %s" && claude --model %s --dangerously-skip-permissions "%s"`,
		agentID,
		mcpServerName,
		mcpServerName,
		agentID,
		projectPath,
		config.Model,
		escapedPrompt,
	)

	// WezTerm only - no fallbacks to other terminals
	var cmd *exec.Cmd
	var paneID int

	if _, err := exec.LookPath("wezterm.exe"); err == nil {
		// Step 1: Split pane with cmd.exe (spawns in current window, not new window)
		// Using --right to split horizontally; command chaining via /k doesn't work reliably
		cmd = exec.Command("wezterm.exe", "cli", "split-pane", "--right", "--cwd", projectPath, "--", "cmd.exe")

		// Set NATS_CLIENT_ID environment variable for the agent process
		cmd.Env = append(os.Environ(), fmt.Sprintf("NATS_CLIENT_ID=%s", natsClientID))

		output, err := cmd.Output()
		if err != nil {
			// Fallback: Try spawn --new-window if split-pane fails
			log.Printf("[SPAWNER] wezterm cli split-pane failed, trying spawn: %v", err)
			cmd = exec.Command("wezterm.exe", "cli", "spawn", "--cwd", projectPath, "--", "cmd.exe")
			cmd.Env = append(os.Environ(), fmt.Sprintf("NATS_CLIENT_ID=%s", natsClientID))

			output, err = cmd.Output()
			if err != nil {
				return 0, fmt.Errorf("failed to spawn agent pane in WezTerm: %w", err)
			}
		}

		// Parse pane ID from output (works for both split-pane and spawn)
		paneIDStr := strings.TrimSpace(string(output))
		if parsedID, parseErr := strconv.Atoi(paneIDStr); parseErr == nil {
			paneID = parsedID
			log.Printf("[SPAWNER] Agent %s spawned in pane %d", agentID, paneID)

			// Step 2: Send command chain via send-text with --no-paste and \r\n to execute
			// Small delay to let the cmd.exe prompt appear
			time.Sleep(500 * time.Millisecond)

			sendCmd := exec.Command("wezterm.exe", "cli", "send-text", "--pane-id", paneIDStr, "--no-paste")
			sendCmd.Stdin = strings.NewReader(cmdChain + "\r\n")
			if sendErr := sendCmd.Run(); sendErr != nil {
				log.Printf("[SPAWNER] Warning: Failed to send command to pane %d: %v", paneID, sendErr)
			} else {
				log.Printf("[SPAWNER] Command chain sent to pane %d", paneID)
			}
		} else {
			log.Printf("[SPAWNER] Warning: Could not parse pane ID from output: %s", paneIDStr)
			paneID = -1
		}
	} else {
		return 0, fmt.Errorf("WezTerm not found in PATH - required for agent spawning")
	}

	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	log.Printf("[SPAWNER] Agent %s launched in WezTerm (PID: %d, Pane: %d)", agentID, pid, paneID)

	// Don't wait - let it run independently
	if cmd.Process != nil {
		go cmd.Wait()
	}

	// Store the pane ID for later use (cleanup, etc.)
	if paneID > 0 {
		s.SetAgentPaneID(agentID, paneID)
		log.Printf("[SPAWNER] Stored pane ID %d for agent %s", paneID, agentID)
	}

	// Note: We can't reliably track the agent PID since it runs in Windows Terminal.
	// Agents register themselves via MCP when they connect.

	// Register in DB with "pending" status (Phase 1 of two-phase registration)
	// Agent will transition to "connected" when it calls register_agent via MCP
	if s.memDB != nil {
		agentControl := &memory.AgentControl{
			AgentID:     agentID,
			ConfigName:  config.Name,
			Role:        string(config.Role),
			ProjectPath: projectPath,
			PID:         &pid,
			Status:      "pending", // Two-phase: pending -> connected (via MCP)
			Model:       config.Model,
			Color:       config.Color,
		}
		if err := s.memDB.RegisterAgent(agentControl); err != nil {
			log.Printf("Warning: Failed to register agent in DB: %v", err)
		}
		log.Printf("[SPAWNER] Agent %s registered with 'pending' status, awaiting MCP connection", agentID)

		// Only spawn PowerShell heartbeat if NATS is not available
		// NATS handles heartbeats natively via pub/sub
		if s.natsURL == "" {
			if err := s.spawnHeartbeatScript(agentID); err != nil {
				log.Printf("Warning: Failed to spawn heartbeat script: %v", err)
			}
		} else {
			log.Printf("[AGENT] Skipping PowerShell heartbeat - using NATS for agent %s", agentID)
		}
	}

	return pid, nil
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

	// 2. Kill heartbeat script from in-memory tracking
	s.mu.Lock()
	if pid, ok := s.heartbeatPIDs[agentID]; ok {
		if err := instance.KillProcess(pid); err != nil {
			log.Printf("Warning: Failed to kill heartbeat script: %v", err)
		}
		delete(s.heartbeatPIDs, agentID)
	}
	s.mu.Unlock()

	// 3. Also kill heartbeat from PID file (spawned by launcher script)
	if err := s.KillHeartbeatFromPIDFile(agentID); err != nil {
		log.Printf("Warning: Failed to kill heartbeat from PID file: %v", err)
	}

	// 4. Mark stopped in DB
	if s.memDB != nil {
		if err := s.memDB.MarkStopped(agentID, reason); err != nil {
			log.Printf("Warning: Failed to mark agent stopped: %v", err)
		}
	}

	// 5. Remove from running agents map
	s.mu.Lock()
	delete(s.runningAgents, agentID)
	s.mu.Unlock()

	// 6. Try to kill by WezTerm pane ID first (most reliable method)
	if paneID, ok := s.GetAgentPaneID(agentID); ok && paneID > 0 {
		log.Printf("[SPAWNER] Killing agent %s via pane ID %d", agentID, paneID)
		if err := s.KillByPaneID(paneID); err != nil {
			log.Printf("Warning: Failed to kill by pane ID: %v", err)
		} else {
			// Successfully killed by pane ID, remove from tracking
			s.mu.Lock()
			delete(s.agentPanes, agentID)
			s.mu.Unlock()
			log.Printf("[SPAWNER] Successfully killed agent %s via pane ID", agentID)
			// Still continue with other cleanup methods as fallback
		}
	}

	// 7. Try to kill the agent process using PID file (fallback method)
	// PID file contains PowerShell process PID inside the terminal
	pid, err := s.GetAgentPIDFromFile(agentID)
	if err == nil && pid > 0 {
		log.Printf("Killing agent %s (PID: %d) and child processes", agentID, pid)

		// Kill any claude.exe child processes
		if err := s.KillChildClaude(pid); err != nil {
			log.Printf("Warning: Failed to kill claude.exe child: %v", err)
		}

		// Kill the PowerShell process (this closes the terminal tab)
		if err := instance.KillProcess(pid); err != nil {
			log.Printf("Warning: Failed to kill PowerShell by PID: %v", err)
		}

		// Clean up PID file
		s.CleanupAgentPIDFile(agentID)
	}

	// 8. Also try killing by window title (catches any stragglers)
	if err := s.KillByWindowTitle(agentID); err != nil {
		log.Printf("Warning: Failed to kill agent by window title: %v", err)
	}

	// 9. Kill any remaining powershell processes with our temp script
	if err := s.KillByTempScript(agentID); err != nil {
		log.Printf("Warning: Failed to kill by temp script: %v", err)
	}

	return nil
}

// KillByPaneID kills a WezTerm pane by its pane ID
func (s *ProcessSpawner) KillByPaneID(paneID int) error {
	cmd := exec.Command("wezterm.exe", "cli", "kill-pane", "--pane-id", fmt.Sprintf("%d", paneID))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to kill pane %d: %w (output: %s)", paneID, err, string(output))
	}
	log.Printf("[SPAWNER] Successfully killed pane %d", paneID)
	return nil
}

// KillChildClaude kills any claude.exe processes that are children of the given parent PID
func (s *ProcessSpawner) KillChildClaude(parentPID int) error {
	// Use PowerShell to find and kill claude.exe child processes
	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf(`Get-CimInstance Win32_Process -Filter "ParentProcessId=%d AND Name='claude.exe'" | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }`, parentPID))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to kill child claude: %w (output: %s)", err, string(output))
	}
	return nil
}

// GetAgentPIDFromFile reads the actual agent PID from the tracking file
func (s *ProcessSpawner) GetAgentPIDFromFile(agentID string) (int, error) {
	pidFile := filepath.Join(s.basePath, "data", "pids", agentID+".pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}
	return pid, nil
}

// CleanupAgentPIDFile removes the PID tracking file for an agent
func (s *ProcessSpawner) CleanupAgentPIDFile(agentID string) error {
	pidFile := filepath.Join(s.basePath, "data", "pids", agentID+".pid")
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}
	return nil
}

// KillHeartbeatFromPIDFile kills the heartbeat process using its PID file
func (s *ProcessSpawner) KillHeartbeatFromPIDFile(agentID string) error {
	pidFile := filepath.Join(s.basePath, "data", "pids", agentID+"-heartbeat.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No heartbeat PID file, that's fine
		}
		return fmt.Errorf("failed to read heartbeat PID file: %w", err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return fmt.Errorf("invalid heartbeat PID in file: %w", err)
	}

	log.Printf("Killing heartbeat process for %s (PID: %d)", agentID, pid)
	if err := instance.KillProcess(pid); err != nil {
		log.Printf("Warning: Failed to kill heartbeat PID %d: %v", pid, err)
	}

	// Clean up heartbeat PID file
	os.Remove(pidFile)
	return nil
}

// KillByWindowTitle finds and kills a process by its window title (fallback method)
func (s *ProcessSpawner) KillByWindowTitle(agentID string) error {
	title := fmt.Sprintf("CLIAIMONITOR-%s", agentID)
	// Use PowerShell to find process by window title and kill it
	cmd := exec.Command("powershell.exe", "-Command",
		fmt.Sprintf(`Get-Process | Where-Object {$_.MainWindowTitle -eq '%s'} | Stop-Process -Force -ErrorAction SilentlyContinue`, title))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to kill by window title: %w (output: %s)", err, string(output))
	}
	return nil
}

// KillByTempScript kills any PowerShell processes running our agent's temp launcher script
func (s *ProcessSpawner) KillByTempScript(agentID string) error {
	tempScriptName := fmt.Sprintf("cliaimonitor-%s-launcher.ps1", agentID)
	// Find and kill any powershell.exe with our temp script in command line
	cmd := exec.Command("powershell.exe", "-Command",
		fmt.Sprintf(`Get-CimInstance Win32_Process -Filter "Name='powershell.exe'" | Where-Object { $_.CommandLine -like '*%s*' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }`, tempScriptName))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to kill by temp script: %w (output: %s)", err, string(output))
	}
	return nil
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
	delete(s.agentPanes, agentID)
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

// CleanupAgentFiles removes MCP config, prompt files, and PID file for an agent
func (s *ProcessSpawner) CleanupAgentFiles(agentID string) error {
	// Remove MCP config
	mcpConfigPath := filepath.Join(s.configsPath, "mcp", fmt.Sprintf("%s-mcp.json", agentID))
	if err := os.Remove(mcpConfigPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove MCP config: %w", err)
	}

	// Remove PID tracking file
	s.CleanupAgentPIDFile(agentID)

	return nil
}

// CleanupAllAgentFiles removes all generated config and PID files
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

	// Clean PID tracking files
	pidsDir := filepath.Join(s.basePath, "data", "pids")
	entries, err = os.ReadDir(pidsDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".pid") {
				os.Remove(filepath.Join(pidsDir, entry.Name()))
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
