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

	// SubjectCaptainStatus is used for Captain status updates
	SubjectCaptainStatus = "captain.status"

	// SubjectCaptainCommands is used for dashboard commands to Captain
	SubjectCaptainCommands = "captain.commands"

	// SubjectSystemBroadcast is used for system-wide announcements
	SubjectSystemBroadcast = "system.broadcast"

	// SubjectEscalationCreate is used when agents raise questions
	SubjectEscalationCreate = "escalation.create"

	// SubjectEscalationForward is used when Captain forwards to human
	SubjectEscalationForward = "escalation.forward"

	// SubjectEscalationResponse is the pattern for human's answer
	// Use fmt.Sprintf(SubjectEscalationResponse, escalationID) to create specific subjects
	SubjectEscalationResponse = "escalation.response.%s"
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

// ClientInfo represents a connected NATS client
type ClientInfo struct {
	ClientID    string    `json:"client_id"`
	ConnectedAt time.Time `json:"connected_at"`
}

// CaptainStatusMessage represents Captain's status update
type CaptainStatusMessage struct {
	Status    string    `json:"status"` // idle, busy, error
	CurrentOp string    `json:"current_op,omitempty"`
	QueueSize int       `json:"queue_size"`
	Timestamp time.Time `json:"timestamp"`
}

// CaptainCommandMessage represents commands sent to Captain from dashboard
type CaptainCommandMessage struct {
	Type    string                 `json:"type"` // spawn_agent, kill_agent, submit_task, pause, resume
	Payload map[string]interface{} `json:"payload"`
	From    string                 `json:"from"` // client ID of sender
}

// EscalationCreateMessage represents an agent raising a question
type EscalationCreateMessage struct {
	ID        string                 `json:"id"`
	AgentID   string                 `json:"agent_id"`
	Question  string                 `json:"question"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// EscalationForwardMessage represents Captain forwarding escalation to human
type EscalationForwardMessage struct {
	ID                string                 `json:"id"`
	AgentID           string                 `json:"agent_id"`
	Question          string                 `json:"question"`
	CaptainContext    string                 `json:"captain_context,omitempty"`
	CaptainRecommends string                 `json:"captain_recommends,omitempty"`
	Timestamp         time.Time              `json:"timestamp"`
}

// EscalationResponseMessage represents human's answer to escalation
type EscalationResponseMessage struct {
	ID        string    `json:"id"`
	Response  string    `json:"response"`
	From      string    `json:"from"` // "human" or client ID
	Timestamp time.Time `json:"timestamp"`
}

// SystemBroadcastMessage represents system-wide announcements
type SystemBroadcastMessage struct {
	Type      string                 `json:"type"` // shutdown, agent_killed, config_change
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}
