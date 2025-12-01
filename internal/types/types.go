package types

import (
	"fmt"
	"time"
)

// AgentStatus represents the current status of an agent
type AgentStatus string

const (
	StatusStarting     AgentStatus = "starting"
	StatusConnected    AgentStatus = "connected"
	StatusWorking      AgentStatus = "working"
	StatusIdle         AgentStatus = "idle"
	StatusBlocked      AgentStatus = "blocked"
	StatusDisconnected AgentStatus = "disconnected"
)

// AgentRole defines the role/specialization of an agent
type AgentRole string

const (
	RoleGoDeveloper AgentRole = "Go Developer"
	RoleCodeAuditor AgentRole = "Code Auditor"
	RoleEngineer    AgentRole = "Engineer"
	RoleSecurity    AgentRole = "Security"
	RoleSupervisor  AgentRole = "Supervisor"
)

// AgentConfig from teams.yaml
type AgentConfig struct {
	Name  string    `yaml:"name" json:"name"`
	Model string    `yaml:"model" json:"model"`
	Role  AgentRole `yaml:"role" json:"role"`
	Color string    `yaml:"color" json:"color"`
}

// Agent represents a running agent instance
type Agent struct {
	ID          string      `json:"id"`
	ConfigName  string      `json:"config_name"`
	Role        AgentRole   `json:"role"`
	Model       string      `json:"model"`
	Color       string      `json:"color"`
	Status      AgentStatus `json:"status"`
	PID         int         `json:"pid"`
	ProjectPath string      `json:"project_path"`
	SpawnedAt   time.Time   `json:"spawned_at"`
	LastSeen    time.Time   `json:"last_seen"`
	CurrentTask string      `json:"current_task"`
}

// AgentMetrics tracks per-agent statistics
type AgentMetrics struct {
	AgentID            string    `json:"agent_id"`
	TokensUsed         int64     `json:"tokens_used"`
	EstimatedCost      float64   `json:"estimated_cost"`
	FailedTests        int       `json:"failed_tests"`
	ConsecutiveRejects int       `json:"consecutive_rejects"`
	IdleSince          time.Time `json:"idle_since"`
	LastUpdated        time.Time `json:"last_updated"`
}

// AlertThresholds configurable via dashboard
type AlertThresholds struct {
	FailedTestsMax        int   `json:"failed_tests_max"`
	IdleTimeMaxSeconds    int   `json:"idle_time_max_seconds"`
	EscalationQueueMax    int   `json:"escalation_queue_max"`
	TokenUsageMax         int64 `json:"token_usage_max"`
	ConsecutiveRejectsMax int   `json:"consecutive_rejects_max"`
}

// DefaultThresholds returns sensible defaults
func DefaultThresholds() AlertThresholds {
	return AlertThresholds{
		FailedTestsMax:        5,
		IdleTimeMaxSeconds:    600, // 10 minutes
		EscalationQueueMax:    10,
		TokenUsageMax:         100000,
		ConsecutiveRejectsMax: 3,
	}
}

// Validate checks that all threshold values are positive
func (t AlertThresholds) Validate() error {
	if t.FailedTestsMax < 1 {
		return fmt.Errorf("failed_tests_max must be at least 1")
	}
	if t.IdleTimeMaxSeconds < 60 {
		return fmt.Errorf("idle_time_max_seconds must be at least 60")
	}
	if t.EscalationQueueMax < 1 {
		return fmt.Errorf("escalation_queue_max must be at least 1")
	}
	if t.TokenUsageMax < 1000 {
		return fmt.Errorf("token_usage_max must be at least 1000")
	}
	if t.ConsecutiveRejectsMax < 1 {
		return fmt.Errorf("consecutive_rejects_max must be at least 1")
	}
	return nil
}

// HumanInputRequest when agent needs human answer
type HumanInputRequest struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Question  string    `json:"question"`
	Context   string    `json:"context"`
	CreatedAt time.Time `json:"created_at"`
	Answered  bool      `json:"answered"`
	Answer    string    `json:"answer"`
}

// Alert for dashboard notification
type Alert struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	AgentID      string    `json:"agent_id"`
	Message      string    `json:"message"`
	Severity     string    `json:"severity"` // "warning", "critical"
	CreatedAt    time.Time `json:"created_at"`
	Acknowledged bool      `json:"acknowledged"`
}

// ActivityLog entry
type ActivityLog struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
}

// SupervisorJudgment records supervisor decisions
type SupervisorJudgment struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Issue     string    `json:"issue"`
	Decision  string    `json:"decision"`
	Reasoning string    `json:"reasoning"`
	Action    string    `json:"action"` // "restart", "pause", "escalate", "continue"
	Timestamp time.Time `json:"timestamp"`
}

// StopApprovalRequest when an agent wants to stop working
type StopApprovalRequest struct {
	ID            string    `json:"id"`
	AgentID       string    `json:"agent_id"`
	Reason        string    `json:"reason"`        // "task_complete", "blocked", "error", "needs_input", "other"
	Context       string    `json:"context"`       // Details about why they want to stop
	WorkCompleted string    `json:"work_completed"` // Summary of what was accomplished
	CreatedAt     time.Time `json:"created_at"`
	Reviewed      bool      `json:"reviewed"`
	Approved      bool      `json:"approved"`
	Response      string    `json:"response"` // Supervisor's response message
	ReviewedBy    string    `json:"reviewed_by"` // "supervisor" or "human"
}

// MetricsSnapshot for history
type MetricsSnapshot struct {
	Timestamp time.Time                `json:"timestamp"`
	Agents    map[string]*AgentMetrics `json:"agents"`
}

// DashboardState is the full persisted state
type DashboardState struct {
	SupervisorConnected bool                            `json:"supervisor_connected"`
	Agents              map[string]*Agent               `json:"agents"`
	Metrics             map[string]*AgentMetrics        `json:"metrics"`
	MetricsHistory      []MetricsSnapshot               `json:"metrics_history"`
	HumanRequests       map[string]*HumanInputRequest   `json:"human_requests"`
	StopRequests        map[string]*StopApprovalRequest `json:"stop_requests"`
	Alerts              []*Alert                        `json:"alerts"`
	ActivityLog         []*ActivityLog                  `json:"activity_log"`
	Judgments           []*SupervisorJudgment           `json:"judgments"`
	Thresholds          AlertThresholds                 `json:"thresholds"`
	LastHumanCheckin    time.Time                       `json:"last_human_checkin"`
	AgentCounters       map[string]int                  `json:"agent_counters"`
}

// NewDashboardState creates empty state with defaults
func NewDashboardState() *DashboardState {
	return &DashboardState{
		SupervisorConnected: false,
		Agents:              make(map[string]*Agent),
		Metrics:             make(map[string]*AgentMetrics),
		MetricsHistory:      []MetricsSnapshot{},
		HumanRequests:       make(map[string]*HumanInputRequest),
		StopRequests:        make(map[string]*StopApprovalRequest),
		Alerts:              []*Alert{},
		ActivityLog:         []*ActivityLog{},
		Judgments:           []*SupervisorJudgment{},
		Thresholds:          DefaultThresholds(),
		LastHumanCheckin:    time.Now(),
		AgentCounters:       make(map[string]int),
	}
}
