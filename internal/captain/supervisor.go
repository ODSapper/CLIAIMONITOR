package captain

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CaptainStatus represents the current state of the Captain process
type CaptainStatus string

const (
	StatusStarting   CaptainStatus = "starting"
	StatusRunning    CaptainStatus = "running"
	StatusCrashed    CaptainStatus = "crashed"
	StatusRestarting CaptainStatus = "restarting"
	StatusStopped    CaptainStatus = "stopped"
	StatusDisabled   CaptainStatus = "disabled" // Crash loop protection triggered
)

// CaptainSupervisor manages the Captain process lifecycle
type CaptainSupervisor struct {
	mu sync.RWMutex

	basePath   string
	serverPort int

	// Process tracking
	captainPID    int
	captainPaneID int // WezTerm pane ID for Captain
	captainCmd    *exec.Cmd

	// Crash loop protection
	respawnCount   int
	respawnWindow  time.Time
	maxRespawns    int
	windowDuration time.Duration

	// State
	status        CaptainStatus
	lastExitCode  int
	lastExitTime  time.Time
	startTime     time.Time
	shutdownChan  chan struct{}
	shutdownOnce  sync.Once

	// Callbacks
	onShutdownRequest func() // Called when Captain exits cleanly (code 0)
}

// SupervisorConfig holds configuration for the CaptainSupervisor
type SupervisorConfig struct {
	BasePath       string
	ServerPort     int
	MaxRespawns    int           // Default: 3
	WindowDuration time.Duration // Default: 1 minute
}

// CaptainInfo provides status information for API responses
type CaptainInfo struct {
	Status       CaptainStatus `json:"status"`
	PID          int           `json:"pid,omitempty"`
	StartTime    *time.Time    `json:"start_time,omitempty"`
	LastExitCode int           `json:"last_exit_code,omitempty"`
	LastExitTime *time.Time    `json:"last_exit_time,omitempty"`
	RespawnCount int           `json:"respawn_count"`
	MaxRespawns  int           `json:"max_respawns"`
	CanRestart   bool          `json:"can_restart"`
}

// NewCaptainSupervisor creates a new supervisor instance
func NewCaptainSupervisor(config SupervisorConfig) *CaptainSupervisor {
	if config.MaxRespawns == 0 {
		config.MaxRespawns = 3
	}
	if config.WindowDuration == 0 {
		config.WindowDuration = 1 * time.Minute
	}

	return &CaptainSupervisor{
		basePath:       config.BasePath,
		serverPort:     config.ServerPort,
		maxRespawns:    config.MaxRespawns,
		windowDuration: config.WindowDuration,
		status:         StatusStopped,
		shutdownChan:   make(chan struct{}),
	}
}

// SetShutdownCallback sets the function called when Captain requests shutdown
func (s *CaptainSupervisor) SetShutdownCallback(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onShutdownRequest = fn
}

// Start launches the Captain process and begins monitoring
func (s *CaptainSupervisor) Start() error {
	s.mu.Lock()
	if s.status == StatusRunning || s.status == StatusStarting {
		s.mu.Unlock()
		return fmt.Errorf("captain already running")
	}
	s.status = StatusStarting
	s.mu.Unlock()

	return s.spawnCaptain()
}

// Stop terminates the Captain process gracefully
func (s *CaptainSupervisor) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.captainCmd != nil && s.captainCmd.Process != nil {
		// Send interrupt signal
		if err := s.captainCmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill captain process: %w", err)
		}
	}

	s.status = StatusStopped
	return nil
}

// Restart manually restarts the Captain (resets crash loop counter)
func (s *CaptainSupervisor) Restart() error {
	s.mu.Lock()
	// Reset crash loop protection on manual restart
	s.respawnCount = 0
	s.respawnWindow = time.Time{}
	s.mu.Unlock()

	// Stop if running
	s.Stop()

	// Small delay to ensure cleanup
	time.Sleep(500 * time.Millisecond)

	return s.Start()
}

// GetInfo returns current Captain status information
func (s *CaptainSupervisor) GetInfo() CaptainInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info := CaptainInfo{
		Status:       s.status,
		PID:          s.captainPID,
		LastExitCode: s.lastExitCode,
		RespawnCount: s.respawnCount,
		MaxRespawns:  s.maxRespawns,
		CanRestart:   s.status == StatusDisabled || s.status == StatusCrashed || s.status == StatusStopped,
	}

	if !s.startTime.IsZero() {
		info.StartTime = &s.startTime
	}
	if !s.lastExitTime.IsZero() {
		info.LastExitTime = &s.lastExitTime
	}

	return info
}

// ShutdownChan returns a channel that closes when Captain requests shutdown
func (s *CaptainSupervisor) ShutdownChan() <-chan struct{} {
	return s.shutdownChan
}

// GetCaptainPaneID returns Captain's WezTerm pane ID (0 if unknown)
func (s *CaptainSupervisor) GetCaptainPaneID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.captainPaneID
}

// MCPConfig structure for Captain's MCP config file
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// MCPServerConfig defines an MCP server connection
type MCPServerConfig struct {
	Type    string            `json:"type"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// createCaptainMCPConfig creates the MCP config file for Captain
func (s *CaptainSupervisor) createCaptainMCPConfig() (string, error) {
	mcpServerURL := fmt.Sprintf("http://localhost:%d/mcp/sse", s.serverPort)

	config := MCPConfig{
		MCPServers: map[string]MCPServerConfig{
			"cliaimonitor": {
				Type: "sse",
				URL:  mcpServerURL,
				Headers: map[string]string{
					"X-Agent-ID":     "Captain",
					"X-Access-Level": "admin",
				},
			},
		},
	}

	// Ensure configs/mcp directory exists
	mcpDir := filepath.Join(s.basePath, "configs", "mcp")
	if err := os.MkdirAll(mcpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create mcp config dir: %w", err)
	}

	configPath := filepath.Join(mcpDir, "captain-mcp.json")

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal MCP config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write MCP config: %w", err)
	}

	return configPath, nil
}

// PaneIDCallback is called when Captain's pane ID is known
type PaneIDCallback func(paneID int)

// onPaneIDReady is called when we know Captain's pane ID
var onPaneIDReady PaneIDCallback

// SetPaneIDCallback sets the callback for when Captain's pane ID is available
func (s *CaptainSupervisor) SetPaneIDCallback(cb PaneIDCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	onPaneIDReady = cb
}

// spawnCaptain launches the Captain in WezTerm
// If already in WezTerm, splits a pane. Otherwise creates new window.
func (s *CaptainSupervisor) spawnCaptain() error {
	// Build Captain system prompt
	captainPrompt := s.buildCaptainPrompt()

	// Write prompt to a file for the launcher to read
	promptFile := filepath.Join(s.basePath, "data", "captain-prompt.md")
	if err := os.WriteFile(promptFile, []byte(captainPrompt), 0644); err != nil {
		return fmt.Errorf("failed to write captain prompt: %w", err)
	}

	// Create .claude/settings.local.json in basePath with appendSystemPrompt
	claudeDir := filepath.Join(s.basePath, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude dir: %w", err)
	}

	settingsFile := filepath.Join(claudeDir, "settings.local.json")
	settings := map[string]string{
		"appendSystemPrompt": captainPrompt,
	}
	settingsJSON, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsFile, settingsJSON, 0644); err != nil {
		return fmt.Errorf("failed to write captain settings: %w", err)
	}

	// Initial prompt - register via MCP and check infrastructure
	initialPrompt := fmt.Sprintf("You are Captain (Orchestrator). First, call mcp__cliaimonitor__register_agent with agent_id='Captain' and role='Orchestrator' to register with the dashboard. Then call mcp__cliaimonitor__get_all_context to restore your session state. Check your monitoring infrastructure: curl http://localhost:%d/api/state", s.serverPort)

	// Create MCP config file for Captain (avoids polluting global MCP registry)
	mcpConfigPath, err := s.createCaptainMCPConfig()
	if err != nil {
		return fmt.Errorf("failed to create MCP config: %w", err)
	}

	// Build the command to run Claude with MCP config file
	claudeCmd := fmt.Sprintf(
		`title Captain && claude --mcp-config "%s" --model claude-opus-4-5-20251101 --dangerously-skip-permissions "%s"`,
		mcpConfigPath,
		initialPrompt,
	)

	var cmd *exec.Cmd
	var captainPaneID int

	// Check if we're running inside WezTerm by looking for WEZTERM_PANE env var
	weztermPane := os.Getenv("WEZTERM_PANE")

	if weztermPane != "" {
		// We're inside WezTerm - split pane below current pane
		fmt.Printf("[SUPERVISOR] Running inside WezTerm (pane %s), splitting for Captain\n", weztermPane)

		// Split the current pane to create Captain's pane below
		splitCmd := exec.Command("wezterm.exe", "cli", "split-pane",
			"--bottom",
			"--percent", "70",
			"--cwd", s.basePath,
			"--", "cmd.exe")

		output, err := splitCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to split pane for Captain: %w (output: %s)", err, string(output))
		}

		// Parse pane ID from output
		paneIDStr := strings.TrimSpace(string(output))
		if paneID, parseErr := strconv.Atoi(paneIDStr); parseErr == nil {
			captainPaneID = paneID
			fmt.Printf("[SUPERVISOR] Captain pane created: %d\n", captainPaneID)

			// Notify spawner of Captain's pane ID
			if onPaneIDReady != nil {
				onPaneIDReady(captainPaneID)
			}

			// Small delay then send the command
			time.Sleep(500 * time.Millisecond)

			// Send the claude command to the new pane
			sendCmd := exec.Command("wezterm.exe", "cli", "send-text",
				"--pane-id", paneIDStr,
				"--no-paste")
			sendCmd.Stdin = strings.NewReader(claudeCmd + "\r\n")
			if sendErr := sendCmd.Run(); sendErr != nil {
				fmt.Printf("[SUPERVISOR] Warning: Failed to send command to Captain pane: %v\n", sendErr)
			}
		} else {
			fmt.Printf("[SUPERVISOR] Warning: Could not parse pane ID from output: %s\n", paneIDStr)
		}

		// We don't have a direct handle to the process, mark as running
		s.mu.Lock()
		s.captainPID = 0 // Can't track PID when using split-pane
		s.status = StatusRunning
		s.startTime = time.Now()
		s.mu.Unlock()

		// Store pane ID for later cleanup
		s.mu.Lock()
		s.captainPaneID = captainPaneID
		s.mu.Unlock()

		return nil
	}

	// Not in WezTerm - spawn a new WezTerm window
	fmt.Println("[SUPERVISOR] Not in WezTerm, spawning new WezTerm window for Captain")

	// Create launcher script with PID tracking
	launcherScript := fmt.Sprintf(`@echo off
title CLIAIMONITOR-Captain

echo.
echo   ================================================
echo     CLIAIMONITOR CAPTAIN - Orchestrator
echo   ================================================
echo.
echo   Dashboard: http://localhost:%d
echo   Project:   %s
echo.

cd /d "%s"

%s
`, s.serverPort, s.basePath, s.basePath, claudeCmd)

	// Write launcher script
	launcherFile := filepath.Join(os.TempDir(), "cliaimonitor-captain-launcher.cmd")
	if err := os.WriteFile(launcherFile, []byte(launcherScript), 0644); err != nil {
		return fmt.Errorf("failed to write launcher script: %w", err)
	}

	// Spawn in WezTerm
	cmd = exec.Command("wezterm.exe", "start", "--always-new-process",
		"--class", "CLIAIMONITOR",
		"--cwd", s.basePath,
		"--", "cmd.exe", "/c", launcherFile)

	if err := cmd.Start(); err != nil {
		s.mu.Lock()
		s.status = StatusCrashed
		s.mu.Unlock()
		return fmt.Errorf("failed to spawn captain in WezTerm: %w", err)
	}

	s.mu.Lock()
	s.captainCmd = cmd
	s.captainPID = cmd.Process.Pid
	s.status = StatusRunning
	s.startTime = time.Now()
	s.mu.Unlock()

	// Monitor the process in a goroutine
	go s.monitorCaptain(cmd)

	return nil
}

// monitorCaptain watches the Captain process and handles exit
func (s *CaptainSupervisor) monitorCaptain(cmd *exec.Cmd) {
	// Wait for process to exit
	err := cmd.Wait()

	s.mu.Lock()
	s.lastExitTime = time.Now()

	// Determine exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1 // Unknown error
		}
	}
	s.lastExitCode = exitCode

	// Handle based on exit code
	if exitCode == 0 {
		// Check if process ran long enough to be a real Captain exit
		// wezterm.exe launcher exits immediately with code 0 when using --always-new-process
		// A real Captain session would run for at least 5 seconds
		runtime := time.Since(s.startTime)
		if runtime < 5*time.Second {
			// This is just the launcher exiting, not Captain
			s.status = StatusRunning // Captain is actually running in the new terminal
			s.captainPID = 0         // We don't have the real PID
			s.captainCmd = nil       // Can't track the real process
			s.mu.Unlock()
			fmt.Printf("WezTerm launcher exited (runtime: %v) - Captain running in separate terminal\n", runtime)
			return
		}

		// Clean exit after real runtime - trigger server shutdown
		s.status = StatusStopped
		callback := s.onShutdownRequest
		s.mu.Unlock()

		fmt.Println("Captain exited cleanly (code 0) - initiating server shutdown")
		s.shutdownOnce.Do(func() {
			close(s.shutdownChan)
		})

		if callback != nil {
			callback()
		}
		return
	}

	// Crash - check if we should respawn
	fmt.Printf("Captain crashed with exit code %d\n", exitCode)
	s.status = StatusCrashed

	// Check crash loop protection
	now := time.Now()
	if s.respawnWindow.IsZero() || now.Sub(s.respawnWindow) > s.windowDuration {
		// Reset window
		s.respawnWindow = now
		s.respawnCount = 1
	} else {
		s.respawnCount++
	}

	if s.respawnCount > s.maxRespawns {
		// Too many crashes - disable auto-respawn
		s.status = StatusDisabled
		s.mu.Unlock()
		fmt.Printf("Captain crash loop detected (%d crashes in %v) - auto-respawn disabled\n",
			s.respawnCount, s.windowDuration)
		fmt.Println("Use dashboard or API to manually restart Captain")
		return
	}

	s.status = StatusRestarting
	s.mu.Unlock()

	// Wait a moment before respawning
	fmt.Printf("Respawning Captain in 2 seconds (attempt %d/%d)...\n", s.respawnCount, s.maxRespawns)
	time.Sleep(2 * time.Second)

	if err := s.spawnCaptain(); err != nil {
		fmt.Printf("Failed to respawn Captain: %v\n", err)
		s.mu.Lock()
		s.status = StatusCrashed
		s.mu.Unlock()
	}
}

// buildCaptainPrompt creates the system prompt for Captain
func (s *CaptainSupervisor) buildCaptainPrompt() string {
	return fmt.Sprintf(`You are Captain, the orchestrator of the CLIAIMONITOR AI agent system.

## Your Role
You coordinate AI agents to work on software development tasks across the Magnolia ecosystem (MAH, MSS, MSS-AI, Planner). You are the central intelligence that monitors, directs, and learns from all agent activity.

## YOUR MONITORING INFRASTRUCTURE

### Dashboard & API (http://localhost:%d)
The dashboard shows real-time state. You can query everything via curl:

**Core State:**
- curl http://localhost:%d/api/state          # Full state: agents, alerts, human requests, metrics
- curl http://localhost:%d/api/health         # System health, uptime, agent counts
- curl http://localhost:%d/api/stats          # Session statistics

**Captain Orchestration:**
- curl http://localhost:%d/api/captain/status       # Your orchestration queue and status
- curl http://localhost:%d/api/captain/subagents    # Active subagent processes
- curl http://localhost:%d/api/captain/escalations  # Issues requiring human review

**Agent Management:**
- curl -X POST http://localhost:%d/api/agents/spawn -d '{"config_name":"Snake","project_path":"...","task":"..."}'
- curl -X POST http://localhost:%d/api/agents/{id}/stop
- curl -X POST http://localhost:%d/api/agents/cleanup  # Remove stale disconnected agents

### SQLite Memory Database (data/memory.db)
You have a persistent memory across sessions! Query it with sqlite3:

**Tables:**
- repos: Discovered git repositories (id, base_path, git_remote, last_scanned)
- repo_files: Important files like CLAUDE.md (repo_id, file_path, content)
- agent_learnings: Knowledge from all agents (agent_id, category, title, content)
- workflow_tasks: Tasks parsed from plans (id, title, status, assigned_agent_id, priority)
- human_decisions: All human approvals/guidance (context, question, answer, decision_type)
- deployments: Agent spawn history (repo_id, deployment_plan, status, agent_configs)
- context_summaries: Session summaries (session_id, agent_id, summary)

**Example Queries:**
sqlite3 data/memory.db "SELECT * FROM repos"
sqlite3 data/memory.db "SELECT title, status, assigned_agent_id FROM workflow_tasks WHERE status='pending'"
sqlite3 data/memory.db "SELECT category, title, content FROM agent_learnings ORDER BY created_at DESC LIMIT 10"
sqlite3 data/memory.db "SELECT question, answer, decision_type FROM human_decisions ORDER BY created_at DESC LIMIT 5"

### State File (data/state.json)
Real-time dashboard state (JSON). Check this for:
- agents: Currently spawned agents with PID, status, current_task
- alerts: Active alerts (unacknowledged issues)
- human_requests: Pending questions from agents needing human input
- stop_requests: Agents requesting permission to stop
- metrics: Token usage, costs, error counts per agent

## Projects in This Ecosystem
- MAH: Hosting platform (Go) at ../MAH - Magnolia Auto Host
- MSS: Firewall/IPS (Go) at ../MSS - Security server
- MSS-AI: AI agent system (Go) at ../mss-ai
- Planner: Task management API at ../planner
- CLIAIMONITOR: This system at %s

## Available Agent Types
Spawn these via the dashboard or API:
- Snake: Opus-powered reconnaissance/scanning agent
- SNTGreen: Sonnet implementation agent (standard tasks)
- SNTPurple: Sonnet analysis/review agent
- OpusGreen: Opus for high-priority implementation
- OpusRed: Opus for critical security work

## Workflow
1. Check your current state: curl http://localhost:%d/api/state
2. Review any pending escalations or human requests
3. For user requests, decide: do it yourself OR spawn specialized agents
4. Track agent progress via the dashboard or API
5. Query memory DB for context from previous sessions

## Spawning Subagents
For quick headless tasks (output captured):
  claude --print "task description"

For persistent terminal agents (use the API):
  curl -X POST http://localhost:%d/api/agents/spawn \
    -H "Content-Type: application/json" \
    -d '{"config_name":"Snake","project_path":"C:/path/to/project","task":"Scan for security issues"}'

## MCP Tools (PREFERRED - Use These!)
You have MCP tools available via the cliaimonitor server. These are your PRIMARY interface:

**Registration & Status:**
- mcp__cliaimonitor__register_agent - Register yourself on startup (agent_id='Captain', role='Orchestrator')
- mcp__cliaimonitor__report_status - Update your status (idle, busy, working)
- mcp__cliaimonitor__send_heartbeat - Keep connection alive

**Context Persistence (Your Memory):**
- mcp__cliaimonitor__save_context - Save key-value context (survives restarts!)
- mcp__cliaimonitor__get_context - Get a specific context entry
- mcp__cliaimonitor__get_all_context - Get ALL saved context (call on startup!)
- mcp__cliaimonitor__log_session - Log significant events

**Common Context Keys:**
- current_focus: What you're currently working on
- recent_work: Summary of recent completed work
- pending_tasks: Tasks waiting to be done
- known_issues: Issues discovered but not yet fixed

**Workflow with MCP:**
1. On startup: Call register_agent, then get_all_context to restore state
2. When starting work: save_context with current_focus
3. When completing work: save_context with recent_work
4. Periodically: send_heartbeat to stay connected

## Important
- When you exit normally (/exit), the entire CLIAIMONITOR system shuts down gracefully
- If you crash, you will be auto-restarted (up to 3 times per minute)
- Use MCP tools for context persistence - they survive restarts!
- Use the API to spawn agents rather than running claude directly for better tracking

Be proactive: check your monitoring infrastructure, review pending items, and coordinate work efficiently.
`, s.serverPort, s.serverPort, s.serverPort, s.serverPort, s.serverPort, s.serverPort, s.serverPort, s.serverPort, s.serverPort, s.serverPort, s.basePath, s.serverPort, s.serverPort)
}
