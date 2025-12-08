// internal/metrics/extended.go
package metrics

import (
	"sync"
	"time"
)

// HealthStatus represents agent health
type HealthStatus string

const (
	HealthHealthy HealthStatus = "healthy"
	HealthIdle    HealthStatus = "idle"
	HealthStuck   HealthStatus = "stuck"
	HealthFailing HealthStatus = "failing"
	HealthError   HealthStatus = "error"
)

// ExtendedAgentMetrics provides comprehensive agent metrics
type ExtendedAgentMetrics struct {
	AgentID     string `json:"agent_id"`
	AgentType   string `json:"agent_type"`

	// Efficiency metrics
	TasksCompleted   int   `json:"tasks_completed"`
	TotalTokens      int64 `json:"total_tokens"`
	TotalTimeSeconds int64 `json:"total_time_seconds"`

	// Progress metrics
	CurrentTaskID    string `json:"current_task_id,omitempty"`
	QueueDepth       int    `json:"queue_depth"`

	// Health metrics
	LastActivity        time.Time `json:"last_activity"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	FailedTests         int       `json:"failed_tests"`
	ReviewRejections    int       `json:"review_rejections"`
}

// TokensPerTask returns average tokens per completed task
func (m *ExtendedAgentMetrics) TokensPerTask() int64 {
	if m.TasksCompleted == 0 {
		return 0
	}
	return m.TotalTokens / int64(m.TasksCompleted)
}

// AvgTaskTimeSeconds returns average time per task in seconds
func (m *ExtendedAgentMetrics) AvgTaskTimeSeconds() int64 {
	if m.TasksCompleted == 0 {
		return 0
	}
	return m.TotalTimeSeconds / int64(m.TasksCompleted)
}

// HealthStatus returns the agent's health status
func (m *ExtendedAgentMetrics) HealthStatus() HealthStatus {
	if m.ConsecutiveFailures >= 3 {
		return HealthFailing
	}

	idleTime := time.Since(m.LastActivity)

	if idleTime > 30*time.Minute {
		return HealthStuck
	}
	if idleTime > 10*time.Minute {
		return HealthIdle
	}

	return HealthHealthy
}

// TeamMetrics aggregates metrics across all agents
type TeamMetrics struct {
	mu       sync.RWMutex
	TeamID   string                           `json:"team_id"`
	Agents   map[string]*ExtendedAgentMetrics `json:"agents"`
}

// NewTeamMetrics creates a new team metrics tracker
func NewTeamMetrics(teamID string) *TeamMetrics {
	return &TeamMetrics{
		TeamID: teamID,
		Agents: make(map[string]*ExtendedAgentMetrics),
	}
}

// AddAgentMetrics adds or updates metrics for an agent
func (t *TeamMetrics) AddAgentMetrics(agentID string, m *ExtendedAgentMetrics) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Agents[agentID] = m
}

// TotalTasks returns total tasks completed across all agents
func (t *TeamMetrics) TotalTasks() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	total := 0
	for _, m := range t.Agents {
		total += m.TasksCompleted
	}
	return total
}

// TotalTokens returns total tokens used across all agents
func (t *TeamMetrics) TotalTokens() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var total int64
	for _, m := range t.Agents {
		total += m.TotalTokens
	}
	return total
}

// ActiveAgents returns count of agents with healthy/idle status
func (t *TeamMetrics) ActiveAgents() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	count := 0
	for _, m := range t.Agents {
		status := m.HealthStatus()
		if status == HealthHealthy || status == HealthIdle {
			count++
		}
	}
	return count
}

// EstimatedCost calculates total cost based on model pricing
func (t *TeamMetrics) EstimatedCost() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var cost float64
	for _, m := range t.Agents {
		// Opus: $15/M input, Sonnet: $3/M input (simplified)
		rate := 0.003 // Default Sonnet rate
		if m.AgentType == "OpusGreen" || m.AgentType == "OpusPurple" || m.AgentType == "OpusRed" {
			rate = 0.015
		}
		cost += float64(m.TotalTokens) * rate / 1000000
	}
	return cost
}
