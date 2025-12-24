package agents

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CLIAIMONITOR/internal/instance"
	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/types"
	"github.com/CLIAIMONITOR/internal/wezterm"
)

// Spawner manages agent process lifecycle
type Spawner interface {
	SpawnAgent(config types.AgentConfig, agentID string, projectPath string, initialPrompt string) (pid int, err error)
	SpawnAgentWithOptions(config types.AgentConfig, agentID string, projectPath string, initialPrompt string, headless bool) (pid int, err error)
	StopAgent(agentID string) error
	StopAgentWithReason(agentID string, reason string) error
	IsAgentRunning(pid int) bool
	GetRunningAgents() map[string]int // agentID -> PID
}

// ProcessSpawner implements Spawner using WezTerm
type ProcessSpawner struct {
	mu             sync.RWMutex
	spawnMu        sync.Mutex // Serializes agent spawning to prevent race conditions
	basePath       string     // CLIAIMONITOR directory
	mcpServerURL   string
	scriptsPath    string
	configsPath    string
	runningAgents  map[string]int // agentID -> PID
	agentPanes     map[string]int // agentID -> WezTerm pane ID
	agentCounters  map[string]int // agentType -> sequence counter
	memDB          memory.MemoryDB

	// Headless agents: spawn in dedicated hidden "Agents" workspace
	agentWindowID int // Window ID for headless agents (-1 = not created yet)

	// Visible agents: spawn as tabs in Captain's window (window 0)
	// Each tab holds up to 9 agents in a 3x3 grid
	visibleTabID    int // Current tab ID for visible agents (-1 = no tab yet)
	visibleTabPanes int // Count of panes in current visible tab
}

// NewSpawner creates a new process spawner
func NewSpawner(basePath string, mcpServerURL string, memDB memory.MemoryDB) *ProcessSpawner {
	return &ProcessSpawner{
		basePath:        basePath,
		mcpServerURL:    mcpServerURL,
		scriptsPath:     filepath.Join(basePath, "scripts"),
		configsPath:     filepath.Join(basePath, "configs"),
		runningAgents:   make(map[string]int),
		agentPanes:      make(map[string]int),
		agentCounters:   make(map[string]int),
		memDB:           memDB,
		agentWindowID:   -1, // No headless window yet
		visibleTabID:    -1, // No visible agent tab yet
		visibleTabPanes: 0,
	}
}

// SetMemoryDB sets the memory database for the spawner
func (s *ProcessSpawner) SetMemoryDB(db memory.MemoryDB) {
	s.memDB = db
}

// PaneInfo holds WezTerm pane information for dynamic grid layout
type PaneInfo struct {
	PaneID   int `json:"pane_id"`
	WindowID int `json:"window_id"`
	TabID    int `json:"tab_id"`
	TopRow   int `json:"top_row"`
	LeftCol  int `json:"left_col"`
}

// getAgentWindowPanes queries WezTerm for all panes, grouped by tab in agent window
func (s *ProcessSpawner) getAgentWindowPanes() (map[int][]PaneInfo, error) {
	if s.agentWindowID < 0 {
		return nil, nil
	}

	cmd := exec.Command("wezterm.exe", "cli", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list panes: %w", err)
	}

	var panes []PaneInfo
	if err := json.Unmarshal(output, &panes); err != nil {
		return nil, fmt.Errorf("failed to parse panes: %w", err)
	}

	// Group by tab_id, filtered to agent window only
	tabPanes := make(map[int][]PaneInfo)
	for _, p := range panes {
		if p.WindowID == s.agentWindowID {
			tabPanes[p.TabID] = append(tabPanes[p.TabID], p)
		}
	}
	return tabPanes, nil
}

// getSpawnTarget determines where to spawn the next agent dynamically
// Returns: (needsNewWindow, needsNewTab, splitFromPaneID, splitDirection)
func (s *ProcessSpawner) getSpawnTarget() (needsNewWindow, needsNewTab bool, splitFromPaneID int, splitDirection string) {
	// No agent window yet - create one
	if s.agentWindowID < 0 {
		return true, false, 0, ""
	}

	// Query current panes
	tabPanes, err := s.getAgentWindowPanes()
	if err != nil {
		log.Printf("[SPAWNER] Error querying panes: %v, creating new window", err)
		return true, false, 0, ""
	}

	// Find a tab with room (< 9 panes)
	var targetTab int = -1
	var panes []PaneInfo
	for tid, ps := range tabPanes {
		if len(ps) < 9 {
			targetTab = tid
			panes = ps
			break
		}
	}

	// All tabs full - need new tab
	if targetTab < 0 {
		for _, ps := range tabPanes {
			if len(ps) > 0 {
				return false, true, ps[0].PaneID, ""
			}
		}
		return true, false, 0, "" // No panes at all, new window
	}

	count := len(panes)
	if count == 0 {
		return false, true, 0, ""
	}

	// Sort panes by grid position (top_row, left_col)
	sort.Slice(panes, func(i, j int) bool {
		if panes[i].TopRow != panes[j].TopRow {
			return panes[i].TopRow < panes[j].TopRow
		}
		return panes[i].LeftCol < panes[j].LeftCol
	})

	// Determine split target based on current count
	// Grid: [0][1][2] / [3][4][5] / [6][7][8]
	switch count {
	case 1:
		return false, false, panes[0].PaneID, "right"
	case 2:
		return false, false, panes[1].PaneID, "right"
	case 3:
		return false, false, panes[0].PaneID, "bottom"
	case 4:
		return false, false, panes[3].PaneID, "right"
	case 5:
		return false, false, panes[4].PaneID, "right"
	case 6:
		return false, false, panes[3].PaneID, "bottom"
	case 7:
		return false, false, panes[6].PaneID, "right"
	case 8:
		return false, false, panes[7].PaneID, "right"
	default:
		return false, true, panes[0].PaneID, ""
	}
}

// getVisibleSpawnTarget determines where to spawn the next visible agent in Captain's window
// Returns: (needsNewTab, splitFromPaneID, splitDirection)
func (s *ProcessSpawner) getVisibleSpawnTarget() (needsNewTab bool, splitFromPaneID int, splitDirection string) {
	// Query all panes to find Captain's window and visible agent tabs
	cmd := exec.Command("wezterm.exe", "cli", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[SPAWNER] Error querying panes for visible spawn: %v", err)
		return true, 0, "" // Create new tab using pane 0
	}

	var allPanes []PaneInfo
	if err := json.Unmarshal(output, &allPanes); err != nil {
		log.Printf("[SPAWNER] Error parsing panes: %v", err)
		return true, 0, ""
	}

	// Find Captain's window (window containing pane 0, typically window 0)
	var captainWindowID int = -1
	var captainTabID int = -1
	for _, p := range allPanes {
		if p.PaneID == 0 {
			captainWindowID = p.WindowID
			captainTabID = p.TabID
			break
		}
	}

	if captainWindowID < 0 {
		log.Printf("[SPAWNER] Could not find Captain's window, using pane 0")
		return true, 0, ""
	}

	// Find tabs in Captain's window that are NOT Captain's tab (those are agent tabs)
	agentTabPanes := make(map[int][]PaneInfo)
	for _, p := range allPanes {
		if p.WindowID == captainWindowID && p.TabID != captainTabID {
			agentTabPanes[p.TabID] = append(agentTabPanes[p.TabID], p)
		}
	}

	// Find a tab with room (< 9 panes)
	var targetTabID int = -1
	var panes []PaneInfo
	for tid, ps := range agentTabPanes {
		if len(ps) < 9 {
			targetTabID = tid
			panes = ps
			break
		}
	}

	// No agent tabs or all full - create new tab
	if targetTabID < 0 || len(panes) == 0 {
		// Use pane 0 to create a new tab in Captain's window
		return true, 0, ""
	}

	// Sort panes by grid position
	sort.Slice(panes, func(i, j int) bool {
		if panes[i].TopRow != panes[j].TopRow {
			return panes[i].TopRow < panes[j].TopRow
		}
		return panes[i].LeftCol < panes[j].LeftCol
	})

	count := len(panes)

	// Grid: [0][1][2] / [3][4][5] / [6][7][8]
	switch count {
	case 1:
		return false, panes[0].PaneID, "right"
	case 2:
		return false, panes[1].PaneID, "right"
	case 3:
		return false, panes[0].PaneID, "bottom"
	case 4:
		return false, panes[3].PaneID, "right"
	case 5:
		return false, panes[4].PaneID, "right"
	case 6:
		return false, panes[3].PaneID, "bottom"
	case 7:
		return false, panes[6].PaneID, "right"
	case 8:
		return false, panes[7].PaneID, "right"
	default:
		return true, panes[0].PaneID, ""
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


// SpawnAgent launches a team agent in WezTerm (visible mode - tabs in Captain's window)
func (s *ProcessSpawner) SpawnAgent(config types.AgentConfig, agentID string, projectPath string, initialPrompt string) (int, error) {
	return s.SpawnAgentWithOptions(config, agentID, projectPath, initialPrompt, false)
}

// SpawnAgentWithOptions launches a team agent in WezTerm with visibility control
// headless=true: spawns in hidden "Agents" workspace with 3x3 grid (Captain monitors via wezterm_get_text)
// headless=false: spawns as a new tab in Captain's window (visible to user)
func (s *ProcessSpawner) SpawnAgentWithOptions(config types.AgentConfig, agentID string, projectPath string, initialPrompt string, headless bool) (int, error) {
	// Serialize spawns to prevent race conditions when determining spawn target
	s.spawnMu.Lock()
	defer s.spawnMu.Unlock()

	// Escape the initial prompt for shell
	escapedPrompt := strings.ReplaceAll(initialPrompt, `"`, `\"`)
	escapedPrompt = strings.ReplaceAll(escapedPrompt, `'`, `''`)

	// Build command: set title and run Claude directly
	cmdChain := fmt.Sprintf(
		`title %s && claude --model %s --dangerously-skip-permissions "%s"`,
		agentID,
		config.Model,
		escapedPrompt,
	)

	var cmd *exec.Cmd
	var paneID int
	var paneIDStr string

	if _, err := exec.LookPath("wezterm.exe"); err == nil {
		var output []byte
		var spawnErr error

		if headless {
			// HEADLESS MODE: Hidden "Agents" workspace with 3x3 grid
			needsNewWindow, needsNewTab, splitFromPaneID, splitDirection := s.getSpawnTarget()

			if needsNewWindow {
				log.Printf("[SPAWNER] Creating headless agent window in Agents workspace")
				cmd = exec.Command("wezterm.exe", "cli", "spawn",
					"--new-window",
					"--workspace", "Agents",
					"--cwd", projectPath,
					"--", "cmd.exe")

				output, spawnErr = cmd.CombinedOutput()
				if spawnErr != nil {
					return 0, fmt.Errorf("failed to spawn agent window: %w", spawnErr)
				}

				paneIDStr = strings.TrimSpace(string(output))
				if parsedID, parseErr := strconv.Atoi(paneIDStr); parseErr == nil {
					paneID = parsedID
					listCmd := exec.Command("wezterm.exe", "cli", "list", "--format", "json")
					if listOut, listErr := listCmd.Output(); listErr == nil {
						var panes []PaneInfo
						if json.Unmarshal(listOut, &panes) == nil {
							for _, p := range panes {
								if p.PaneID == paneID {
									s.agentWindowID = p.WindowID
									log.Printf("[SPAWNER] Headless window created: window_id=%d, pane=%d", s.agentWindowID, paneID)
									titleCmd := exec.Command("wezterm.exe", "cli", "set-window-title",
										"--window-id", strconv.Itoa(s.agentWindowID),
										"Agent Squad (Headless)")
									titleCmd.Run()
									break
								}
							}
						}
					}
				}
			} else if needsNewTab {
				log.Printf("[SPAWNER] Creating new tab in headless window (pane %d)", splitFromPaneID)
				cmd = exec.Command("wezterm.exe", "cli", "spawn",
					"--pane-id", strconv.Itoa(splitFromPaneID),
					"--cwd", projectPath,
					"--", "cmd.exe")
				output, spawnErr = cmd.CombinedOutput()
				if spawnErr != nil {
					return 0, fmt.Errorf("failed to spawn new tab: %w", spawnErr)
				}
			} else {
				log.Printf("[SPAWNER] Splitting pane %d %s", splitFromPaneID, splitDirection)
				cmd = exec.Command("wezterm.exe", "cli", "split-pane",
					"--pane-id", strconv.Itoa(splitFromPaneID),
					"--"+splitDirection,
					"--cwd", projectPath,
					"--", "cmd.exe")
				output, spawnErr = cmd.CombinedOutput()
				if spawnErr != nil {
					return 0, fmt.Errorf("failed to split pane: %w", spawnErr)
				}
			}
		} else {
			// VISIBLE MODE: Agents spawn in Captain's window with 3x3 grid per tab
			needsNewTab, splitFromPaneID, splitDirection := s.getVisibleSpawnTarget()

			if needsNewTab {
				// Create new tab in Captain's window (use pane 0 as reference)
				log.Printf("[SPAWNER] Creating new visible agent tab in Captain window for %s", agentID)
				cmd = exec.Command("wezterm.exe", "cli", "spawn",
					"--pane-id", "0",
					"--cwd", projectPath,
					"--", "cmd.exe")

				output, spawnErr = cmd.CombinedOutput()
				if spawnErr != nil {
					return 0, fmt.Errorf("failed to spawn visible agent tab: %w", spawnErr)
				}
			} else {
				// Split existing pane in visible agent tab
				log.Printf("[SPAWNER] Splitting visible pane %d %s for %s", splitFromPaneID, splitDirection, agentID)
				cmd = exec.Command("wezterm.exe", "cli", "split-pane",
					"--pane-id", strconv.Itoa(splitFromPaneID),
					"--"+splitDirection,
					"--cwd", projectPath,
					"--", "cmd.exe")

				output, spawnErr = cmd.CombinedOutput()
				if spawnErr != nil {
					return 0, fmt.Errorf("failed to split visible pane: %w", spawnErr)
				}
			}
		}

		// Parse pane ID from output
		if paneID == 0 {
			paneIDStr = strings.TrimSpace(string(output))
			if parsedID, parseErr := strconv.Atoi(paneIDStr); parseErr == nil {
				paneID = parsedID
			}
		}

		if paneID > 0 {
			log.Printf("[SPAWNER] Agent %s spawned in pane %d (headless=%v)", agentID, paneID, headless)

			colors := GetAgentColors(config.Name)
			time.Sleep(300 * time.Millisecond)

			// Set background color
			clearCmd := exec.Command("wezterm.exe", "cli", "send-text", "--pane-id", paneIDStr, "--no-paste")
			clearSeq := fmt.Sprintf("\x1b]11;%s\x07\x1b[2J\x1b[H", colors.BgRGB)
			clearCmd.Stdin = strings.NewReader(clearSeq)
			clearCmd.Run()

			time.Sleep(100 * time.Millisecond)

			// Send command to start Claude
			sendCmd := exec.Command("wezterm.exe", "cli", "send-text", "--pane-id", paneIDStr, "--no-paste")
			sendCmd.Stdin = strings.NewReader(cmdChain + "\r\n")
			if sendErr := sendCmd.Run(); sendErr != nil {
				log.Printf("[SPAWNER] Warning: Failed to send command to pane %d: %v", paneID, sendErr)
			} else {
				log.Printf("[SPAWNER] Command sent to pane %d for agent %s", paneID, agentID)
			}

			// Set tab title for visible agents
			if !headless {
				time.Sleep(100 * time.Millisecond)
				tabTitleCmd := exec.Command("wezterm.exe", "cli", "set-tab-title", "--pane-id", paneIDStr, agentID)
				tabTitleCmd.Run()
			}
		} else {
			log.Printf("[SPAWNER] Warning: Could not parse pane ID from output: %s", paneIDStr)
			paneID = -1
		}
	} else {
		return 0, fmt.Errorf("WezTerm not found in PATH")
	}

	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	log.Printf("[SPAWNER] Agent %s launched (PID: %d, Pane: %d, headless=%v)", agentID, pid, paneID, headless)

	if cmd != nil && cmd.Process != nil {
		go func() {
			cmd.Wait()
		}()
	}

	if paneID > 0 {
		s.SetAgentPaneID(agentID, paneID)
	}

	return pid, nil
}

// StopAgent terminates an agent process (backward compatible - no reason)
func (s *ProcessSpawner) StopAgent(agentID string) error {
	return s.StopAgentWithReason(agentID, "manual stop")
}

// StopAgentWithReason terminates an agent process with a specific reason
func (s *ProcessSpawner) StopAgentWithReason(agentID string, reason string) error {
	log.Printf("[SPAWNER] Stopping agent %s with reason: %s", agentID, reason)

	// 1. Remove from running agents map
	s.mu.Lock()
	delete(s.runningAgents, agentID)
	s.mu.Unlock()

	// 6. Try to kill by WezTerm pane ID first (most reliable method)
	if paneID, ok := s.GetAgentPaneID(agentID); ok && paneID > 0 {
		log.Printf("[SPAWNER] Killing agent %s via pane ID %d", agentID, paneID)
		if err := s.KillByPaneID(paneID); err != nil {
			log.Printf("[SPAWNER] Warning: Failed to kill agent %s by pane ID %d: %v", agentID, paneID, err)
		} else {
			log.Printf("[SPAWNER] Successfully killed agent %s via pane ID %d", agentID, paneID)
		}
		// Always remove pane ID from tracking (pane is either closed or already gone)
		s.mu.Lock()
		delete(s.agentPanes, agentID)
		s.mu.Unlock()
	}

	// 7. Try to kill the agent process using PID file (fallback method)
	// PID file contains PowerShell process PID inside the terminal
	pid, err := s.GetAgentPIDFromFile(agentID)
	if err == nil && pid > 0 {
		log.Printf("[SPAWNER] Killing agent %s (PID: %d) and child processes", agentID, pid)

		// Kill any claude.exe child processes
		if err := s.KillChildClaude(pid); err != nil {
			log.Printf("[SPAWNER] Warning: Failed to kill claude.exe child for agent %s (PID %d): %v", agentID, pid, err)
		}

		// Kill the PowerShell process (this closes the terminal tab)
		if err := instance.KillProcess(pid); err != nil {
			log.Printf("[SPAWNER] Warning: Failed to kill PowerShell by PID for agent %s (PID %d): %v", agentID, pid, err)
		}

		// Clean up PID file
		if err := s.CleanupAgentPIDFile(agentID); err != nil {
			log.Printf("[SPAWNER] Warning: Failed to cleanup PID file for agent %s: %v", agentID, err)
		}
	} else if err != nil {
		log.Printf("[SPAWNER] Warning: Failed to get agent PID from file for agent %s: %v", agentID, err)
	}

	// 8. Also try killing by window title (catches any stragglers)
	if err := s.KillByWindowTitle(agentID); err != nil {
		log.Printf("[SPAWNER] Warning: Failed to kill agent %s by window title: %v", agentID, err)
	}

	// 9. Kill any remaining powershell processes with our temp script
	if err := s.KillByTempScript(agentID); err != nil {
		log.Printf("[SPAWNER] Warning: Failed to kill agent %s by temp script: %v", agentID, err)
	}

	return nil
}

// KillByPaneID kills a WezTerm pane by its pane ID
// Uses centralized WezTerm ops for rate limiting and timeout handling
func (s *ProcessSpawner) KillByPaneID(paneID int) error {
	return wezterm.Get().KillPane(paneID)
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
		log.Printf("[SPAWNER] Warning: Failed to check if PID %d is running: %v", pid, err)
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
	var lastErr error

	// Clean MCP configs
	mcpDir := filepath.Join(s.configsPath, "mcp")
	entries, err := os.ReadDir(mcpDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), "-mcp.json") {
				filePath := filepath.Join(mcpDir, entry.Name())
				if removeErr := os.Remove(filePath); removeErr != nil && !os.IsNotExist(removeErr) {
					log.Printf("[SPAWNER] Warning: Failed to remove MCP config %s: %v", filePath, removeErr)
					lastErr = removeErr
				}
			}
		}
	} else if !os.IsNotExist(err) {
		log.Printf("[SPAWNER] Warning: Failed to read MCP config directory: %v", err)
		lastErr = err
	}

	// Clean PID tracking files
	pidsDir := filepath.Join(s.basePath, "data", "pids")
	entries, err = os.ReadDir(pidsDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".pid") {
				filePath := filepath.Join(pidsDir, entry.Name())
				if removeErr := os.Remove(filePath); removeErr != nil && !os.IsNotExist(removeErr) {
					log.Printf("[SPAWNER] Warning: Failed to remove PID file %s: %v", filePath, removeErr)
					lastErr = removeErr
				}
			}
		}
	} else if !os.IsNotExist(err) {
		log.Printf("[SPAWNER] Warning: Failed to read PID directory: %v", err)
		lastErr = err
	}

	return lastErr
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

