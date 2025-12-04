package nats

import "time"

// Subject pattern constants for NATS messaging
const (
	// SubjectAgentHeartbeat is the pattern for agent heartbeat messages
	// Use fmt.Sprintf(SubjectAgentHeartbeat, agentID) to create specific subjects
	SubjectAgentHeartbeat = "agent.%s.heartbeat"

	// SubjectAgentStatus is the pattern for agent status updates
	SubjectAgentStatus = "agent.%s.status"

	// SubjectAgentCommand is the pattern for commands sent to specific agents
	SubjectAgentCommand = "agent.%s.command"

	// SubjectAgentShutdown is the pattern for agent shutdown requests
	SubjectAgentShutdown = "agent.%s.shutdown"

	// SubjectAllHeartbeats subscribes to all agent heartbeats
	SubjectAllHeartbeats = "agent.*.heartbeat"

	// SubjectAllStatus subscribes to all agent status updates
	SubjectAllStatus = "agent.*.status"

	// SubjectToolCall is used for tool execution requests
	SubjectToolCall = "tools.call"

	// SubjectCaptainDecision is used for captain decision broadcasts
	SubjectCaptainDecision = "captain.decision"

	// SubjectDashboardState is used for dashboard state updates
	SubjectDashboardState = "dashboard.state"

	// SubjectDashboardAlert is used for dashboard alert messages
	SubjectDashboardAlert = "dashboard.alert"
)

// HeartbeatMessage represents an agent heartbeat message
type HeartbeatMessage struct {
	AgentID     string    `json:"agent_id"`
	ConfigName  string    `json:"config_name"`
	ProjectPath string    `json:"project_path"`
	Status      string    `json:"status"`
	CurrentTask string    `json:"current_task"`
	Timestamp   time.Time `json:"timestamp"`
}

// StatusMessage represents an agent status update
type StatusMessage struct {
	AgentID   string    `json:"agent_id"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// CommandMessage represents a command sent to an agent
type CommandMessage struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// ShutdownMessage represents a shutdown request or notification
type ShutdownMessage struct {
	Reason   string `json:"reason"`
	Approved bool   `json:"approved"`
	Force    bool   `json:"force"`
}

// ToolCallRequest represents a request to execute a tool
type ToolCallRequest struct {
	RequestID string                 `json:"request_id"`
	AgentID   string                 `json:"agent_id"`
	Tool      string                 `json:"tool"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolCallResponse represents the response from a tool execution
type ToolCallResponse struct {
	RequestID string      `json:"request_id"`
	Success   bool        `json:"success"`
	Result    interface{} `json:"result"`
	Error     string      `json:"error,omitempty"`
}

// StopApprovalRequest represents a request from an agent to stop
type StopApprovalRequest struct {
	AgentID       string `json:"agent_id"`
	Reason        string `json:"reason"`
	Context       string `json:"context"`
	WorkCompleted bool   `json:"work_completed"`
}

// StopApprovalResponse represents the captain's decision on a stop request
type StopApprovalResponse struct {
	Approved bool   `json:"approved"`
	Message  string `json:"message"`
}
