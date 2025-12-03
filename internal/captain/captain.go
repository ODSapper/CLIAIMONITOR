package captain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/CLIAIMONITOR/internal/agents"
	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/supervisor"
	"github.com/CLIAIMONITOR/internal/types"
)

// AgentMode determines how an agent is spawned
type AgentMode string

const (
	// ModeSubagent spawns a quick, fire-and-forget agent that captures output
	ModeSubagent AgentMode = "subagent"
	// ModeTerminal spawns a persistent agent in Windows Terminal with MCP
	ModeTerminal AgentMode = "terminal"
)

// TaskType categorizes tasks for mode selection
type TaskType string

const (
	TaskRecon          TaskType = "recon"          // Reconnaissance, scanning
	TaskAnalysis       TaskType = "analysis"       // Code review, analysis
	TaskImplementation TaskType = "implementation" // Writing/modifying code
	TaskTesting        TaskType = "testing"        // Running tests
	TaskPlanning       TaskType = "planning"       // Task management, API calls
)

// Captain is the orchestrator that decides how to spawn agents
type Captain struct {
	mu           sync.RWMutex
	basePath     string
	spawner      *agents.ProcessSpawner
	memDB        memory.MemoryDB
	configs      map[string]types.AgentConfig
	plannerAPIKey string
	plannerURL   string

	// Active subagent tracking
	activeSubagents map[string]*SubagentResult

	// Orchestration state
	running        bool
	lastCycle      time.Time
	cycleInterval  time.Duration
	escalations    []Escalation
	taskQueue      []*CaptainTask
	decisionEngine supervisor.DecisionEngine
	reportParser   supervisor.ReportParser
}

// SubagentResult contains the output from a subagent execution
type SubagentResult struct {
	AgentID     string        `json:"agent_id"`
	TaskType    TaskType      `json:"task_type"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Duration    time.Duration `json:"duration"`
	Output      string        `json:"output"`
	ExitCode    int           `json:"exit_code"`
	Error       string        `json:"error,omitempty"`
	Status      string        `json:"status"` // running, completed, failed
}

// Mission describes a task to be executed
type Mission struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	TaskType     TaskType          `json:"task_type"`
	ProjectPath  string            `json:"project_path"`
	Priority     int               `json:"priority"`
	RequiresHuman bool             `json:"requires_human"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ModeDecision explains why a particular mode was chosen
type ModeDecision struct {
	Mode        AgentMode `json:"mode"`
	Reason      string    `json:"reason"`
	AgentType   string    `json:"agent_type"`
	Parallelizable bool   `json:"parallelizable"`
}

// CaptainTask holds a task with optional recon report
type CaptainTask struct {
	Mission      Mission                `json:"mission"`
	NeedsRecon   bool                   `json:"needs_recon"`
	ReconReport  *supervisor.ReconReport `json:"recon_report,omitempty"`
	ActionPlan   *supervisor.ActionPlan  `json:"action_plan,omitempty"`
	Status       string                 `json:"status"` // pending, recon_running, recon_complete, analyzing, executing, completed, failed
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// Escalation represents a task requiring human intervention
type Escalation struct {
	ID           string    `json:"id"`
	TaskID       string    `json:"task_id"`
	AgentID      string    `json:"agent_id"`
	Reason       string    `json:"reason"`
	Context      string    `json:"context"`
	Question     string    `json:"question"`
	CreatedAt    time.Time `json:"created_at"`
	Resolved     bool      `json:"resolved"`
	Resolution   string    `json:"resolution,omitempty"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

// NewCaptain creates a new Captain orchestrator
func NewCaptain(basePath string, spawner *agents.ProcessSpawner, memDB memory.MemoryDB, configs map[string]types.AgentConfig) *Captain {
	return &Captain{
		basePath:        basePath,
		spawner:         spawner,
		memDB:           memDB,
		configs:         configs,
		plannerURL:      "https://plannerprojectmss.vercel.app/api/v1",
		activeSubagents: make(map[string]*SubagentResult),
		running:         false,
		cycleInterval:   30 * time.Second,
		escalations:     make([]Escalation, 0),
		taskQueue:       make([]*CaptainTask, 0),
		decisionEngine:  supervisor.NewDecisionEngine(memDB),
		reportParser:    supervisor.NewReportParser(),
	}
}

// SetPlannerAPIKey sets the API key for Planner integration
func (c *Captain) SetPlannerAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.plannerAPIKey = key
}

// DecideMode determines the best execution mode for a mission
// All tasks run as subagents (headless) in the Captain's process
func (c *Captain) DecideMode(mission Mission) ModeDecision {
	decision := ModeDecision{
		Mode:           ModeSubagent, // Always use subagent mode - runs in same process
		Parallelizable: true,
	}

	switch mission.TaskType {
	case TaskRecon:
		decision.Reason = "Recon: scanning and reporting findings"
		decision.AgentType = "Snake"

	case TaskAnalysis:
		decision.Reason = "Analysis: reviewing code and providing assessment"
		decision.AgentType = "SNTPurple"

	case TaskImplementation:
		decision.Reason = "Implementation: writing/modifying code"
		decision.AgentType = selectImplementationAgent(mission.Priority)
		decision.Parallelizable = false // Sequential for code changes

	case TaskTesting:
		decision.Reason = "Testing: running tests and reporting results"
		decision.AgentType = "SNTGreen"

	case TaskPlanning:
		decision.Reason = "Planning: task management and coordination"
		decision.AgentType = "Planner"

	default:
		decision.Reason = "General task execution"
		decision.AgentType = "SNTGreen"
	}

	return decision
}

// selectImplementationAgent picks the right agent based on priority
func selectImplementationAgent(priority int) string {
	if priority <= 1 {
		return "OpusRed" // Critical security - use Opus
	} else if priority <= 2 {
		return "OpusGreen" // High priority architecture
	}
	return "SNTGreen" // Standard implementation
}

// ExecuteMission runs a mission using the appropriate mode
func (c *Captain) ExecuteMission(ctx context.Context, mission Mission) (*SubagentResult, error) {
	decision := c.DecideMode(mission)

	switch decision.Mode {
	case ModeSubagent:
		return c.executeSubagent(ctx, mission, decision)
	case ModeTerminal:
		return c.executeTerminal(ctx, mission, decision)
	default:
		return nil, fmt.Errorf("unknown mode: %s", decision.Mode)
	}
}

// ExecuteMissionsParallel runs multiple missions in parallel (subagent mode only)
func (c *Captain) ExecuteMissionsParallel(ctx context.Context, missions []Mission) []*SubagentResult {
	var wg sync.WaitGroup
	results := make([]*SubagentResult, len(missions))

	for i, mission := range missions {
		wg.Add(1)
		go func(idx int, m Mission) {
			defer wg.Done()
			result, err := c.ExecuteMission(ctx, m)
			if err != nil {
				results[idx] = &SubagentResult{
					AgentID:  fmt.Sprintf("mission-%d", idx),
					Status:   "failed",
					Error:    err.Error(),
					TaskType: m.TaskType,
				}
				return
			}
			results[idx] = result
		}(i, mission)
	}

	wg.Wait()
	return results
}

// executeSubagent spawns a quick Claude agent and captures output
func (c *Captain) executeSubagent(ctx context.Context, mission Mission, decision ModeDecision) (*SubagentResult, error) {
	// Generate team-compatible agent ID (e.g., team-opusgreen001)
	var agentID string
	if c.spawner != nil {
		agentID = c.spawner.GenerateAgentID(decision.AgentType)
	} else {
		// Fallback for tests where spawner is nil
		agentID = fmt.Sprintf("team-%s", strings.ToLower(decision.AgentType))
	}

	result := &SubagentResult{
		AgentID:   agentID,
		TaskType:  mission.TaskType,
		StartTime: time.Now(),
		Status:    "running",
	}

	c.mu.Lock()
	c.activeSubagents[agentID] = result
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.activeSubagents, agentID)
		c.mu.Unlock()
	}()

	// Build the prompt for Claude
	prompt := c.buildSubagentPrompt(mission, decision)

	// Create a temporary prompt file
	promptFile := filepath.Join(c.basePath, "data", fmt.Sprintf("subagent-%s.md", agentID))
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return nil, fmt.Errorf("failed to write prompt file: %w", err)
	}
	defer os.Remove(promptFile)

	// Run Claude CLI with the prompt
	// Using --print flag to just get the response without interactive mode
	args := []string{
		"--print",           // Non-interactive, print response
		"--dangerously-skip-permissions", // Skip permission prompts for automation
	}

	// Add model selection based on agent type
	model := c.getModelForAgent(decision.AgentType)
	if model != "" {
		args = append(args, "--model", model)
	}

	// Add the prompt
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = mission.ProjectPath

	// Capture output
	output, err := cmd.CombinedOutput()

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Output = string(output)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Status = "failed"
		result.Error = err.Error()
	} else {
		result.Status = "completed"
		result.ExitCode = 0
	}

	return result, nil
}

// executeTerminal spawns a persistent agent in Windows Terminal
func (c *Captain) executeTerminal(ctx context.Context, mission Mission, decision ModeDecision) (*SubagentResult, error) {
	config, exists := c.configs[decision.AgentType]
	if !exists {
		// Try to find a matching config
		for name, cfg := range c.configs {
			if strings.Contains(strings.ToLower(name), strings.ToLower(decision.AgentType)) {
				config = cfg
				exists = true
				break
			}
		}
	}

	if !exists {
		return nil, fmt.Errorf("no config found for agent type: %s", decision.AgentType)
	}

	// Spawner is required for terminal mode
	if c.spawner == nil {
		return nil, fmt.Errorf("spawner not configured - terminal mode unavailable")
	}

	// Generate team-compatible agent ID (e.g., team-opusgreen001)
	agentID := c.spawner.GenerateAgentID(decision.AgentType)
	initialPrompt := fmt.Sprintf(
		"You are agent '%s' with role '%s'. Register via MCP: mcp__cliaimonitor__register_agent with agent_id='%s'. "+
			"Your team ID for Planner API is '%s'. "+
			"Mission: %s. Description: %s. Work autonomously. Report progress via MCP tools.",
		agentID, config.Role, agentID, agentID, mission.Title, mission.Description,
	)

	pid, err := c.spawner.SpawnAgent(config, agentID, mission.ProjectPath, initialPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn terminal agent: %w", err)
	}

	return &SubagentResult{
		AgentID:   agentID,
		TaskType:  mission.TaskType,
		StartTime: time.Now(),
		Status:    "spawned",
		ExitCode:  pid, // Store PID in exit code field for terminal agents
		Output:    fmt.Sprintf("Terminal agent spawned with PID %d", pid),
	}, nil
}

// buildSubagentPrompt creates a focused prompt for subagent execution
func (c *Captain) buildSubagentPrompt(mission Mission, decision ModeDecision) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Mission: %s\n\n", mission.Title))
	sb.WriteString(fmt.Sprintf("## Task Type: %s\n\n", mission.TaskType))
	sb.WriteString(fmt.Sprintf("## Project Path: %s\n\n", mission.ProjectPath))
	sb.WriteString(fmt.Sprintf("## Description\n%s\n\n", mission.Description))

	// Add role-specific instructions
	switch mission.TaskType {
	case TaskRecon:
		sb.WriteString("## Instructions\n")
		sb.WriteString("You are a reconnaissance agent. Your mission is to scan, analyze, and report.\n")
		sb.WriteString("- Observe only - do not modify files\n")
		sb.WriteString("- Provide findings in structured YAML format\n")
		sb.WriteString("- Prioritize by severity (critical > high > medium > low)\n")
		sb.WriteString("- Include file:line references where applicable\n")

	case TaskAnalysis:
		sb.WriteString("## Instructions\n")
		sb.WriteString("You are an analysis agent. Review the target and provide assessment.\n")
		sb.WriteString("- Be thorough but concise\n")
		sb.WriteString("- Provide specific recommendations\n")
		sb.WriteString("- Reference specific code locations\n")

	case TaskPlanning:
		sb.WriteString("## Instructions\n")
		sb.WriteString("You are a planning agent. Interact with the Planner API.\n")
		sb.WriteString(fmt.Sprintf("- API Base: %s\n", c.plannerURL))
		if c.plannerAPIKey != "" {
			sb.WriteString("- Use X-API-Key header for authenticated requests\n")
		}
		sb.WriteString("- Return structured JSON results\n")

	case TaskTesting:
		sb.WriteString("## Instructions\n")
		sb.WriteString("Run the requested tests and report results.\n")
		sb.WriteString("- Include pass/fail counts\n")
		sb.WriteString("- Note any failures with details\n")
		sb.WriteString("- Provide coverage information if available\n")

	default:
		sb.WriteString("## Instructions\n")
		sb.WriteString("Complete the mission and report results.\n")
	}

	// Add metadata if present
	if len(mission.Metadata) > 0 {
		sb.WriteString("\n## Additional Context\n")
		for k, v := range mission.Metadata {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
		}
	}

	return sb.String()
}

// getModelForAgent returns the appropriate Claude model for an agent type
func (c *Captain) getModelForAgent(agentType string) string {
	// Check if we have a config for this agent
	if config, exists := c.configs[agentType]; exists {
		return config.Model
	}

	// Default model selection based on agent type prefix
	agentLower := strings.ToLower(agentType)
	if strings.HasPrefix(agentLower, "opus") || strings.HasPrefix(agentLower, "snake") {
		return "claude-opus-4-5-20251101"
	}
	return "claude-sonnet-4-5-20250929"
}

// GetActiveSubagents returns currently running subagents
func (c *Captain) GetActiveSubagents() map[string]*SubagentResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*SubagentResult)
	for k, v := range c.activeSubagents {
		result[k] = v
	}
	return result
}

// ImportPendingTasks loads tasks from pending_tasks.json and creates missions
func (c *Captain) ImportPendingTasks() ([]Mission, error) {
	tasksFile := filepath.Join(c.basePath, "data", "pending_tasks.json")
	data, err := os.ReadFile(tasksFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read pending tasks: %w", err)
	}

	var pendingData struct {
		Tasks []struct {
			ID               string `json:"id"`
			Title            string `json:"title"`
			Repo             string `json:"repo"`
			Priority         int    `json:"priority"`
			TaskRequirements struct {
				Text              string `json:"text"`
				Source            string `json:"source"`
				EstimatedEffort   string `json:"estimated_effort"`
				AgentRecommendation string `json:"agent_recommendation"`
				RequiresHuman     bool   `json:"requires_human"`
				HumanReason       string `json:"human_reason"`
			} `json:"task_requirements"`
		} `json:"tasks"`
	}

	if err := json.Unmarshal(data, &pendingData); err != nil {
		return nil, fmt.Errorf("failed to parse pending tasks: %w", err)
	}

	var missions []Mission
	for _, task := range pendingData.Tasks {
		// Determine task type from title/description
		taskType := inferTaskType(task.Title, task.TaskRequirements.Text)

		// Map repo to project path
		projectPath := c.resolveProjectPath(task.Repo)

		missions = append(missions, Mission{
			ID:           task.ID,
			Title:        task.Title,
			Description:  task.TaskRequirements.Text,
			TaskType:     taskType,
			ProjectPath:  projectPath,
			Priority:     task.Priority,
			RequiresHuman: task.TaskRequirements.RequiresHuman,
			Metadata: map[string]string{
				"source":              task.TaskRequirements.Source,
				"estimated_effort":    task.TaskRequirements.EstimatedEffort,
				"agent_recommendation": task.TaskRequirements.AgentRecommendation,
				"repo":                task.Repo,
			},
		})
	}

	return missions, nil
}

// inferTaskType determines the task type from title and description
func inferTaskType(title, description string) TaskType {
	combined := strings.ToLower(title + " " + description)

	if strings.Contains(combined, "scan") || strings.Contains(combined, "recon") ||
		strings.Contains(combined, "audit") || strings.Contains(combined, "discover") {
		return TaskRecon
	}

	if strings.Contains(combined, "review") || strings.Contains(combined, "analyze") ||
		strings.Contains(combined, "assess") {
		return TaskAnalysis
	}

	if strings.Contains(combined, "test") || strings.Contains(combined, "coverage") {
		return TaskTesting
	}

	if strings.Contains(combined, "plan") || strings.Contains(combined, "task") ||
		strings.Contains(combined, "api") {
		return TaskPlanning
	}

	// Default to implementation
	return TaskImplementation
}

// resolveProjectPath maps repo name to full path
func (c *Captain) resolveProjectPath(repo string) string {
	// Base path for Magnolia projects
	basePath := filepath.Dir(c.basePath) // Go up from CLIAIMONITOR

	repoMap := map[string]string{
		"MAH":       filepath.Join(basePath, "MAH"),
		"MSS":       filepath.Join(basePath, "MSS"),
		"mss-ai":    filepath.Join(basePath, "mss-ai"),
		"planner":   filepath.Join(basePath, "planner"),
		"mss-suite": filepath.Join(basePath, "mss-suite"),
	}

	if path, exists := repoMap[repo]; exists {
		return path
	}

	// Default to repo name as subdirectory
	return filepath.Join(basePath, repo)
}

// Run is the main orchestration loop
func (c *Captain) Run(ctx context.Context) {
	c.mu.Lock()
	c.running = true
	c.mu.Unlock()

	ticker := time.NewTicker(c.cycleInterval)
	defer ticker.Stop()

	// Run initial cycle immediately
	c.runCycle(ctx)

	for {
		select {
		case <-ctx.Done():
			c.mu.Lock()
			c.running = false
			c.mu.Unlock()
			return
		case <-ticker.C:
			c.runCycle(ctx)
		}
	}
}

// runCycle executes one orchestration cycle
func (c *Captain) runCycle(ctx context.Context) {
	c.mu.Lock()
	c.lastCycle = time.Now()
	c.mu.Unlock()

	// 1. Check for pending tasks
	tasks := c.checkPendingTasks()

	// 2. For tasks needing recon, spawn Snake
	for _, task := range tasks {
		if task.NeedsRecon && task.Status == "pending" {
			report, err := c.runSnakeRecon(ctx, task)
			if err != nil {
				// Mark task as failed
				task.Status = "failed"
				task.UpdatedAt = time.Now()
				continue
			}
			task.ReconReport = report
			task.Status = "recon_complete"
			task.UpdatedAt = time.Now()
		}
	}

	// 3. Analyze and spawn agents
	for _, task := range tasks {
		if task.Status == "recon_complete" {
			plan := c.analyzeAndPlan(task)
			if plan != nil {
				task.ActionPlan = plan
				task.Status = "analyzing"
				task.UpdatedAt = time.Now()

				// Check for escalation
				if plan.RequiresHuman {
					c.createEscalation(task, plan.EscalationReason)
					task.Status = "escalated"
					continue
				}

				// Execute agent spawns - pass project path from mission
				c.executeAgentSpawns(ctx, plan, task.Mission.ProjectPath)
				task.Status = "executing"
				task.UpdatedAt = time.Now()
			}
		}
	}

	// 4. Health check running agents
	c.checkAgentHealth()

	// 5. Process escalations
	c.processEscalations()

	// Update task queue
	c.mu.Lock()
	c.taskQueue = tasks
	c.mu.Unlock()
}

// checkPendingTasks loads tasks from pending_tasks.json or internal queue
func (c *Captain) checkPendingTasks() []*CaptainTask {
	c.mu.RLock()
	existingTasks := make([]*CaptainTask, len(c.taskQueue))
	copy(existingTasks, c.taskQueue)
	c.mu.RUnlock()

	// Try to load new tasks from pending_tasks.json
	missions, err := c.ImportPendingTasks()
	if err == nil && len(missions) > 0 {
		// Convert missions to CaptainTasks
		for _, mission := range missions {
			// Check if already in queue
			found := false
			for _, existing := range existingTasks {
				if existing.Mission.ID == mission.ID {
					found = true
					break
				}
			}

			if !found {
				task := &CaptainTask{
					Mission:    mission,
					NeedsRecon: shouldRunRecon(mission),
					Status:     "pending",
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
				}
				existingTasks = append(existingTasks, task)
			}
		}
	}

	return existingTasks
}

// shouldRunRecon determines if a mission needs reconnaissance
func shouldRunRecon(mission Mission) bool {
	// Recon is useful for:
	// 1. Implementation tasks (to understand scope)
	// 2. Large refactors or architectural changes
	// 3. Security-related tasks
	// 4. Tasks in unfamiliar codebases

	if mission.TaskType == TaskImplementation {
		return true
	}

	if mission.TaskType == TaskAnalysis {
		return true
	}

	// Check keywords in description
	desc := strings.ToLower(mission.Description)
	if strings.Contains(desc, "security") ||
		strings.Contains(desc, "refactor") ||
		strings.Contains(desc, "architecture") ||
		strings.Contains(desc, "migrate") {
		return true
	}

	return false
}

// runSnakeRecon spawns a Snake agent to perform reconnaissance
func (c *Captain) runSnakeRecon(ctx context.Context, task *CaptainTask) (*supervisor.ReconReport, error) {
	task.Status = "recon_running"
	task.UpdatedAt = time.Now()

	// Create a reconnaissance mission
	reconMission := Mission{
		ID:           fmt.Sprintf("recon-%s", task.Mission.ID),
		Title:        fmt.Sprintf("Reconnaissance: %s", task.Mission.Title),
		Description:  fmt.Sprintf("Scan and analyze the codebase to understand:\n%s\n\nProvide findings in YAML format.", task.Mission.Description),
		TaskType:     TaskRecon,
		ProjectPath:  task.Mission.ProjectPath,
		Priority:     task.Mission.Priority,
		RequiresHuman: false,
		Metadata: map[string]string{
			"parent_task": task.Mission.ID,
			"format":      "yaml",
		},
	}

	// Execute Snake subagent
	result, err := c.executeSubagent(ctx, reconMission, ModeDecision{
		Mode:      ModeSubagent,
		AgentType: "Snake",
		Reason:    "Reconnaissance mission",
	})

	if err != nil {
		return nil, fmt.Errorf("snake recon failed: %w", err)
	}

	if result.Status != "completed" {
		return nil, fmt.Errorf("snake recon did not complete: %s", result.Status)
	}

	// Parse the output as YAML
	report, err := c.reportParser.ParseYAML([]byte(result.Output))
	if err != nil {
		// Try JSON format as fallback
		report, err = c.reportParser.ParseJSON([]byte(result.Output))
		if err != nil {
			return nil, fmt.Errorf("failed to parse recon report: %w", err)
		}
	}

	// Store report in memory DB
	if err := c.storeReconReport(ctx, report, task.Mission.ProjectPath); err != nil {
		// Log but don't fail - we still have the report
		fmt.Printf("Warning: failed to store recon report: %v\n", err)
	}

	return report, nil
}

// storeReconReport saves a reconnaissance report to the memory database
func (c *Captain) storeReconReport(ctx context.Context, report *supervisor.ReconReport, projectPath string) error {
	// Check if memDB implements ReconRepository interface
	reconRepo, ok := c.memDB.(memory.ReconRepository)
	if !ok {
		// MemoryDB doesn't support reconnaissance storage yet
		// This is not a fatal error - we still have the report in memory
		return fmt.Errorf("memory database does not implement ReconRepository interface")
	}

	// Create or get environment
	env := &memory.Environment{
		ID:          sanitizeEnvID(projectPath),
		Name:        filepath.Base(projectPath),
		Description: fmt.Sprintf("Project at %s", projectPath),
		EnvType:     "internal",
		BasePath:    projectPath,
		Metadata:    make(map[string]interface{}),
	}

	if err := reconRepo.RegisterEnvironment(ctx, env); err != nil {
		return fmt.Errorf("failed to register environment: %w", err)
	}

	// Create scan record
	scan := &memory.ReconScan{
		ID:                 report.ID,
		EnvID:              env.ID,
		AgentID:            report.AgentID,
		ScanType:           "initial",
		Mission:            report.Mission,
		StartedAt:          report.Timestamp,
		Status:             "completed",
		TotalFilesScanned:  report.Summary.TotalFilesScanned,
		LanguagesDetected:  report.Summary.Languages,
		FrameworksDetected: report.Summary.Frameworks,
		SecurityScore:      report.Summary.SecurityScore,
	}

	now := time.Now()
	scan.CompletedAt = &now

	if err := reconRepo.RecordScan(ctx, scan); err != nil {
		return fmt.Errorf("failed to record scan: %w", err)
	}

	// Store all findings
	allFindings := make([]*memory.ReconFinding, 0)

	for _, f := range report.Findings.Critical {
		allFindings = append(allFindings, convertFinding(f, scan.ID, env.ID, "critical"))
	}
	for _, f := range report.Findings.High {
		allFindings = append(allFindings, convertFinding(f, scan.ID, env.ID, "high"))
	}
	for _, f := range report.Findings.Medium {
		allFindings = append(allFindings, convertFinding(f, scan.ID, env.ID, "medium"))
	}
	for _, f := range report.Findings.Low {
		allFindings = append(allFindings, convertFinding(f, scan.ID, env.ID, "low"))
	}

	if len(allFindings) > 0 {
		if err := reconRepo.SaveFindings(ctx, allFindings); err != nil {
			return fmt.Errorf("failed to save findings: %w", err)
		}
	}

	// Update environment last scanned
	if err := reconRepo.UpdateEnvironmentLastScan(ctx, env.ID); err != nil {
		return fmt.Errorf("failed to update environment last scan: %w", err)
	}

	return nil
}

// convertFinding converts supervisor.ReconFinding to memory.ReconFinding
func convertFinding(f *supervisor.ReconFinding, scanID, envID, severity string) *memory.ReconFinding {
	return &memory.ReconFinding{
		ID:              f.ID,
		ScanID:          scanID,
		EnvID:           envID,
		FindingType:     f.Type,
		Severity:        severity,
		Title:           fmt.Sprintf("%s finding", f.Type),
		Description:     f.Description,
		Location:        f.Location,
		Recommendation:  f.Recommendation,
		Status:          "open",
		Metadata:        make(map[string]interface{}),
		DiscoveredAt:    time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// sanitizeEnvID creates a valid environment ID from a path
func sanitizeEnvID(path string) string {
	// Use the base name, replacing invalid characters
	id := filepath.Base(path)
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.ReplaceAll(id, ".", "_")
	id = strings.ToLower(id)
	return id
}

// analyzeAndPlan uses the DecisionEngine to create an action plan
func (c *Captain) analyzeAndPlan(task *CaptainTask) *supervisor.ActionPlan {
	if task.ReconReport == nil {
		return nil
	}

	plan, err := c.decisionEngine.AnalyzeReport(context.Background(), task.ReconReport)
	if err != nil {
		fmt.Printf("Error analyzing report: %v\n", err)
		return nil
	}

	return plan
}

// executeAgentSpawns spawns terminal agents based on action plan
func (c *Captain) executeAgentSpawns(ctx context.Context, plan *supervisor.ActionPlan, projectPath string) {
	// Spawn agents for each recommendation
	for _, rec := range plan.AgentRecommendations {
		mission := Mission{
			ID:           fmt.Sprintf("task-%d-%s", time.Now().UnixNano(), rec.AgentType),
			Title:        rec.Task,
			Description:  fmt.Sprintf("%s\n\nRationale: %s", rec.Task, rec.Rationale),
			TaskType:     TaskImplementation,
			ProjectPath:  projectPath, // Must set project path for agent to work in correct directory
			Priority:     rec.Priority,
			RequiresHuman: plan.RequiresHuman,
			Metadata: map[string]string{
				"plan_id":     plan.ID,
				"agent_type":  rec.AgentType,
				"finding_ids": strings.Join(rec.FindingIDs, ","),
			},
		}

		decision := ModeDecision{
			Mode:      ModeTerminal,
			AgentType: rec.AgentType,
			Reason:    rec.Rationale,
		}

		// Spawn the agent
		_, err := c.executeTerminal(ctx, mission, decision)
		if err != nil {
			fmt.Printf("Error spawning agent %s: %v\n", rec.AgentType, err)
			continue
		}

		fmt.Printf("Spawned agent %s for task: %s\n", rec.AgentType, rec.Task)
	}
}

// checkAgentHealth monitors running agents for staleness or failures
func (c *Captain) checkAgentHealth() {
	// Get running agents from spawner
	runningAgents := c.spawner.GetRunningAgents()

	// Check for stale agents (no activity > 5 min)
	// This would require querying state.json or MCP activity logs
	// For now, just log running agent count
	if len(runningAgents) > 0 {
		fmt.Printf("Health check: %d agents currently running\n", len(runningAgents))
	}

	// In a full implementation, this would:
	// 1. Query state.json for last_seen timestamps
	// 2. Check MCP activity logs for recent tool calls
	// 3. Create escalations for stale agents (no activity > 5 min)
	// 4. Clean up disconnected agents
	// 5. Monitor resource usage (memory, CPU)
	// 6. Check for failed test runs or consecutive errors
}

// processEscalations checks and manages escalation queue
func (c *Captain) processEscalations() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Count unresolved escalations
	unresolvedCount := 0
	for _, esc := range c.escalations {
		if !esc.Resolved {
			unresolvedCount++
		}
	}

	if unresolvedCount > 0 {
		fmt.Printf("Info: %d unresolved escalations pending human review\n", unresolvedCount)
	}

	// In a full implementation, this would:
	// 1. Send notifications via webhook
	// 2. Store escalations in persistent queue
	// 3. Provide HTTP endpoint for human to resolve
	// 4. Resume tasks once escalation is resolved
}

// createEscalation adds a new escalation for a task
func (c *Captain) createEscalation(task *CaptainTask, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	escalation := Escalation{
		ID:        fmt.Sprintf("esc-%d", time.Now().UnixNano()),
		TaskID:    task.Mission.ID,
		AgentID:   "", // Not agent-specific
		Reason:    reason,
		Context:   fmt.Sprintf("Task: %s\nDescription: %s", task.Mission.Title, task.Mission.Description),
		Question:  "This task requires human approval. Should we proceed?",
		CreatedAt: time.Now(),
		Resolved:  false,
	}

	c.escalations = append(c.escalations, escalation)
	fmt.Printf("Escalation created: %s - %s\n", escalation.ID, reason)
}

// createAgentEscalation adds a new escalation for an agent issue
func (c *Captain) createAgentEscalation(agentID, reason, context string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	escalation := Escalation{
		ID:        fmt.Sprintf("esc-agent-%d", time.Now().UnixNano()),
		TaskID:    "",
		AgentID:   agentID,
		Reason:    reason,
		Context:   context,
		Question:  "How should we handle this agent issue?",
		CreatedAt: time.Now(),
		Resolved:  false,
	}

	c.escalations = append(c.escalations, escalation)
	fmt.Printf("Agent escalation created: %s - %s\n", escalation.ID, reason)
}

// GetEscalations returns all escalations
func (c *Captain) GetEscalations() []Escalation {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Escalation, len(c.escalations))
	copy(result, c.escalations)
	return result
}

// GetTaskQueue returns current task queue
func (c *Captain) GetTaskQueue() []*CaptainTask {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*CaptainTask, len(c.taskQueue))
	copy(result, c.taskQueue)
	return result
}

// SetCycleInterval configures the orchestration cycle interval
func (c *Captain) SetCycleInterval(interval time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cycleInterval = interval
}

// IsRunning returns whether the orchestration loop is active
func (c *Captain) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}
