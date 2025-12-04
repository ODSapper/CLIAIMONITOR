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
		OnHeartbeat:      bridge.handleHeartbeat,
		OnStatusUpdate:   bridge.handleStatusUpdate,
		OnToolCall:       bridge.handleToolCall,
		OnStopApproval:   bridge.handleStopApproval,
		OnShutdownNotify: bridge.handleShutdownNotify,
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

	// Update heartbeat tracking (similar to HTTP handler)
	b.server.heartbeatMu.Lock()
	info, exists := b.server.agentHeartbeats[agentID]
	if !exists {
		info = &HeartbeatInfo{
			AgentID:     agentID,
			ConfigName:  configName,
			ProjectPath: projectPath,
		}
		b.server.agentHeartbeats[agentID] = info
	}
	info.Status = status
	info.CurrentTask = task
	info.LastSeen = time.Now()
	b.server.heartbeatMu.Unlock()

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

	// Update database heartbeat
	if b.server.memDB != nil {
		b.server.memDB.UpdateHeartbeat(agentID)
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

	// Update database
	if b.server.memDB != nil {
		b.server.memDB.UpdateHeartbeat(agentID)
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

	// Remove from heartbeat tracking
	b.server.heartbeatMu.Lock()
	delete(b.server.agentHeartbeats, agentID)
	b.server.heartbeatMu.Unlock()

	// Update database
	if b.server.memDB != nil {
		b.server.memDB.UpdateStatus(agentID, "disconnected", fmt.Sprintf("shutdown: %s", reason))
	}

	b.server.broadcastState()
	return nil
}
