package supervisor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/CLIAIMONITOR/internal/memory"
)

// Planner generates deployment strategies for repositories
type Planner struct {
	memDB memory.MemoryDB
}

// NewPlanner creates a new deployment planner
func NewPlanner(memDB memory.MemoryDB) *Planner {
	return &Planner{
		memDB: memDB,
	}
}

// DeploymentPlan represents a proposed deployment strategy
type DeploymentPlan struct {
	RepoID         string                 `json:"repo_id"`
	Strategy       string                 `json:"strategy"`       // 'sequential', 'parallel', 'phased'
	TotalTasks     int                    `json:"total_tasks"`
	Analysis       TaskAnalysis           `json:"analysis"`
	AgentProposals []AgentProposal        `json:"agent_proposals"`
	Rationale      string                 `json:"rationale"`
	EstimatedTime  string                 `json:"estimated_time"`
	Risks          []string               `json:"risks"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// TaskAnalysis contains analysis of tasks
type TaskAnalysis struct {
	TotalTasks       int            `json:"total_tasks"`
	PendingTasks     int            `json:"pending_tasks"`
	PriorityBreakdown map[string]int `json:"priority_breakdown"`
	CategoryBreakdown map[string]int `json:"category_breakdown"`
	ComplexityScore  int            `json:"complexity_score"`
	Dependencies     []string       `json:"dependencies"`
}

// AgentProposal represents a proposed agent to spawn
type AgentProposal struct {
	Role           string   `json:"role"`             // 'coder', 'tester', 'reviewer'
	ConfigName     string   `json:"config_name"`
	TaskIDs        []string `json:"task_ids"`
	Justification  string   `json:"justification"`
	Priority       int      `json:"priority"`         // 1-5, higher = spawn first
	EstimatedTasks int      `json:"estimated_tasks"`
}

// AnalyzeTasks analyzes tasks for a repository
func (p *Planner) AnalyzeTasks(repoID string) (*TaskAnalysis, error) {
	// Get all tasks for this repo
	tasks, err := p.memDB.GetTasks(memory.TaskFilter{
		RepoID: repoID,
		Limit:  1000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks: %w", err)
	}

	analysis := &TaskAnalysis{
		TotalTasks:        len(tasks),
		PriorityBreakdown: make(map[string]int),
		CategoryBreakdown: make(map[string]int),
		Dependencies:      []string{},
	}

	pendingCount := 0
	complexityScore := 0

	for _, task := range tasks {
		// Count by status
		if task.Status == "pending" {
			pendingCount++
		}

		// Count by priority
		analysis.PriorityBreakdown[task.Priority]++

		// Calculate complexity score
		switch task.Priority {
		case "critical":
			complexityScore += 4
		case "high":
			complexityScore += 3
		case "medium":
			complexityScore += 2
		case "low":
			complexityScore += 1
		}

		// Categorize by task characteristics
		category := categorizeTask(task)
		analysis.CategoryBreakdown[category]++

		// Track dependencies
		if task.ParentTaskID != "" {
			analysis.Dependencies = append(analysis.Dependencies, task.ParentTaskID)
		}
	}

	analysis.PendingTasks = pendingCount
	analysis.ComplexityScore = complexityScore

	return analysis, nil
}

// ProposeAgents generates agent proposals based on analysis
func (p *Planner) ProposeAgents(repoID string, analysis *TaskAnalysis) ([]AgentProposal, error) {
	var proposals []AgentProposal

	// Get pending tasks to assign
	tasks, err := p.memDB.GetTasks(memory.TaskFilter{
		RepoID: repoID,
		Status: "pending",
		Limit:  1000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get pending tasks: %w", err)
	}

	// Group tasks by category
	tasksByCategory := make(map[string][]*memory.WorkflowTask)
	for _, task := range tasks {
		category := categorizeTask(task)
		tasksByCategory[category] = append(tasksByCategory[category], task)
	}

	// Propose agents based on task categories

	// 1. Coder agents for implementation tasks
	if coderTasks, ok := tasksByCategory["implementation"]; ok && len(coderTasks) > 0 {
		proposals = append(proposals, AgentProposal{
			Role:           "coder",
			ConfigName:     "coder",
			TaskIDs:        extractTaskIDs(coderTasks),
			Justification:  fmt.Sprintf("Found %d implementation tasks requiring code development", len(coderTasks)),
			Priority:       5,
			EstimatedTasks: len(coderTasks),
		})
	}

	// 2. Tester agents for testing tasks
	if testTasks, ok := tasksByCategory["testing"]; ok && len(testTasks) > 0 {
		proposals = append(proposals, AgentProposal{
			Role:           "tester",
			ConfigName:     "tester",
			TaskIDs:        extractTaskIDs(testTasks),
			Justification:  fmt.Sprintf("Found %d testing tasks requiring validation and QA", len(testTasks)),
			Priority:       4,
			EstimatedTasks: len(testTasks),
		})
	}

	// 3. Reviewer agents if many tasks or high complexity
	if analysis.ComplexityScore > 20 || analysis.TotalTasks > 10 {
		proposals = append(proposals, AgentProposal{
			Role:           "reviewer",
			ConfigName:     "reviewer",
			TaskIDs:        []string{},
			Justification:  fmt.Sprintf("High complexity (score: %d) or task count (%d) suggests need for code review", analysis.ComplexityScore, analysis.TotalTasks),
			Priority:       3,
			EstimatedTasks: 0, // Reviewer doesn't have assigned tasks
		})
	}

	// 4. Additional coders for large workload
	// Task distribution strategy
	// Current method uses a naive half-split approach for load distribution
	// Limitations in current implementation:
	// - Ignores task complexity and interdependencies
	// - Assumes uniform task difficulty
	// - Cannot handle nuanced task allocation requirements
	//
	// Ideal future improvements:
	// 1. Complexity-weighted task distribution
	// 2. Dependency graph-based task allocation
	// 3. Dynamic load balancing considering agent capabilities
	// 4. Machine learning driven task distribution
	//
	// Current approach provides a simple, deterministic baseline
	if len(tasksByCategory["implementation"]) > 10 {
		// Split tasks for multiple coders
		allCoderTasks := tasksByCategory["implementation"]
		half := len(allCoderTasks) / 2

		proposals = append(proposals, AgentProposal{
			Role:           "coder",
			ConfigName:     "coder",
			TaskIDs:        extractTaskIDs(allCoderTasks[half:]),
			Justification:  fmt.Sprintf("Large workload (%d tasks) benefits from parallel development", len(allCoderTasks)),
			Priority:       4,
			EstimatedTasks: len(allCoderTasks) - half,
		})
	}

	// 5. Bug fix specialists for bug-related tasks
	if bugTasks, ok := tasksByCategory["bugfix"]; ok && len(bugTasks) > 0 {
		proposals = append(proposals, AgentProposal{
			Role:           "coder",
			ConfigName:     "coder",
			TaskIDs:        extractTaskIDs(bugTasks),
			Justification:  fmt.Sprintf("Found %d bug fix tasks requiring focused debugging", len(bugTasks)),
			Priority:       5,
			EstimatedTasks: len(bugTasks),
		})
	}

	return proposals, nil
}

// CreateDeploymentPlan generates a full deployment plan
func (p *Planner) CreateDeploymentPlan(repoID string) (*DeploymentPlan, error) {
	// Analyze tasks
	analysis, err := p.AnalyzeTasks(repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze tasks: %w", err)
	}

	// Propose agents
	proposals, err := p.ProposeAgents(repoID, analysis)
	if err != nil {
		return nil, fmt.Errorf("failed to propose agents: %w", err)
	}

	// Determine strategy
	strategy := determineStrategy(analysis, proposals)

	// Build deployment plan
	plan := &DeploymentPlan{
		RepoID:         repoID,
		Strategy:       strategy,
		TotalTasks:     analysis.TotalTasks,
		Analysis:       *analysis,
		AgentProposals: proposals,
		Rationale:      generateRationale(analysis, proposals, strategy),
		EstimatedTime:  estimateTime(analysis),
		Risks:          identifyRisks(analysis),
		Metadata: map[string]interface{}{
			"generated_by": "planner",
			"version":      "1.0",
		},
	}

	return plan, nil
}

// StoreDeploymentPlan saves a deployment plan to memory.db
func (p *Planner) StoreDeploymentPlan(plan *DeploymentPlan) (int64, error) {
	// Serialize plan to JSON
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return 0, fmt.Errorf("failed to serialize plan: %w", err)
	}

	// Serialize agent configs
	agentConfigsJSON, err := json.Marshal(plan.AgentProposals)
	if err != nil {
		return 0, fmt.Errorf("failed to serialize agent configs: %w", err)
	}

	deployment := &memory.Deployment{
		RepoID:         plan.RepoID,
		DeploymentPlan: string(planJSON),
		Status:         "proposed",
		AgentConfigs:   string(agentConfigsJSON),
	}

	if err := p.memDB.CreateDeployment(deployment); err != nil {
		return 0, fmt.Errorf("failed to create deployment: %w", err)
	}

	return deployment.ID, nil
}

// Helper functions

func categorizeTask(task *memory.WorkflowTask) string {
	title := strings.ToLower(task.Title)
	description := strings.ToLower(task.Description)
	combined := title + " " + description

	// Bug fixes
	if strings.Contains(combined, "fix") || strings.Contains(combined, "bug") {
		return "bugfix"
	}

	// Testing
	if strings.Contains(combined, "test") || strings.Contains(combined, "qa") {
		return "testing"
	}

	// Documentation
	if strings.Contains(combined, "doc") || strings.Contains(combined, "readme") {
		return "documentation"
	}

	// Refactoring
	if strings.Contains(combined, "refactor") || strings.Contains(combined, "cleanup") {
		return "refactoring"
	}

	// Default to implementation
	return "implementation"
}

func extractTaskIDs(tasks []*memory.WorkflowTask) []string {
	ids := make([]string, len(tasks))
	for i, task := range tasks {
		ids[i] = task.ID
	}
	return ids
}

func determineStrategy(analysis *TaskAnalysis, proposals []AgentProposal) string {
	// If many dependencies, use sequential
	// Use multiplication to avoid integer division edge case
	// (e.g., 5 deps in 11 tasks: 5 > 5 is false, but 5 > 5.5 should be false)
	if len(analysis.Dependencies)*2 > analysis.TotalTasks {
		return "sequential"
	}

	// If high complexity but manageable, use phased
	if analysis.ComplexityScore > 30 {
		return "phased"
	}

	// Default to parallel for efficiency
	return "parallel"
}

func generateRationale(analysis *TaskAnalysis, proposals []AgentProposal, strategy string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Analyzed %d tasks with complexity score of %d", analysis.TotalTasks, analysis.ComplexityScore))
	parts = append(parts, fmt.Sprintf("Proposing %d agents using %s strategy", len(proposals), strategy))

	if analysis.PendingTasks > 0 {
		parts = append(parts, fmt.Sprintf("%d tasks are pending and ready for assignment", analysis.PendingTasks))
	}

	return strings.Join(parts, ". ") + "."
}

// EstimateTime provides a rough time estimate based on complexity score.
// Formula: hours = complexity_score / 2, with minimum of 2 hours
// Calibration notes:
// - ComplexityScore of 4 = "1-2 hours" (small task)
// - ComplexityScore of 10 = "4-8 hours" (medium task)
// - ComplexityScore of 20 = "8-16 hours" (medium-large task)
// - ComplexityScore of 40+ = "1-2 weeks" (large task)
// This is a heuristic and should be refined based on actual project data.
func estimateTime(analysis *TaskAnalysis) string {
	// Simple heuristic: complexity score maps to hours
	hours := analysis.ComplexityScore / 2

	if hours < 2 {
		return "1-2 hours"
	} else if hours < 8 {
		return "2-8 hours"
	} else if hours < 24 {
		return "1-3 days"
	} else if hours < 80 {
		return "1-2 weeks"
	}

	return "2+ weeks"
}

func identifyRisks(analysis *TaskAnalysis) []string {
	risks := []string{}

	if analysis.ComplexityScore > 50 {
		risks = append(risks, "High complexity may require extended development time")
	}

	if len(analysis.Dependencies) > 5 {
		risks = append(risks, "Many task dependencies could create bottlenecks")
	}

	if analysis.PriorityBreakdown["critical"] > 3 {
		risks = append(risks, "Multiple critical priority tasks require careful coordination")
	}

	if len(risks) == 0 {
		risks = append(risks, "No significant risks identified")
	}

	return risks
}
