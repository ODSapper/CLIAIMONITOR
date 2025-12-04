package router

import (
	"sync"
	"time"

	"github.com/CLIAIMONITOR/internal/memory"
)

// AgentComms handles agent communication, replacing PowerShell heartbeat scripts
type AgentComms struct {
	memDB      memory.MemoryDB
	router     *SkillRouter
	shutdownMu sync.RWMutex
	shutdowns  map[string]chan struct{} // agent ID -> shutdown signal
}

// NewAgentComms creates a new agent communication handler
func NewAgentComms(memDB memory.MemoryDB) *AgentComms {
	return &AgentComms{
		memDB:     memDB,
		router:    NewSkillRouter(memDB),
		shutdowns: make(map[string]chan struct{}),
	}
}

// HeartbeatRequest represents an agent heartbeat
type HeartbeatRequest struct {
	AgentID     string `json:"agent_id"`
	ConfigName  string `json:"config_name"`
	ProjectPath string `json:"project_path"`
	Status      string `json:"status"`
	CurrentTask string `json:"current_task"`
}

// HeartbeatResponse is returned to the agent
type HeartbeatResponse struct {
	OK            bool      `json:"ok"`
	Timestamp     time.Time `json:"timestamp"`
	ShouldStop    bool      `json:"should_stop"`
	StopReason    string    `json:"stop_reason,omitempty"`
	HasMessages   bool      `json:"has_messages"`
	MessageCount  int       `json:"message_count,omitempty"`
}

// ProcessHeartbeat handles an agent heartbeat and returns instructions
func (c *AgentComms) ProcessHeartbeat(req *HeartbeatRequest) (*HeartbeatResponse, error) {
	// Update agent status
	if err := c.memDB.UpdateStatus(req.AgentID, req.Status, req.CurrentTask); err != nil {
		// Log but don't fail
	}

	// Check if agent should shut down
	shouldStop, stopReason, _ := c.memDB.CheckShutdownFlag(req.AgentID)

	return &HeartbeatResponse{
		OK:           true,
		Timestamp:    time.Now(),
		ShouldStop:   shouldStop,
		StopReason:   stopReason,
		HasMessages:  false, // TODO: implement message queue
		MessageCount: 0,
	}, nil
}

// StatusUpdate represents an agent status update
type StatusUpdate struct {
	AgentID     string `json:"agent_id"`
	Status      string `json:"status"`
	CurrentTask string `json:"current_task"`
	Progress    int    `json:"progress,omitempty"` // 0-100
}

// UpdateStatus updates an agent's status
func (c *AgentComms) UpdateStatus(update *StatusUpdate) error {
	return c.memDB.UpdateStatus(update.AgentID, update.Status, update.CurrentTask)
}

// SignalRequest represents a signal from one agent to another
type SignalRequest struct {
	FromAgentID string                 `json:"from_agent_id"`
	ToAgentID   string                 `json:"to_agent_id"` // empty = broadcast
	Signal      string                 `json:"signal"`      // stop, pause, resume, message
	Context     map[string]interface{} `json:"context,omitempty"`
}

// SignalResponse confirms signal delivery
type SignalResponse struct {
	Delivered bool   `json:"delivered"`
	Target    string `json:"target"`
	Signal    string `json:"signal"`
}

// SendSignal sends a signal to another agent
func (c *AgentComms) SendSignal(req *SignalRequest) (*SignalResponse, error) {
	switch req.Signal {
	case "stop":
		// Set shutdown flag for target agent
		if req.ToAgentID != "" {
			reason := "Stop signal from " + req.FromAgentID
			if ctx, ok := req.Context["reason"].(string); ok {
				reason = ctx
			}
			if err := c.memDB.SetShutdownFlag(req.ToAgentID, reason); err != nil {
				return nil, err
			}
		}
	case "pause", "resume":
		// Update agent status
		status := "paused"
		if req.Signal == "resume" {
			status = "working"
		}
		if req.ToAgentID != "" {
			if err := c.memDB.UpdateStatus(req.ToAgentID, status, ""); err != nil {
				return nil, err
			}
		}
	}

	return &SignalResponse{
		Delivered: true,
		Target:    req.ToAgentID,
		Signal:    req.Signal,
	}, nil
}

// QueryRequest represents a routed query request
type QueryRequest struct {
	AgentID string `json:"agent_id"`
	Query   string `json:"query"`
	Limit   int    `json:"limit,omitempty"`
}

// RouteQuery routes a query through the skill router
func (c *AgentComms) RouteQuery(req *QueryRequest) (*QueryResult, error) {
	return c.router.RouteQuery(req.Query, req.Limit)
}

// ShutdownCheck is a quick poll for shutdown status
type ShutdownCheck struct {
	ShouldStop bool   `json:"should_stop"`
	Reason     string `json:"reason,omitempty"`
}

// CheckShutdown checks if an agent should shut down
func (c *AgentComms) CheckShutdown(agentID string) (*ShutdownCheck, error) {
	shouldStop, reason, err := c.memDB.CheckShutdownFlag(agentID)
	if err != nil {
		return nil, err
	}
	return &ShutdownCheck{
		ShouldStop: shouldStop,
		Reason:     reason,
	}, nil
}

// RegisterShutdownChannel registers a channel to receive shutdown signals
func (c *AgentComms) RegisterShutdownChannel(agentID string) <-chan struct{} {
	c.shutdownMu.Lock()
	defer c.shutdownMu.Unlock()

	ch := make(chan struct{})
	c.shutdowns[agentID] = ch
	return ch
}

// UnregisterShutdownChannel removes a shutdown channel
func (c *AgentComms) UnregisterShutdownChannel(agentID string) {
	c.shutdownMu.Lock()
	defer c.shutdownMu.Unlock()

	if ch, exists := c.shutdowns[agentID]; exists {
		close(ch)
		delete(c.shutdowns, agentID)
	}
}

// TriggerShutdown sends a shutdown signal to an agent
func (c *AgentComms) TriggerShutdown(agentID string) {
	c.shutdownMu.RLock()
	ch, exists := c.shutdowns[agentID]
	c.shutdownMu.RUnlock()

	if exists {
		select {
		case <-ch:
			// Already closed
		default:
			close(ch)
		}
	}
}
