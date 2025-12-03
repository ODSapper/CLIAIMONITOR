package supervisor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/CLIAIMONITOR/internal/agents"
	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/types"
)

// Dispatcher executes action plans by spawning and coordinating agents
type Dispatcher interface {
	// Execute action plan by spawning agents
	ExecutePlan(ctx context.Context, plan *ActionPlan) (*DispatchResult, error)

	// Spawn single agent with task
	SpawnAgent(ctx context.Context, rec *AgentRecommendation, projectPath string) (string, error)

	// Get status of dispatched agents
	GetDispatchStatus(ctx context.Context, dispatchID string) (*DispatchStatus, error)

	// Cancel/abort a dispatch
	AbortDispatch(ctx context.Context, dispatchID string) error

	// List all dispatches
	ListDispatches(ctx context.Context, filter DispatchFilter) ([]*DispatchSummary, error)
}

// DispatchResult contains the result of executing an action plan
type DispatchResult struct {
	DispatchID    string          `json:"dispatch_id"`
	PlanID        string          `json:"plan_id"`
	Mode          OperationalMode `json:"mode"`
	AgentsSpawned []SpawnedAgent  `json:"agents_spawned"`
	StartTime     time.Time       `json:"start_time"`
	Status        string          `json:"status"` // spawning, running, completed, failed
}

// SpawnedAgent represents an agent that was spawned
type SpawnedAgent struct {
	AgentID   string    `json:"agent_id"`
	AgentType string    `json:"agent_type"`
	Task      string    `json:"task"`
	Status    string    `json:"status"` // spawning, running, completed, failed
	SpawnedAt time.Time `json:"spawned_at"`
	PID       int       `json:"pid,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// DispatchStatus provides current status of a dispatch
type DispatchStatus struct {
	DispatchID      string          `json:"dispatch_id"`
	Status          string          `json:"status"`
	AgentsTotal     int             `json:"agents_total"`
	AgentsRunning   int             `json:"agents_running"`
	AgentsCompleted int             `json:"agents_completed"`
	AgentsFailed    int             `json:"agents_failed"`
	Agents          []SpawnedAgent  `json:"agents"`
	StartTime       time.Time       `json:"start_time"`
	LastUpdated     time.Time       `json:"last_updated"`
}

// DispatchSummary provides a brief overview of a dispatch
type DispatchSummary struct {
	DispatchID    string          `json:"dispatch_id"`
	PlanID        string          `json:"plan_id"`
	Mode          OperationalMode `json:"mode"`
	Status        string          `json:"status"`
	AgentsTotal   int             `json:"agents_total"`
	StartTime     time.Time       `json:"start_time"`
}

// DispatchFilter filters dispatch queries
type DispatchFilter struct {
	Status string
	Limit  int
	Offset int
}

// StandardDispatcher implements the Dispatcher interface
type StandardDispatcher struct {
	memDB     memory.MemoryDB
	spawner   agents.Spawner
	configs   map[string]types.AgentConfig

	mu         sync.RWMutex
	dispatches map[string]*dispatchState
}

// dispatchState tracks internal state of a dispatch
type dispatchState struct {
	result      *DispatchResult
	agents      map[string]*SpawnedAgent
	projectPath string
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewDispatcher creates a new dispatcher
func NewDispatcher(memDB memory.MemoryDB, spawner agents.Spawner, configs map[string]types.AgentConfig) Dispatcher {
	return &StandardDispatcher{
		memDB:      memDB,
		spawner:    spawner,
		configs:    configs,
		dispatches: make(map[string]*dispatchState),
	}
}

// ExecutePlan executes an action plan by spawning agents
func (d *StandardDispatcher) ExecutePlan(ctx context.Context, plan *ActionPlan) (*DispatchResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan is nil")
	}

	// Generate dispatch ID
	dispatchID := fmt.Sprintf("dispatch-%d", time.Now().Unix())

	// Create dispatch context
	dispatchCtx, cancel := context.WithCancel(ctx)

	// Create dispatch result
	result := &DispatchResult{
		DispatchID:    dispatchID,
		PlanID:        plan.ID,
		Mode:          plan.Mode,
		AgentsSpawned: make([]SpawnedAgent, 0),
		StartTime:     time.Now(),
		Status:        "spawning",
	}

	// Create dispatch state
	state := &dispatchState{
		result:      result,
		agents:      make(map[string]*SpawnedAgent),
		projectPath: "", // Will be set from plan context
		ctx:         dispatchCtx,
		cancel:      cancel,
	}

	// Store dispatch state
	d.mu.Lock()
	d.dispatches[dispatchID] = state
	d.mu.Unlock()

	// Store dispatch in memory DB
	if err := d.storeDispatch(result); err != nil {
		return nil, fmt.Errorf("failed to store dispatch: %w", err)
	}

	// Spawn agents based on recommendations
	go d.spawnAgents(dispatchCtx, plan, state)

	return result, nil
}

// spawnAgents spawns agents according to the plan
func (d *StandardDispatcher) spawnAgents(ctx context.Context, plan *ActionPlan, state *dispatchState) {
	// TODO: Extract project path from plan metadata
	projectPath := "C:\\Users\\Admin\\Documents\\VS Projects\\CLIAIMONITOR"

	for _, rec := range plan.AgentRecommendations {
		select {
		case <-ctx.Done():
			// Dispatch was cancelled
			state.result.Status = "cancelled"
			return
		default:
			agentID, err := d.SpawnAgent(ctx, rec, projectPath)
			if err != nil {
				// Record failure
				spawnedAgent := SpawnedAgent{
					AgentID:   agentID,
					AgentType: rec.AgentType,
					Task:      rec.Task,
					Status:    "failed",
					SpawnedAt: time.Now(),
					Error:     err.Error(),
				}
				state.agents[agentID] = &spawnedAgent
				state.result.AgentsSpawned = append(state.result.AgentsSpawned, spawnedAgent)
				continue
			}

			// Add to spawned agents
			spawnedAgent := SpawnedAgent{
				AgentID:   agentID,
				AgentType: rec.AgentType,
				Task:      rec.Task,
				Status:    "running",
				SpawnedAt: time.Now(),
			}
			state.agents[agentID] = &spawnedAgent
			state.result.AgentsSpawned = append(state.result.AgentsSpawned, spawnedAgent)
		}

		// Small delay between spawns to avoid overwhelming the system
		time.Sleep(2 * time.Second)
	}

	// Update status
	state.result.Status = "running"
}

// SpawnAgent spawns a single agent with the given recommendation
func (d *StandardDispatcher) SpawnAgent(ctx context.Context, rec *AgentRecommendation, projectPath string) (string, error) {
	if rec == nil {
		return "", fmt.Errorf("recommendation is nil")
	}

	// Get agent config
	config, ok := d.configs[rec.AgentType]
	if !ok {
		return "", fmt.Errorf("unknown agent type: %s", rec.AgentType)
	}

	// Generate agent ID
	agentID := d.generateAgentID(config)

	// Build initial prompt with context verification
	initialPrompt := d.buildInitialPrompt(rec, agentID, config.Role)

	// Spawn agent
	_, err := d.spawner.SpawnAgent(config, agentID, projectPath, initialPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to spawn agent: %w", err)
	}

	// Agent registration happens via MCP when the agent connects
	// We just return the agent ID here

	return agentID, nil
}

// GetDispatchStatus retrieves the current status of a dispatch
func (d *StandardDispatcher) GetDispatchStatus(ctx context.Context, dispatchID string) (*DispatchStatus, error) {
	d.mu.RLock()
	state, exists := d.dispatches[dispatchID]
	d.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("dispatch not found: %s", dispatchID)
	}

	// Build status
	status := &DispatchStatus{
		DispatchID:  dispatchID,
		Status:      state.result.Status,
		AgentsTotal: len(state.agents),
		StartTime:   state.result.StartTime,
		LastUpdated: time.Now(),
		Agents:      make([]SpawnedAgent, 0, len(state.agents)),
	}

	// Count agent statuses
	for _, agent := range state.agents {
		status.Agents = append(status.Agents, *agent)

		switch agent.Status {
		case "running":
			status.AgentsRunning++
		case "completed":
			status.AgentsCompleted++
		case "failed":
			status.AgentsFailed++
		}
	}

	return status, nil
}

// AbortDispatch cancels a dispatch and stops all agents
func (d *StandardDispatcher) AbortDispatch(ctx context.Context, dispatchID string) error {
	d.mu.Lock()
	state, exists := d.dispatches[dispatchID]
	if !exists {
		d.mu.Unlock()
		return fmt.Errorf("dispatch not found: %s", dispatchID)
	}

	// Cancel dispatch context
	state.cancel()
	state.result.Status = "aborted"

	d.mu.Unlock()

	// Stop all agents
	for agentID := range state.agents {
		if err := d.spawner.StopAgent(agentID); err != nil {
			// Log error but continue
			fmt.Printf("Failed to stop agent %s: %v\n", agentID, err)
		}
	}

	return nil
}

// ListDispatches retrieves dispatches matching the filter
func (d *StandardDispatcher) ListDispatches(ctx context.Context, filter DispatchFilter) ([]*DispatchSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	summaries := make([]*DispatchSummary, 0)

	for _, state := range d.dispatches {
		// Apply filter
		if filter.Status != "" && state.result.Status != filter.Status {
			continue
		}

		summary := &DispatchSummary{
			DispatchID:  state.result.DispatchID,
			PlanID:      state.result.PlanID,
			Mode:        state.result.Mode,
			Status:      state.result.Status,
			AgentsTotal: len(state.agents),
			StartTime:   state.result.StartTime,
		}

		summaries = append(summaries, summary)
	}

	// Apply limit and offset
	start := 0
	if filter.Offset > 0 && filter.Offset < len(summaries) {
		start = filter.Offset
	}

	end := len(summaries)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}

	if start >= len(summaries) {
		return []*DispatchSummary{}, nil
	}

	return summaries[start:end], nil
}

// Helper functions

func (d *StandardDispatcher) generateAgentID(config types.AgentConfig) string {
	if config.Numbering && config.Prefix != "" {
		// Use counter-based numbering (simplified version)
		// In production, this would use the agent counter from state
		timestamp := time.Now().Unix() % 1000
		return fmt.Sprintf("%s%03d", config.Prefix, timestamp)
	}
	return fmt.Sprintf("%s-%d", config.Name, time.Now().Unix())
}

func (d *StandardDispatcher) buildInitialPrompt(rec *AgentRecommendation, agentID string, role types.AgentRole) string {
	// Build prompt with agent identity and registration
	prompt := fmt.Sprintf(
		"You are agent '%s' with role '%s'. Call mcp__cliaimonitor__register_agent with agent_id='%s' and role='%s' to register. ",
		agentID, role, agentID, role)

	prompt += fmt.Sprintf("TASK: %s. ", rec.Task)

	if rec.Rationale != "" {
		prompt += fmt.Sprintf("Rationale: %s. ", rec.Rationale)
	}

	if len(rec.FindingIDs) > 0 {
		prompt += fmt.Sprintf("Addresses findings: %v. ", rec.FindingIDs)
	}

	prompt += "Work autonomously. Use MCP tools to report progress."

	return prompt
}

func (d *StandardDispatcher) storeDispatch(result *DispatchResult) error {
	// Store dispatch metadata in memory DB
	// This would use a proper storage mechanism in production
	return nil
}
