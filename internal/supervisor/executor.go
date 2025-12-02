package supervisor

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/CLIAIMONITOR/internal/agents"
	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/types"
)

// Executor bridges deployment plans to agent spawning
type Executor struct {
	memDB   memory.MemoryDB
	spawner *agents.ProcessSpawner
	configs map[string]types.AgentConfig
}

// NewExecutor creates a new deployment executor
func NewExecutor(memDB memory.MemoryDB, spawner *agents.ProcessSpawner, configs map[string]types.AgentConfig) *Executor {
	return &Executor{
		memDB:   memDB,
		spawner: spawner,
		configs: configs,
	}
}

// ExecutionResult contains results of plan execution
type ExecutionResult struct {
	DeploymentID  int64            `json:"deployment_id"`
	SpawnedAgents []SpawnedAgent   `json:"spawned_agents"`
	FailedAgents  []FailedAgent    `json:"failed_agents"`
	TasksAssigned int              `json:"tasks_assigned"`
	Status        string           `json:"status"`
}

// SpawnedAgent represents a successfully spawned agent
type SpawnedAgent struct {
	AgentID    string   `json:"agent_id"`
	ConfigName string   `json:"config_name"`
	Role       string   `json:"role"`
	TaskIDs    []string `json:"task_ids"`
	PID        int      `json:"pid"`
}

// FailedAgent represents a failed agent spawn attempt
type FailedAgent struct {
	ConfigName string `json:"config_name"`
	Role       string `json:"role"`
	Error      string `json:"error"`
}

// ExecutePlan spawns agents based on a deployment plan
func (e *Executor) ExecutePlan(deploymentID int64) (*ExecutionResult, error) {
	// Get deployment from DB
	deployment, err := e.memDB.GetDeployment(deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	if deployment.Status != "proposed" && deployment.Status != "approved" {
		return nil, fmt.Errorf("deployment status must be 'proposed' or 'approved', got '%s'", deployment.Status)
	}

	// Parse agent proposals from the deployment
	var proposals []AgentProposal
	if err := json.Unmarshal([]byte(deployment.AgentConfigs), &proposals); err != nil {
		return nil, fmt.Errorf("failed to parse agent configs: %w", err)
	}

	// Sort by priority (highest first)
	sort.Slice(proposals, func(i, j int) bool {
		return proposals[i].Priority > proposals[j].Priority
	})

	result := &ExecutionResult{
		DeploymentID:  deploymentID,
		SpawnedAgents: []SpawnedAgent{},
		FailedAgents:  []FailedAgent{},
	}

	// Update deployment status to executing
	if err := e.memDB.UpdateDeploymentStatus(deploymentID, "executing"); err != nil {
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Get repository info for project path
	repo, err := e.memDB.GetRepo(deployment.RepoID)
	if err != nil {
		e.memDB.UpdateDeploymentStatus(deploymentID, "failed")
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	// Spawn agents from proposals
	for _, proposal := range proposals {
		spawned, err := e.spawnFromProposal(proposal, repo.BasePath)
		if err != nil {
			result.FailedAgents = append(result.FailedAgents, FailedAgent{
				ConfigName: proposal.ConfigName,
				Role:       proposal.Role,
				Error:      err.Error(),
			})
			continue
		}

		result.SpawnedAgents = append(result.SpawnedAgents, *spawned)

		// Assign tasks to agent
		for _, taskID := range proposal.TaskIDs {
			if err := e.memDB.UpdateTaskStatus(taskID, "assigned", spawned.AgentID); err == nil {
				result.TasksAssigned++
			}
		}
	}

	// Update final status
	if len(result.FailedAgents) > 0 && len(result.SpawnedAgents) == 0 {
		result.Status = "failed"
		e.memDB.UpdateDeploymentStatus(deploymentID, "failed")
	} else if len(result.FailedAgents) > 0 {
		result.Status = "partial"
		e.memDB.UpdateDeploymentStatus(deploymentID, "completed")
	} else {
		result.Status = "completed"
		e.memDB.UpdateDeploymentStatus(deploymentID, "completed")
	}

	return result, nil
}

// spawnFromProposal converts an AgentProposal to an actual agent spawn
func (e *Executor) spawnFromProposal(proposal AgentProposal, projectPath string) (*SpawnedAgent, error) {
	// Map proposal role to agent config
	config, agentID, err := e.resolveConfig(proposal)
	if err != nil {
		return nil, err
	}

	// Build initial prompt describing the tasks
	initialPrompt := buildInitialPrompt(proposal)

	// Spawn the agent using ProcessSpawner
	pid, err := e.spawner.SpawnAgent(config, agentID, projectPath, initialPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn agent: %w", err)
	}

	return &SpawnedAgent{
		AgentID:    agentID,
		ConfigName: proposal.ConfigName,
		Role:       proposal.Role,
		TaskIDs:    proposal.TaskIDs,
		PID:        pid,
	}, nil
}

// resolveConfig maps a proposal to an actual agent config
func (e *Executor) resolveConfig(proposal AgentProposal) (types.AgentConfig, string, error) {
	// First try exact match by config name
	if config, ok := e.configs[proposal.ConfigName]; ok {
		agentID := fmt.Sprintf("%s-%03d", proposal.ConfigName, generateSequenceNum())
		return config, agentID, nil
	}

	// Map role to default config
	roleMapping := map[string]string{
		"coder":    "SNTGreen",  // Go Developer
		"tester":   "SNTPurple", // Code Auditor (can do testing)
		"reviewer": "SNTPurple", // Code Auditor
	}

	if configName, ok := roleMapping[proposal.Role]; ok {
		if config, exists := e.configs[configName]; exists {
			agentID := fmt.Sprintf("%s-%03d", configName, generateSequenceNum())
			return config, agentID, nil
		}
	}

	// Fallback to first available config
	for name, config := range e.configs {
		agentID := fmt.Sprintf("%s-%03d", name, generateSequenceNum())
		return config, agentID, nil
	}

	return types.AgentConfig{}, "", fmt.Errorf("no agent config available for role '%s'", proposal.Role)
}

// buildInitialPrompt creates a task-specific prompt for the agent
func buildInitialPrompt(proposal AgentProposal) string {
	if len(proposal.TaskIDs) == 0 {
		return fmt.Sprintf("You are a %s agent. %s Work autonomously.", proposal.Role, proposal.Justification)
	}

	return fmt.Sprintf(
		"You are a %s agent. You have been assigned %d tasks: %v. %s Work autonomously on these tasks. "+
			"Use MCP tools to report your status and complete tasks.",
		proposal.Role,
		len(proposal.TaskIDs),
		proposal.TaskIDs,
		proposal.Justification,
	)
}

// Simple sequence number generator for unique agent IDs
var sequenceCounter int

func generateSequenceNum() int {
	sequenceCounter++
	return sequenceCounter
}
