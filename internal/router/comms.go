package router

import (
	"log"
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
	// NOTE: Agent status is now tracked in-memory only via JSONStore
	// Shutdown signals are handled via channels, not database flags

	// NOTE: Message queue not yet implemented in MemoryDB
	// HasMessages always returns false until message queue storage is added
	// Future: Add message table to track agent-to-agent messages
	hasMessages := false
	messageCount := 0

	return &HeartbeatResponse{
		OK:           true,
		Timestamp:    time.Now(),
		ShouldStop:   false,
		StopReason:   "",
		HasMessages:  hasMessages,
		MessageCount: messageCount,
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
	// NOTE: Agent status is now tracked in-memory only via JSONStore
	// This method is kept for API compatibility but does nothing
	log.Printf("[COMMS] Status update for agent %s: %s - %s", update.AgentID, update.Status, update.CurrentTask)
	return nil
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
	// NOTE: Agent signals are now handled via channels, not database flags
	// This method is kept for API compatibility but uses the channel mechanism
	switch req.Signal {
	case "stop":
		if req.ToAgentID != "" {
			c.TriggerShutdown(req.ToAgentID)
		}
	case "pause", "resume":
		// Log the signal, actual state tracking is in JSONStore
		status := "paused"
		if req.Signal == "resume" {
			status = "working"
		}
		log.Printf("[COMMS] Signal %s -> agent %s: %s", req.Signal, req.ToAgentID, status)
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
	// NOTE: Shutdown signals are now handled via channels, not database flags
	// Check if shutdown channel exists and is closed
	c.shutdownMu.RLock()
	ch, exists := c.shutdowns[agentID]
	c.shutdownMu.RUnlock()

	shouldStop := false
	if exists {
		select {
		case <-ch:
			shouldStop = true
		default:
			shouldStop = false
		}
	}

	return &ShutdownCheck{
		ShouldStop: shouldStop,
		Reason:     "shutdown signal received",
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
