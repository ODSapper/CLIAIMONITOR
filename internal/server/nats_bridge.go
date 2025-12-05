package server

import (
	"fmt"
	"log"
	"time"

	natslib "github.com/CLIAIMONITOR/internal/nats"
	"github.com/CLIAIMONITOR/internal/types"
)

// NATSBridge connects NATS messaging to server state management
type NATSBridge struct {
	server  *Server
	handler *natslib.Handler
}

// NewNATSBridge creates a bridge between NATS and server state
func NewNATSBridge(s *Server, client *natslib.Client) *NATSBridge {
	bridge := &NATSBridge{
		server: s,
	}

	callbacks := natslib.HandlerCallbacks{
		OnHeartbeat:         bridge.handleHeartbeat,
		OnStatusUpdate:      bridge.handleStatusUpdate,
		OnToolCall:          bridge.handleToolCall,
		OnStopApproval:      bridge.handleStopApproval,
		OnShutdownNotify:    bridge.handleShutdownNotify,
		OnCaptainStatus:     bridge.handleCaptainStatus,
		OnEscalationForward: bridge.handleEscalationForward,
		OnSystemBroadcast:   bridge.handleSystemBroadcast,
	}

	bridge.handler = natslib.NewHandler(client, callbacks)
	return bridge
}

// Start begins processing NATS messages
func (b *NATSBridge) Start() error {
	return b.handler.Start()
}

// Stop terminates message processing
func (b *NATSBridge) Stop() {
	b.handler.Stop()
}

// handleHeartbeat processes agent heartbeats via NATS
func (b *NATSBridge) handleHeartbeat(agentID, status, task, configName, projectPath string) error {
	log.Printf("[NATS-BRIDGE] Heartbeat from %s: status=%s task=%s", agentID, status, task)

	// Update agent in store
	b.server.store.UpdateAgent(agentID, func(a *types.Agent) {
		a.Status = types.AgentStatus(status)
		a.CurrentTask = task
		a.LastSeen = time.Now()
		if configName != "" {
			a.ConfigName = configName
		}
		if projectPath != "" {
			a.ProjectPath = projectPath
		}
	})

	// Update database status
	if b.server.memDB != nil {
		b.server.memDB.UpdateStatus(agentID, status, task)
	}

	b.server.broadcastState()
	return nil
}

// handleStatusUpdate processes status changes via NATS
func (b *NATSBridge) handleStatusUpdate(agentID, status, message string) error {
	log.Printf("[NATS-BRIDGE] Status update from %s: %s - %s", agentID, status, message)

	b.server.store.UpdateAgent(agentID, func(a *types.Agent) {
		a.Status = types.AgentStatus(status)
		a.CurrentTask = message
		a.LastSeen = time.Now()
	})

	// Update metrics idle tracking
	if status == string(types.StatusIdle) {
		b.server.metrics.SetAgentIdle(agentID)
	} else {
		b.server.metrics.SetAgentActive(agentID)
	}

	// Update database status
	if b.server.memDB != nil {
		b.server.memDB.UpdateStatus(agentID, status, message)
	}

	b.server.broadcastState()
	return nil
}

// handleToolCall processes tool calls via NATS request-reply
func (b *NATSBridge) handleToolCall(agentID, tool string, args map[string]interface{}) (interface{}, error) {
	log.Printf("[NATS-BRIDGE] Tool call from %s: %s", agentID, tool)

	// Delegate to MCP tool registry (placeholder - will be wired to actual tools)
	// For now, return error indicating tool not found via NATS
	return nil, fmt.Errorf("tool %s not yet available via NATS (use MCP SSE for now)", tool)
}

// handleStopApproval processes stop approval requests via NATS
func (b *NATSBridge) handleStopApproval(agentID, reason, context string, workCompleted bool) (bool, string, error) {
	log.Printf("[NATS-BRIDGE] Stop approval request from %s: %s (work_completed=%v)", agentID, reason, workCompleted)

	// Check if agent already has approved shutdown
	state := b.server.store.GetState()
	if agent, ok := state.Agents[agentID]; ok {
		if agent.ShutdownRequested {
			return true, "shutdown already approved", nil
		}
	}

	// Create stop request for human approval
	req := &types.StopApprovalRequest{
		ID:        fmt.Sprintf("nats-stop-%d", time.Now().UnixNano()),
		AgentID:   agentID,
		Reason:    fmt.Sprintf("%s (context: %s, work_completed: %v)", reason, context, workCompleted),
		CreatedAt: time.Now(),
		Reviewed:  false,
	}
	b.server.store.AddStopRequest(req)

	// Alert via WebSocket
	b.server.hub.BroadcastAlert(&types.Alert{
		ID:        req.ID,
		Type:      "stop_approval_needed",
		AgentID:   agentID,
		Message:   fmt.Sprintf("[NATS] Agent %s wants to stop: %s", agentID, reason),
		Severity:  "warning",
		CreatedAt: time.Now(),
	})

	b.server.broadcastState()

	// Return pending - agent should poll or wait for notification
	return false, "pending_approval", nil
}

// handleShutdownNotify processes shutdown notifications
func (b *NATSBridge) handleShutdownNotify(agentID, reason string, approved, force bool) error {
	log.Printf("[NATS-BRIDGE] Shutdown notification from %s: reason=%s approved=%v force=%v", agentID, reason, approved, force)

	// Update agent status to disconnected
	b.server.store.UpdateAgent(agentID, func(a *types.Agent) {
		a.Status = types.StatusDisconnected
		a.LastSeen = time.Now()
	})

	// Update database
	if b.server.memDB != nil {
		b.server.memDB.UpdateStatus(agentID, "disconnected", fmt.Sprintf("shutdown: %s", reason))
	}

	b.server.broadcastState()
	return nil
}

// handleCaptainStatus processes Captain status updates
func (b *NATSBridge) handleCaptainStatus(status, currentOp string, queueSize int) error {
	log.Printf("[NATS-BRIDGE] Captain status update: status=%s currentOp=%s queueSize=%d", status, currentOp, queueSize)

	// Update Captain status in store
	b.server.store.SetCaptainStatus(status)

	// Mark Captain as connected when we receive status updates
	b.server.store.SetCaptainConnected(true)

	// Broadcast updated state to dashboard
	b.server.broadcastState()
	return nil
}

// handleEscalationForward processes escalations forwarded by Captain to human
func (b *NATSBridge) handleEscalationForward(id, agentID, question, captainContext, captainRecommends string) error {
	log.Printf("[NATS-BRIDGE] Escalation forwarded from Captain: id=%s agent=%s", id, agentID)

	// Create alert for dashboard
	alert := &types.Alert{
		ID:        id,
		Type:      "escalation",
		AgentID:   agentID,
		Message:   fmt.Sprintf("Question from agent %s: %s", agentID, question),
		Severity:  "info",
		CreatedAt: time.Now(),
	}

	// If captain provided context or recommendations, add to message
	if captainContext != "" {
		alert.Message += fmt.Sprintf("\n\nCaptain Context: %s", captainContext)
	}
	if captainRecommends != "" {
		alert.Message += fmt.Sprintf("\n\nCaptain Recommends: %s", captainRecommends)
	}

	// Broadcast alert to dashboard via WebSocket
	b.server.hub.BroadcastAlert(alert)

	// Also broadcast state update
	b.server.broadcastState()
	return nil
}

// handleSystemBroadcast processes system-wide broadcast messages
func (b *NATSBridge) handleSystemBroadcast(msgType, message string, data map[string]interface{}) error {
	log.Printf("[NATS-BRIDGE] System broadcast: type=%s message=%s", msgType, message)

	// Create alert for dashboard based on broadcast type
	severity := "info"
	if msgType == "shutdown" || msgType == "agent_killed" {
		severity = "warning"
	}

	alert := &types.Alert{
		ID:        fmt.Sprintf("broadcast-%d", time.Now().UnixNano()),
		Type:      msgType,
		Message:   message,
		Severity:  severity,
		CreatedAt: time.Now(),
	}

	// Extract agent ID if present in data
	if agentID, ok := data["agent_id"].(string); ok {
		alert.AgentID = agentID
	}

	// Broadcast alert to dashboard
	b.server.hub.BroadcastAlert(alert)

	return nil
}
